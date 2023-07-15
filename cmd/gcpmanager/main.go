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

	ctx := context.Background()
	iamClient, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		log.Fatalf("failed to create iam client: %v", err)
		log.Fatal(err)
	}

	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("failed to create storage client: %v", err)
	}

	hc := &http.Client{}
	mc := metadata.NewClient(hc)
	saEmail, err := gcpmanager.GetServiceAccountEmail(mc)
	if err != nil {
		log.Fatalf("failed to get the SA email: %v", err)
	}

	s := gcpmanager.Server{
		Clients: gcpmanager.Clients{
			Iam:      iamClient,
			Metadata: mc,
			Storage:  storageClient,
			Http:     hc,
		},
		SaEmail: saEmail,
	}
	gs := grpc.NewServer()
	sci.RegisterControllerServer(gs, &s)

	// Setup Health Check
	hs := health.NewServer()
	hs.SetServingStatus("", hv1.HealthCheckResponse_SERVING)
	hv1.RegisterHealthServer(gs, hs)

	fmt.Printf("gcpmanager server listening on port %v...", port)
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if err := gs.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}