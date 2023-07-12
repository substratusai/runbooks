// Package gcs provides a gRPC server implementation for the Blob Storage Interface (BSI).
// Google Cloud Storage (GCS) is the implemented storage service.
package gcs

import (
	"context"
	"log"
	"net"
	"time"

	bsi "github.com/substratusai/substratus/internal/bsi"

	storage "cloud.google.com/go/storage"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// server implements the bsi.ControllerServer interface.
type server struct {
	bsi.UnimplementedControllerServer
	storageClient *storage.Client
}

// CreateSignedURL generates a signed URL for a specified GCS bucket and object path.
func (s *server) CreateSignedURL(ctx context.Context, req *bsi.CreateSignedURLRequest) (*bsi.CreateSignedURLResponse, error) {
	// Use the bucket and object path from the request
	bucket := s.storageClient.Bucket(req.GetBucketName())
	obj := bucket.Object(req.GetObjectPath())
	if _, err := obj.Attrs(ctx); err != nil {
		if err != storage.ErrObjectNotExist {
			// An error occurred that was NOT ErrObjectNotExist.
			// This is an unexpected error and we should return it.
			return nil, err
		}
		// the object doesn't exist and we want to continue and create the signed URL.
	}

	// Create a signed URL
	url, err := storage.SignedURL(req.GetBucketName(), req.GetObjectPath(), &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(time.Duration(req.GetExpirationSeconds()) * time.Second),
	})
	if err != nil {
		return nil, err
	}

	// Format the completion time to the proto spec
	completionTime := timestamppb.New(time.Now())
	if err != nil {
		return nil, err
	}

	return &bsi.CreateSignedURLResponse{Url: url, CompletedAt: completionTime}, nil
}

func main() {
	// Create a storage client
	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("failed to create storage client: %v", err)
	}

	s := grpc.NewServer()
	bsi.RegisterControllerServer(s, &server{
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
