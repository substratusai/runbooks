package kind_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	sci "github.com/substratusai/substratus/internal/sci"
	scikind "github.com/substratusai/substratus/internal/sci/kind"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestServer(t *testing.T) {
	bucketDir := os.TempDir()

	s := &scikind.Server{}

	signedURLServer := httptest.NewServer(s)
	defer signedURLServer.Close()
	s.SignedURLAddress = signedURLServer.URL

	gs := grpc.NewServer()
	sci.RegisterControllerServer(gs, s)

	grpcAddr := fmt.Sprintf(":%v", 2222)
	go func() {
		lis, err := net.Listen("tcp", grpcAddr)
		require.NoError(t, err)
		require.NoError(t, gs.Serve(lis))
	}()

	conn, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	c := sci.NewControllerClient(conn)

	ctx := context.Background()

	{
		t.Log("Creating signed url")
		resp, err := c.CreateSignedURL(ctx, &sci.CreateSignedURLRequest{
			Md5Checksum: "123",
			ObjectName:  "abc/uploads/latest.tar.gz",
		})
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%v/abc/uploads/latest.tar.gz", signedURLServer.URL), resp.Url)
	}

	{
		t.Log("Uploading file")
		body := bytes.NewReader([]byte("hello"))
		req, err := http.NewRequest(
			http.MethodPut,
			fmt.Sprintf("%v%v", signedURLServer.URL, filepath.Join(bucketDir, "/abc/uploads/latest.tar.gz")),
			body,
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/octet-stream")

		// MD5 encoded "hello":
		md5Hex := "5d41402abc4b2a76b9719d911017c592"
		md5Btys, err := hex.DecodeString(md5Hex)
		require.NoError(t, err)
		md5B64 := base64.StdEncoding.EncodeToString(md5Btys)
		req.Header.Set("Content-MD5", md5B64)

		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		require.FileExists(t, filepath.Join(bucketDir, "abc/uploads/md5.txt"))
		contents, err := os.ReadFile(filepath.Join(bucketDir, "abc/uploads/latest.tar.gz"))
		require.NoError(t, err)
		require.Equal(t, "hello", string(contents))
	}

	{
		t.Log("Getting md5")
		resp, err := c.GetObjectMd5(ctx, &sci.GetObjectMd5Request{
			ObjectName: filepath.Join(bucketDir, "abc/uploads/latest.tar.gz"),
		})
		require.NoError(t, err)
		require.Equal(t, "5d41402abc4b2a76b9719d911017c592", resp.Md5Checksum)
	}

}
