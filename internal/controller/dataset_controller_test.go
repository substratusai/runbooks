package controller_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
			Source: apiv1.DatasetSource{
				URL:      "https://test.internal/does/not/exist.jsonl",
				Filename: "does-not-exist.jsonl",
			},
			Size: resource.MustParse("1Gi"),
		},
	}
	require.NoError(t, k8sClient.Create(ctx, dataset), "create a dataset")

	// Test that a data puller ServiceAccount gets created by the controller.
	var sa corev1.ServiceAccount
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: dataset.Namespace, Name: "data-puller"}, &sa)
		assert.NoError(t, err, "getting the data puller serviceaccount")
	}, timeout, interval, "waiting for the data puller serviceaccount to be created")
	require.Equal(t, "substratus-data-puller@test-project-id.iam.gserviceaccount.com", sa.Annotations["iam.gke.io/gcp-service-account"])

	// Test that a data puller Job gets created by the controller.
	var job batchv1.Job
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: dataset.Namespace, Name: dataset.Name + "-data-puller"}, &job)
		assert.NoError(t, err, "getting the data puller job")
	}, timeout, interval, "waiting for the data puller job to be created")
	require.Equal(t, "puller", job.Spec.Template.Spec.Containers[0].Name)
}
