package controller_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNotebook(t *testing.T) {
	name := strings.ToLower(t.Name())

	model := &apiv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-mdl",
			Namespace: "default",
		},
		Spec: apiv1.ModelSpec{
			Source: apiv1.ModelSource{},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, model), "create a model to be referenced by the notebook")
	modelWithUpdatedStatus := model.DeepCopy()
	modelWithUpdatedStatus.Status.ContainerImage = "test"
	meta.SetStatusCondition(&modelWithUpdatedStatus.Status.Conditions, metav1.Condition{
		Type:   controller.ConditionReady,
		Status: metav1.ConditionTrue,
		Reason: "FakedByTheTest",
	})
	require.NoError(t, k8sClient.Status().Patch(ctx, modelWithUpdatedStatus, client.MergeFrom(model)), "patching the model with a fake ready status")

	notebook := &apiv1.Notebook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-nb",
			Namespace: "default",
		},
		Spec: apiv1.NotebookSpec{
			ModelName: model.Name,
		},
	}
	require.NoError(t, k8sClient.Create(ctx, notebook), "creating a notebook")

	// Test that a notebook Pod gets created by the controller.
	var pod corev1.Pod
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: notebook.Namespace, Name: notebook.Name + "-notebook"}, &pod)
		assert.NoError(t, err, "getting the notebook pod")
	}, timeout, interval, "waiting for the notebook pod to be created")
	require.Equal(t, "notebook", pod.Spec.Containers[0].Name)
	require.Contains(t, strings.Join(pod.Spec.Containers[0].Command, " "), "jupyter")
}
