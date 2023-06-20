package controller_test

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/controller"
	//+kubebuilder:scaffold:imports
)

const (
	timeout  = time.Second * 10
	interval = time.Second / 10
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestMain(m *testing.M) {
	//var buf bytes.Buffer
	logf.SetLogger(zap.New(
		zap.UseDevMode(true),
		//zap.WriteTo(&buf),
	))

	ctx, cancel = context.WithCancel(context.TODO())

	log.Println("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	requireNoError(err)

	requireNoError(apiv1.AddToScheme(scheme.Scheme))

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	requireNoError(err)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	requireNoError(err)

	cloudContext := &mockCloudContext{}

	gpuType := controller.GPUTypeNone

	err = (&controller.ModelReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Config: controller.ModelReconcilerConfig{
			ImageRegistry: "test.internal",
		},
		CloudContext: cloudContext,
		GPUType:      gpuType,
	}).SetupWithManager(mgr)
	requireNoError(err)
	err = (&controller.ModelServerReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		GPUType: gpuType,
	}).SetupWithManager(mgr)
	requireNoError(err)
	err = (&controller.NotebookReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		GPUType: gpuType,
	}).SetupWithManager(mgr)
	requireNoError(err)
	err = (&controller.DatasetReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		CloudContext: cloudContext,
	}).SetupWithManager(mgr)
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

	os.Exit(code)
}

func requireNoError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func slurpTestFile(t *testing.T, filename string) string {
	_, testFilename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(testFilename)
	contents, err := ioutil.ReadFile(filepath.Join(dir, "tests", filename))
	require.NoError(t, err)

	return string(contents)
}

// Assert that the mock implements the interface.
var _ controller.CloudContext = &mockCloudContext{}

type mockCloudContext struct{}

func (m *mockCloudContext) AuthNServiceAccount(runtime controller.Runtime, sa *corev1.ServiceAccount) error {
	if sa.Annotations == nil {
		sa.Annotations = map[string]string{}
	}
	sa.Annotations["test-cloud-authn"] = "set-by-mock-cloud-context"
	return nil
}
