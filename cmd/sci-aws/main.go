package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/substratusai/substratus/internal/sci"
	awssci "github.com/substratusai/substratus/internal/sci/aws"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	hv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	// serve by default on port 10081
	var port int
	flag.IntVar(&port, "port", 10081, "port number to listen on")
	flag.Parse()

	// Create new AWS Server
	s, err := awssci.NewAWSServer()
	if err != nil {
		log.Fatalf("failed to create AWS server: %v", err)
	}

	gs := grpc.NewServer()
	sci.RegisterControllerServer(gs, s)

	// Setup Health Check
	hs := health.NewServer()
	hs.SetServingStatus("", hv1.HealthCheckResponse_SERVING)
	hv1.RegisterHealthServer(gs, hs)

	fmt.Printf("awssci server listening on port %v...", port)
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if err := gs.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
