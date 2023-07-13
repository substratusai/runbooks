package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"

	"cloud.google.com/go/storage"
	"github.com/substratusai/substratus/internal/gcpmanager"
	"github.com/substratusai/substratus/internal/sci"
	"google.golang.org/grpc"
)

func main() {
	// Create a storage client
	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("failed to create storage client: %v", err)
	}

	s := grpc.NewServer()
	sci.RegisterControllerServer(s, &gcpmanager.Server{
		StorageClient: storageClient,
	})

	invokeManually(storageClient)

	port := 10443
	fmt.Printf("gcpmanager server listening on port %v...", port)
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func invokeManually(storageClient *storage.Client) {
	payload := sci.CreateSignedURLRequest{
		BucketName:        "substr-models1",
		ObjectName:        "README.md",
		ExpirationSeconds: 600,
	}
	serv := gcpmanager.Server{
		StorageClient: storageClient,
	}

	fmt.Println("calling CreateSignedURL with payload:")
	resp, err := serv.CreateSignedURL(context.Background(), &payload)
	if err != nil {
		log.Fatalf("failed to create signed URL: %v", err)
	}
	fmt.Printf("signed URL: %v\n", resp.Url)
}
