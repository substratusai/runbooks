package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
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
	s, err := NewServer()
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

func NewServer() (*awssci.Server, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	clusterID, err := awssci.GetClusterID()
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster ID: %w", err)
	}

	oidcProviderURL, err := awssci.GetOidcProviderUrl(sess, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster OIDC provider URL: %w", err)
	}

	stsSvc := sts.New(sess)
	accountId, err := awssci.GetAccountID(stsSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to get account ID: %w", err)
	}

	oidcProviderARN := fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", accountId, oidcProviderURL)

	c := &awssci.Clients{
		S3Client:  s3.New(sess),
		IAMClient: iam.New(sess),
	}

	return &awssci.Server{
		Clients:         *c,
		OIDCProviderURL: oidcProviderURL,
		OIDCProviderARN: oidcProviderARN,
	}, nil
}
