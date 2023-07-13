// Package gcp provides a GCP implementation of the Substratus Cloud Interface (SCI).
package gcpmanager

import (
	"context"
	"log"
	"net"
	"time"

	sci "github.com/substratusai/substratus/internal/sci"

	storage "cloud.google.com/go/storage"
	"google.golang.org/grpc"
)

// server implements the sci.ControllerServer interface.
type server struct {
	sci.UnimplementedControllerServer
	storageClient *storage.Client
}

// CreateSignedURL generates a signed URL for a specified GCS bucket and object path.
func (s *server) CreateSignedURL(ctx context.Context, req *sci.CreateSignedURLRequest) (*sci.CreateSignedURLResponse, error) {
	bucketName, objectName := req.GetBucketName(), req.GetObjectName()

	bucket := s.storageClient.Bucket(bucketName)
	obj := bucket.Object(objectName)
	if _, err := obj.Attrs(ctx); err != nil {
		if err != storage.ErrObjectNotExist {
			// An error occurred that was NOT ErrObjectNotExist.
			// This is an unexpected error and we should return it.
			return nil, err
		}
		// the object doesn't exist and we want to continue and create the signed URL.
	}

	opts := &storage.SignedURLOptions{
		// this is optional storage.SigningSchemeV2 is the default. storage.SigningSchemeV4 is also available.
		Scheme:  storage.SigningSchemeV2,
		Method:  "POST",
		Expires: time.Now().Add(time.Duration(req.GetExpirationSeconds()) * time.Second),
	}

	// Create a signed URL
	url, err := storage.SignedURL(bucketName, objectName, opts)
	if err != nil {
		return nil, err
	}

	return &sci.CreateSignedURLResponse{Url: url}, nil
}

func main() {
	// Create a storage client
	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("failed to create storage client: %v", err)
	}

	s := grpc.NewServer()
	sci.RegisterControllerServer(s, &server{
		storageClient: storageClient,
	})

	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
