package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"

	ctrl "sigs.k8s.io/controller-runtime"

	"cloud.google.com/go/compute/metadata"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/storage"
	"github.com/substratusai/substratus/internal/sci"
	"github.com/substratusai/substratus/internal/sci/gcp"
	"google.golang.org/api/iam/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	hv1 "google.golang.org/grpc/health/grpc_health_v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var setupLog = ctrl.Log.WithName("setup")

func main() {
	// serve by default on port 10080
	var port int
	flag.IntVar(&port, "port", 10080, "port number to listen on")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	ctx := context.Background()
	iamCredClient, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		setupLog.Error(err, "failed to create iam credentials client")
		os.Exit(1)
	}

	iamService, err := iam.NewService(ctx)
	if err != nil {
		setupLog.Error(err, "failed to create iam client")
		os.Exit(1)
	}

	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		setupLog.Error(err, "failed to create storage client")
		os.Exit(1)
	}

	hc := &http.Client{}
	mc := metadata.NewClient(hc)

	s, err := gcp.NewServer()
	if err != nil {
		setupLog.Error(err, "failed to create server")
		os.Exit(1)
	}
	s.Clients = gcp.Clients{
		IAMCredentialsClient: iamCredClient,
		IAM:                  iamService,
		Metadata:             mc,
		Storage:              storageClient,
		HTTP:                 hc,
	}
	if err := s.AutoConfigure(mc); err != nil {
		setupLog.Error(err, "failed to AutoConfigure server")
		os.Exit(1)
	}

	if err := s.Validate(); err != nil {
		setupLog.Error(err, "failed to validate server")
		os.Exit(1)
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
		setupLog.Error(err, "failed to listen", "port", port)
		os.Exit(1)
	}

	if err := gs.Serve(lis); err != nil {
		setupLog.Error(err, "failed to serve", "port", port)
		os.Exit(1)
	}
}
