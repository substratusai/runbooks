package controller_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/controller"
	//+kubebuilder:scaffold:imports
)

const (
	timeout  = time.Second * 5
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
	requireNoError(controller.SetupIndexes(mgr))

	testCloud := &cloud.GCP{}
	testCloud.ProjectID = "test-project-id"
	testCloud.ClusterName = "test-cluster-name"
	testCloud.ClusterLocation = "us-central1"
	testCloud.ArtifactBucketURL = &cloud.BucketURL{Scheme: "gs", Bucket: "test-artifact-bucket"}
	testCloud.RegistryURL = "registry.test"

	//runtimeMgr, err := controller.NewRuntimeManager(controller.GPUTypeNvidiaL4)
	//requireNoError(err)

	err = (&controller.ModelReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Cloud:  testCloud,
		ContainerImageReconciler: &controller.ContainerImageReconciler{
			Scheme: mgr.GetScheme(),
			Client: mgr.GetClient(),
			Cloud:  testCloud,
			Kind:   "Model",
		},
	}).SetupWithManager(mgr)
	requireNoError(err)
	err = (&controller.ServerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		ContainerImageReconciler: &controller.ContainerImageReconciler{
			Scheme: mgr.GetScheme(),
			Client: mgr.GetClient(),
			Cloud:  testCloud,
			Kind:   "Server",
		},
	}).SetupWithManager(mgr)
	requireNoError(err)
	err = (&controller.NotebookReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		ContainerImageReconciler: &controller.ContainerImageReconciler{
			Scheme: mgr.GetScheme(),
			Client: mgr.GetClient(),
			Cloud:  testCloud,
			Kind:   "Notebook",
		},
	}).SetupWithManager(mgr)
	requireNoError(err)
	err = (&controller.DatasetReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Cloud:  testCloud,
		ContainerImageReconciler: &controller.ContainerImageReconciler{
			Scheme: mgr.GetScheme(),
			Client: mgr.GetClient(),
			Cloud:  testCloud,
			Kind:   "Dataset",
		},
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

type testObject interface {
	client.Object
	GetConditions() *[]metav1.Condition
	GetStatusReady() bool
	SetStatusReady(bool)
	GetImage() *apiv1.Image
}

func testContainerBuild(t *testing.T, obj testObject, kind string) {
	var sa corev1.ServiceAccount
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: "container-builder"}, &sa)
		assert.NoError(t, err, "getting the container builder serviceaccount")
	}, timeout, interval, "waiting for the container builder serviceaccount to be created")
	require.Equal(t, "substratus-container-builder@test-project-id.iam.gserviceaccount.com", sa.Annotations["iam.gke.io/gcp-service-account"])

	// Test that a container builder Job gets created by the controller.
	var builderJob batchv1.Job
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName() + "-" + strings.ToLower(kind) + "-container-builder"}, &builderJob)
		assert.NoError(t, err, "getting the container builder job")
	}, timeout, interval, "waiting for the container builder job to be created")
	require.Equal(t, "builder", builderJob.Spec.Template.Spec.Containers[0].Name)

	fakeJobComplete(t, &builderJob)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}, obj)
		assert.NoError(t, err, "getting object")
		assert.True(t, meta.IsStatusConditionTrue(*obj.GetConditions(), apiv1.ConditionBuilt))
	}, timeout, interval, "waiting for the container to be ready")
}

func fakeJobComplete(t *testing.T, job *batchv1.Job) {
	updated := job.DeepCopy()
	updated.Status.Succeeded = 1
	require.NoError(t, k8sClient.Status().Patch(ctx, updated, client.MergeFrom(job)), "patching the job with completed count")
}

func fakePodReady(t *testing.T, pod *corev1.Pod) {
	updated := pod.DeepCopy()
	updated.Status.Phase = corev1.PodRunning
	updated.Status.Conditions = append(updated.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})
	require.NoError(t, k8sClient.Status().Patch(ctx, updated, client.MergeFrom(pod)), "patching the pod with ready status")
}

func debugObject(t *testing.T, obj client.Object) func() {
	return func() {
		if !t.Failed() {
			return
		}
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			fmt.Printf("TEST DEBUG: Error getting object: %v\n", err)
		}
		pretty, err := json.MarshalIndent(obj, "", "    ")
		if err != nil {
			fmt.Printf("TEST DEBUG: Marshalling object: %v\n", err)
		}
		fmt.Printf("TEST DEBUG: %T: %v\n", obj, string(pretty))
	}
}
