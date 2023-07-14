package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"

	"cloud.google.com/go/storage"
	"github.com/substratusai/substratus/internal/gcpmanager"
	"github.com/substratusai/substratus/internal/sci"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	hv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	// serve by default on port 10080
	var port int
	flag.IntVar(&port, "port", 10080, "port number to listen on")
	flag.Parse()

	// Create a storage client
	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("failed to create storage client: %v", err)
	}

	s := grpc.NewServer()
	sci.RegisterControllerServer(s, &gcpmanager.Server{
		StorageClient: storageClient,
	})

	// Setup Health Check
	hs := health.NewServer()
	hs.SetServingStatus("", hv1.HealthCheckResponse_SERVING)
	hv1.RegisterHealthServer(s, hs)

	fmt.Printf("gcpmanager server listening on port %v...", port)
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
