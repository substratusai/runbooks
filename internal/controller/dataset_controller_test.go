package controller_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDataset(t *testing.T) {
	name := strings.ToLower(t.Name())

	dataset := &apiv1.Dataset{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-ds",
			Namespace: "default",
		},
		Spec: apiv1.DatasetSpec{
			Build: &apiv1.Build{
				Git: &apiv1.BuildGit{
					URL: "https://github.com/substratusai/dataset-some-dataset",
				},
			},
			Params: map[string]intstr.IntOrString{
				"s": intstr.FromString("something-dataset"),
				"x": intstr.FromInt(123),
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, dataset), "create a dataset")
	t.Cleanup(debugObject(t, dataset))
	t.Cleanup(debugObject(t, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: dataset.Namespace, Name: dataset.Name + "-data-loader"}}))

	testContainerBuild(t, dataset, "Dataset")
	testParamsConfigMap(t, dataset, "Dataset", `{ "s": "something-dataset", "x": 123 }`)

	testDatasetLoad(t, dataset)
}

func testDatasetLoad(t *testing.T, dataset *apiv1.Dataset) {
	// Test that a data loader ServiceAccount gets created by the controller.
	var sa corev1.ServiceAccount
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: dataset.Namespace, Name: "data-loader"}, &sa)
		assert.NoError(t, err, "getting the data loader serviceaccount")
	}, timeout, interval, "waiting for the data loader serviceaccount to be created")
	require.Equal(t, "substratus@test-project-id.iam.gserviceaccount.com", sa.Annotations["iam.gke.io/gcp-service-account"])

	// Test that a data loader builder Job gets created by the controller.
	var loaderJob batchv1.Job
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: dataset.Namespace, Name: dataset.Name + "-data-loader"}, &loaderJob)
		assert.NoError(t, err, "getting the data loader job")
	}, timeout, interval, "waiting for the data loader job to be created")
	require.Equal(t, "load", loaderJob.Spec.Template.Spec.Containers[0].Name)

	fakeJobComplete(t, &loaderJob)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dataset), dataset)
		assert.NoError(t, err, "getting the dataset")
		assert.True(t, meta.IsStatusConditionTrue(dataset.Status.Conditions, apiv1.ConditionLoaded))
		assert.True(t, dataset.Status.Ready)
	}, timeout, interval, "waiting for the dataset to be ready")
	require.Contains(t, dataset.Status.Artifacts.URL, "gs://test-artifact-bucket")

}
