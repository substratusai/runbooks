package controller_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestDataset(t *testing.T) {
	name := strings.ToLower(t.Name())

	dataset := &apiv1.Dataset{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-ds",
			Namespace: "default",
		},
		Spec: apiv1.DatasetSpec{
			Filename: "does-not-exist.jsonl",
			Container: apiv1.Container{
				Git: &apiv1.GitSource{
					URL: "https://github.com/substratusai/dataset-some-dataset",
				},
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, dataset), "create a dataset")

	fakeContainerBuild(t, dataset)

	// Test that a data loader ServiceAccount gets created by the controller.
	var sa corev1.ServiceAccount
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: dataset.Namespace, Name: "data-loader"}, &sa)
		assert.NoError(t, err, "getting the data loader serviceaccount")
	}, timeout, interval, "waiting for the data loader serviceaccount to be created")
	require.Equal(t, "substratus-data-loader@test-project-id.iam.gserviceaccount.com", sa.Annotations["iam.gke.io/gcp-service-account"])

	// Test that a data loader builder Job gets created by the controller.
	var loaderJob batchv1.Job
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: dataset.Namespace, Name: dataset.Name + "-data-loader"}, &loaderJob)
		assert.NoError(t, err, "getting the data loader job")
	}, timeout, interval, "waiting for the data loader job to be created")
	require.Equal(t, "loader", loaderJob.Spec.Template.Spec.Containers[0].Name)
}
