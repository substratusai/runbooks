package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/controller"
	"github.com/substratusai/substratus/internal/sci"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(apiv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "df3bdd2d.substratus.ai",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := controller.SetupIndexes(mgr); err != nil {
		setupLog.Error(err, "unable to setup indexes")
		os.Exit(1)
	}

	//runtimeMgr, err := controller.NewRuntimeManager(controller.GPUType(os.Getenv("GPU_TYPE")))
	//if err != nil {
	//	setupLog.Error(err, "unable to configure runtime manager")
	//	os.Exit(1)
	//}

	// NOTE: NewCloudContext() will look up environment variables (intended for local development)
	// and if they are not specified, it will try to use metadata servers on the cloud.
	cloudContext, err := cloud.NewContext()
	if err != nil {
		setupLog.Error(err, "unable to determine cloud context")
		os.Exit(1)
	}

	// TODO(any): setup TLS
	conn, err := grpc.Dial(
		"gcp-manager.substratus.svc.cluster.local:10080",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		setupLog.Error(err, "unable to create an SCI gRPC client")
		os.Exit(1)
	}
	defer conn.Close()
	// Create a client using the connection
	gc := sci.NewControllerClient(conn)

	if err = (&controller.ModelReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		CloudContext: cloudContext,
		ContainerImageReconciler: &controller.ContainerImageReconciler{
			Scheme:                 mgr.GetScheme(),
			Client:                 mgr.GetClient(),
			CloudContext:           cloudContext,
			CloudManagerGrpcClient: gc,
			Kind:                   "Model",
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Model")
		os.Exit(1)
	}
	if err = (&controller.ServerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		ContainerImageReconciler: &controller.ContainerImageReconciler{
			Scheme:                 mgr.GetScheme(),
			Client:                 mgr.GetClient(),
			CloudContext:           cloudContext,
			CloudManagerGrpcClient: gc,
			Kind:                   "Server",
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Server")
		os.Exit(1)
	}

	if err = (&controller.NotebookReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		ContainerImageReconciler: &controller.ContainerImageReconciler{
			Scheme:                 mgr.GetScheme(),
			Client:                 mgr.GetClient(),
			CloudContext:           cloudContext,
			CloudManagerGrpcClient: gc,
			Kind:                   "Notebook",
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Notebook")
		os.Exit(1)
	}
	if err = (&controller.DatasetReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		CloudContext: cloudContext,
		ContainerImageReconciler: &controller.ContainerImageReconciler{
			Scheme:                 mgr.GetScheme(),
			Client:                 mgr.GetClient(),
			CloudContext:           cloudContext,
			CloudManagerGrpcClient: gc,
			Kind:                   "Dataset",
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Dataset")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
