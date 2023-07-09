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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestModelLoaderFromGit(t *testing.T) {
	name := strings.ToLower(t.Name())

	model := &apiv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-mdl",
			Namespace: "default",
		},
		Spec: apiv1.ModelSpec{
			Container: apiv1.Container{
				Git: &apiv1.GitSource{
					URL: "https://test.com/test/test.git",
				},
			},
			Loader: &apiv1.ModelLoader{},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, model), "create a model that references a git repository")

	// Test that a model builder ServiceAccount gets created by the controller.
	var sa corev1.ServiceAccount
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: "model-builder"}, &sa)
		assert.NoError(t, err, "getting the model builder serviceaccount")
	}, timeout, interval, "waiting for the image builder serviceaccount to be created")
	require.Equal(t, "substratus-model-builder@test-project-id.iam.gserviceaccount.com", sa.Annotations["iam.gke.io/gcp-service-account"])

	// Test that a model builder Job gets created by the controller.
	var job batchv1.Job
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Name + "-model-builder"}, &job)
		assert.NoError(t, err, "getting the model builder job")
	}, timeout, interval, "waiting for the image builder job to be created")
	require.Equal(t, "builder", job.Spec.Template.Spec.Containers[0].Name)
}

func TestModelTrainerFromGit(t *testing.T) {
	name := strings.ToLower(t.Name())

	baseModel := &apiv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-base-mdl",
			Namespace: "default",
		},
		Spec: apiv1.ModelSpec{
			Container: apiv1.Container{
				Image: "some-test-image",
			},
			Loader: &apiv1.ModelLoader{},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, baseModel), "create a model to be referenced by the trained model")

	fakeModelLoad(t, baseModel)

	dataset := &apiv1.Dataset{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-ds",
			Namespace: "default",
		},
		Spec: apiv1.DatasetSpec{
			Filename: "does-not-exist.jsonl",
			Container: apiv1.Container{
				Git: &apiv1.GitSource{
					URL: "https://github.com/substratusai/dataset-test-test",
				},
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, dataset), "create a dataset to be referenced by the trained model")
	datasetWithUpdatedStatus := dataset.DeepCopy()
	datasetWithUpdatedStatus.Status.URL = "gs://test-bucket/test.jsonl"
	datasetWithUpdatedStatus.Status.Ready = true
	require.NoError(t, k8sClient.Status().Patch(ctx, datasetWithUpdatedStatus, client.MergeFrom(dataset)), "patching the dataset with a fake ready status")

	trainedModel := &apiv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-trained-mdl",
			Namespace: baseModel.Namespace,
		},
		Spec: apiv1.ModelSpec{
			Container: apiv1.Container{
				Git: &apiv1.GitSource{
					URL: "https://test.com/test/test",
				},
			},
			Trainer: &apiv1.ModelTrainer{
				BaseModel: &apiv1.ObjectRef{
					Name: baseModel.Name,
				},
				Dataset: apiv1.ObjectRef{
					Name: dataset.Name,
				},
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, trainedModel), "creating a model that references another model for training")

	// Test that a model trainer ServiceAccount gets created by the controller.
	var sa corev1.ServiceAccount
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: trainedModel.Namespace, Name: "model-trainer"}, &sa)
		assert.NoError(t, err, "getting the model trainer serviceaccount")
	}, timeout, interval, "waiting for the image trainer serviceaccount to be created")
	require.Equal(t, "substratus-model-trainer@test-project-id.iam.gserviceaccount.com", sa.Annotations["iam.gke.io/gcp-service-account"])

	// Test that a trainer Pod gets created by the controller.
	var job batchv1.Job
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: trainedModel.Namespace, Name: trainedModel.Name + "-model-trainer"}, &job)
		assert.NoError(t, err, "getting the model training job")
	}, timeout, interval, "waiting for the model training job to be created")
	require.Equal(t, "trainer", job.Spec.Template.Spec.Containers[0].Name)
	require.Contains(t, strings.Join(job.Spec.Template.Spec.Containers[0].Args, " "), "train.sh")

	// TODO: Test build Job after training Job.
}
