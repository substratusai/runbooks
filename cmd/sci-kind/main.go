package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/substratusai/substratus/internal/sci"
	scikind "github.com/substratusai/substratus/internal/sci/kind"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	hv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	var cfg struct {
		port                 int
		signedURLPort        int
		hostSignedURLAddress string
	}
	flag.IntVar(&cfg.port, "port", 10080, "port number to listen on")
	flag.IntVar(&cfg.signedURLPort, "signed-url-port", 8080, "port to listen for signed url traffic")
	flag.StringVar(&cfg.hostSignedURLAddress, "host-signed-url-address", "http://localhost:30080",
		"host address that port forwards to the signed url port within the cluster. this should be set in kind config.yaml.")
	flag.Parse()

	s := &scikind.Server{
		SignedURLAddress: cfg.hostSignedURLAddress,
	}
	signedURLServer := &http.Server{
		Addr:    fmt.Sprintf(":%v", cfg.signedURLPort),
		Handler: s,
	}
	go func() {
		log.Printf("Listening for signed URL traffic on address: %v", cfg.signedURLPort)
		log.Fatal(signedURLServer.ListenAndServe())
	}()

	gs := grpc.NewServer()
	sci.RegisterControllerServer(gs, s)

	// Setup Health Check
	hs := health.NewServer()
	hs.SetServingStatus("", hv1.HealthCheckResponse_SERVING)
	hv1.RegisterHealthServer(gs, hs)

	addr := fmt.Sprintf(":%v", cfg.port)
	log.Printf("Listening for gRPC traffic on address: %v", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if err := gs.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
