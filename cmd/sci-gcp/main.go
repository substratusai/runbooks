package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"

	"cloud.google.com/go/compute/metadata"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/storage"
	"github.com/substratusai/substratus/internal/sci"
	"github.com/substratusai/substratus/internal/sci/gcp"
	"google.golang.org/api/iam/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	hv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	// serve by default on port 10080
	var port int
	flag.IntVar(&port, "port", 10080, "port number to listen on")
	flag.Parse()

	ctx := context.Background()
	iamCredClient, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		log.Fatalf("failed to create iam credentials client: %v", err)
	}

	iamService, err := iam.NewService(ctx)
	if err != nil {
		log.Fatalf("failed to create iam client: %v", err)
	}

	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("failed to create storage client: %v", err)
	}

	hc := &http.Client{}
	mc := metadata.NewClient(hc)

	s, err := gcp.NewServer()
	if err != nil {
		log.Fatalf("error creating new server: %v", err)
	}
	s.Clients = gcp.Clients{
		IAMCredentialsClient: iamCredClient,
		IAM:                  iamService,
		Metadata:             mc,
		Storage:              storageClient,
		Http:                 hc,
	}
	if err := s.AutoConfigure(mc); err != nil {
		log.Fatalf("error with automatically configuring sci-gcp: %v", err)
	}
	gs := grpc.NewServer()
	sci.RegisterControllerServer(gs, s)

	// Setup Health Check
	hs := health.NewServer()
	hs.SetServingStatus("", hv1.HealthCheckResponse_SERVING)
	hv1.RegisterHealthServer(gs, hs)

	fmt.Printf("sci.gcp server listening on port %v...", port)
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if err := gs.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
