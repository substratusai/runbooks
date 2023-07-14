// Package gcp provides a GCP implementation of the Substratus Cloud Interface (SCI).
package gcpmanager

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	storage "cloud.google.com/go/storage"
	sci "github.com/substratusai/substratus/internal/sci"
	"golang.org/x/oauth2/google"
	credentialspb "google.golang.org/genproto/googleapis/iam/credentials/v1"
)

// server implements the sci.ControllerServer interface.
type Server struct {
	sci.UnimplementedControllerServer
	Clients
	SaEmail string
}

type Clients struct {
	Iam      *credentials.IamCredentialsClient
	Metadata *metadata.Client
	Storage  *storage.Client
	Http     *http.Client
}

// CreateSignedURL generates a signed URL for a specified GCS bucket and object path.
func (s *Server) CreateSignedURL(ctx context.Context, req *sci.CreateSignedURLRequest) (*sci.CreateSignedURLResponse, error) {
	bucketName, objectName := req.GetBucketName(), req.GetObjectName()
	bucket := s.Clients.Storage.Bucket(bucketName)
	obj := bucket.Object(objectName)
	if _, err := obj.Attrs(ctx); err != nil && err != storage.ErrObjectNotExist {
		// An error occurred that was NOT ErrObjectNotExist.
		// This is an unexpected error and we should return it.
		return nil, err
	}

	opts := &storage.SignedURLOptions{
		Scheme: storage.SigningSchemeV4,
		Method: http.MethodPut,
		Headers: []string{
			"Content-Type:application/octet-stream",
		},
		Expires:        time.Now().Add(time.Duration(req.GetExpirationSeconds()) * time.Second),
		GoogleAccessID: s.SaEmail,
		SignBytes: func(b []byte) ([]byte, error) {
			req := &credentialspb.SignBlobRequest{
				Payload: b,
				Name:    s.SaEmail,
			}
			resp, err := s.Clients.Iam.SignBlob(ctx, req)
			if err != nil {
				panic(err)
			}
			return resp.SignedBlob, err
		},
	}

	// Create a signed URL
	url, err := storage.SignedURL(bucketName, objectName, opts)
	if err != nil {
		return nil, err
	}

	return &sci.CreateSignedURLResponse{Url: url}, nil
}

func GetServiceAccountEmail(m *metadata.Client) (string, error) {
	if metadata.OnGCE() {
		email, err := m.Email("default")
		if err != nil {
			return "", err
		}
		return email, nil
	} else {
		// Parse the service account email from the key file.
		keyFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		if keyFile == "" {
			return "", fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS environment variable not set")
		}

		keyBytes, err := ioutil.ReadFile(keyFile)
		if err != nil {
			return "", err
		}

		cfg, err := google.JWTConfigFromJSON(keyBytes)
		if err != nil {
			return "", err
		}

		return cfg.Email, nil
	}
}
