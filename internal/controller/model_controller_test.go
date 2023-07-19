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
)

func TestModelLoaderFromGit(t *testing.T) {
	name := strings.ToLower(t.Name())

	model := &apiv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-mdl",
			Namespace: "default",
		},
		Spec: apiv1.ModelSpec{
			Image: apiv1.Image{
				Git: &apiv1.GitSource{
					URL: "https://test.internal/test/model-loader.git",
				},
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, model), "create a model that references a git repository")

	testContainerBuild(t, model, "Model")

	testModelLoad(t, model)
}

func testModelLoad(t *testing.T, model *apiv1.Model) {
	// Test that a container loader Job gets created by the controller.
	var loaderJob batchv1.Job
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: model.GetNamespace(), Name: model.GetName() + "-modeller"}, &loaderJob)
		assert.NoError(t, err, "getting the model loader job")
	}, timeout, interval, "waiting for the  model loader job to be created")
	require.Equal(t, "model", loaderJob.Spec.Template.Spec.Containers[0].Name)

	fakeJobComplete(t, &loaderJob)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: model.GetNamespace(), Name: model.GetName()}, model)
		assert.NoError(t, err, "getting model")
		assert.True(t, meta.IsStatusConditionTrue(model.Status.Conditions, apiv1.ConditionModelled))
		assert.True(t, model.Status.Ready)
	}, timeout, interval, "waiting for the model to be ready")
	require.Contains(t, model.Status.Artifacts.URL, "gs://test-artifact-bucket")
}

func TestModelTrainerFromGit(t *testing.T) {
	name := strings.ToLower(t.Name())

	baseModel := &apiv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-base-mdl",
			Namespace: "default",
		},
		Spec: apiv1.ModelSpec{
			Image: apiv1.Image{
				Name: "some-test-image",
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, baseModel), "create a model to be referenced by the trained model")

	t.Cleanup(debugObject(t, baseModel))

	testModelLoad(t, baseModel)

	dataset := &apiv1.Dataset{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-ds",
			Namespace: "default",
		},
		Spec: apiv1.DatasetSpec{
			Image: apiv1.Image{
				Name: "some-image",
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, dataset), "create a dataset to be referenced by the trained model")

	t.Cleanup(debugObject(t, dataset))

	testDatasetLoad(t, dataset)

	trainedModel := &apiv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-trained-mdl",
			Namespace: baseModel.Namespace,
		},
		Spec: apiv1.ModelSpec{
			Command: []string{"model.sh"},
			Image: apiv1.Image{
				Git: &apiv1.GitSource{
					URL: "https://test.com/test/test",
				},
			},
			BaseModel: &apiv1.ObjectRef{
				Name: baseModel.Name,
			},
			TrainingDataset: &apiv1.ObjectRef{
				Name: dataset.Name,
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, trainedModel), "creating a model that references another model for training")

	t.Cleanup(debugObject(t, trainedModel))

	testContainerBuild(t, trainedModel, "Model")

	testModelTrain(t, trainedModel)
}

func testModelTrain(t *testing.T, model *apiv1.Model) {
	// Test that a model trainer ServiceAccount gets created by the controller.
	var sa corev1.ServiceAccount
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: "modeller"}, &sa)
		assert.NoError(t, err, "getting the model trainer serviceaccount")
	}, timeout, interval, "waiting for the model trainer serviceaccount to be created")
	require.Equal(t, "substratus-modeller@test-project-id.iam.gserviceaccount.com", sa.Annotations["iam.gke.io/gcp-service-account"])

	// Test that a trainer Job gets created by the controller.
	var job batchv1.Job
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Name + "-modeller"}, &job)
		assert.NoError(t, err, "getting the model training job")
	}, timeout, interval, "waiting for the model training job to be created")
	require.Equal(t, "model", job.Spec.Template.Spec.Containers[0].Name)
	require.Contains(t, strings.Join(job.Spec.Template.Spec.Containers[0].Command, " "), "model.sh")

	fakeJobComplete(t, &job)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: model.GetNamespace(), Name: model.GetName()}, model)
		assert.NoError(t, err, "getting model")
		assert.True(t, meta.IsStatusConditionTrue(model.Status.Conditions, apiv1.ConditionModelled))
		assert.True(t, model.Status.Ready)
	}, timeout, interval, "waiting for the model to be ready")
	require.Contains(t, model.Status.Artifacts.URL, "gs://test-artifact-bucket")
}
