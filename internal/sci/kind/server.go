package kind

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	sci "github.com/substratusai/substratus/internal/sci"
)

var _ sci.ControllerServer = &Server{}

type Server struct {
	SignedURLAddress string

	sci.UnimplementedControllerServer
}

// ServeHTTP implements the http.Handler interface and is used to provide cloud-bucket-like signed URL support.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Signed URL Server: %v", r.URL.Path)

	switch r.Method {
	case http.MethodPut:
		// Expect "Content-Type: application/octet-stream" in a PUT request and save the body to a file.
		if r.Header.Get("Content-Type") != "application/octet-stream" {
			log.Print("client sent wrong content-type")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		md5B64 := r.Header.Get("Content-MD5")
		if md5B64 == "" {
			log.Print("client did not send content-md5")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		md5Raw, err := base64.StdEncoding.DecodeString(md5B64)
		if err != nil {
			log.Printf("content-md5 is not base64 encoded: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		md5 := hex.EncodeToString(md5Raw)

		if err := s.saveUpload(r.Body, r.URL.Path, md5); err != nil {
			log.Printf("failed to save upload: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

}

func (s *Server) saveUpload(r io.Reader, urlPath, md5 string) error {
	// urlPath should look like: "/bucket/<guid>/..."
	dir := filepath.Dir(urlPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir (all): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "md5.txt"), []byte(md5), 0644); err != nil {
		return fmt.Errorf("write md5 file: %v", err)
	}

	f, err := os.Create(urlPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return f.Close()
}

func (s *Server) CreateSignedURL(ctx context.Context, req *sci.CreateSignedURLRequest) (*sci.CreateSignedURLResponse, error) {
	log.Printf("CreateSignedURL: %v", req.ObjectName)

	return &sci.CreateSignedURLResponse{
		Url: fmt.Sprintf("%v/%v", s.SignedURLAddress, req.ObjectName),
	}, nil
}

func (s *Server) GetObjectMd5(ctx context.Context, req *sci.GetObjectMd5Request) (*sci.GetObjectMd5Response, error) {
	log.Printf("GetObjectMd5: %v", req.ObjectName)

	path := filepath.Join(filepath.Dir(req.ObjectName), "md5.txt")
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read md5 file: %v", err)
	}

	md5 := strings.TrimSpace(string(contents))
	log.Printf("GetObjectMd5: found file %q with md5: %v", path, md5)

	return &sci.GetObjectMd5Response{
		Md5Checksum: md5,
	}, nil
}

func (s *Server) BindIdentity(ctx context.Context, in *sci.BindIdentityRequest) (*sci.BindIdentityResponse, error) {
	return &sci.BindIdentityResponse{}, nil
}
