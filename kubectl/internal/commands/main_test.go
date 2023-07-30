package commands_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	apiv1 "github.com/substratusai/substratus/api/v1"
	iclient "github.com/substratusai/substratus/kubectl/internal/client"
	"github.com/substratusai/substratus/kubectl/internal/commands"
	//+kubebuilder:scaffold:imports
)

const (
	timeout  = time.Second * 5
	interval = time.Second / 10
	testUUID = "c1d1eb65-75bd-48a5-9bad-802810fc9117"
)

var (
	kubectlKubeconfigPath string
	k8sClient             client.Client
	testEnv               *envtest.Environment
	ctx                   context.Context
	cancel                context.CancelFunc
	stdout                bytes.Buffer
)

func TestMain(m *testing.M) {
	commands.NewClient = newClientWithMockPortForward
	commands.NotebookStdout = &stdout
	commands.NewUUID = func() string { return testUUID }

	//var buf bytes.Buffer
	logf.SetLogger(zap.New(
		zap.UseDevMode(true),
		//zap.WriteTo(&buf),
	))

	ctx, cancel = context.WithCancel(context.TODO())

	log.Println("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	requireNoError(err)

	kubectlUser, err := testEnv.ControlPlane.AddUser(envtest.User{
		Name:   "kubectl-user",
		Groups: []string{"system:masters"},
	}, nil)
	requireNoError(err)
	kubeconfig, err := kubectlUser.KubeConfig()
	requireNoError(err)
	kubectlKubeconfigPath = filepath.Join(os.TempDir(), "kubeconfig.yaml")
	requireNoError(os.WriteFile(kubectlKubeconfigPath, kubeconfig, 0644))

	log.Printf("wrote test kubeconfig to: %s", kubectlKubeconfigPath)

	requireNoError(apiv1.AddToScheme(scheme.Scheme))

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	requireNoError(err)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	requireNoError(err)

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		log.Println("starting manager")
		err := mgr.Start(ctx)
		if err != nil {
			log.Printf("starting manager: %s", err)
		}
	}()

	log.Println("running tests")
	code := m.Run()

	// TODO: Run cleanup on ctrl-C, etc.
	log.Println("stopping manager")
	cancel()
	log.Println("stopping test environment")
	requireNoError(testEnv.Stop())

	fmt.Printf(`
======== Command Stdout ========
%s
================================
`, stdout.String())

	os.Exit(code)
}

func requireNoError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type mockClient struct {
	iclient.Interface
}

func (c *mockClient) PortForwardNotebook(ctx context.Context, verbose bool, nb *apiv1.Notebook, ready chan struct{}) error {
	log.Println("mockClient.PortForwardNotebook called")
	ready <- struct{}{}
	ctx.Done()
	return fmt.Errorf("mock PortForwardNotebook exiting because of ctx.Done()")
}

func newClientWithMockPortForward(inf kubernetes.Interface, cfg *rest.Config) iclient.Interface {
	return &mockClient{
		// Only mock PortForwardNotebook()
		Interface: iclient.NewClient(inf, cfg),
	}
}
