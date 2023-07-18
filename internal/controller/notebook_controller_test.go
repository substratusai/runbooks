package controller_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNotebookFromGitWithModelOnly(t *testing.T) {
	name := strings.ToLower(t.Name())

	model := &apiv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-mdl",
			Namespace: "default",
		},
		Spec: apiv1.ModelSpec{
			Image: apiv1.Image{
				Name: "some-image",
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, model), "create a model to be referenced by the notebook")
	t.Cleanup(debugObject(t, model))

	testModelLoad(t, model)

	notebook := &apiv1.Notebook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-nb",
			Namespace: "default",
		},
		Spec: apiv1.NotebookSpec{
			Command: []string{"notebook.sh"},
			Image: apiv1.Image{
				Git: &apiv1.GitSource{
					URL: "https://github.com/substratusai/notebook-test-test",
				},
			},
			Model: &apiv1.ObjectRef{
				Name: model.Name,
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, notebook), "creating a notebook")
	t.Cleanup(debugObject(t, notebook))

	testContainerBuild(t, notebook, "Notebook")

	// Test that a notebook Pod gets created by the controller.
	var pod corev1.Pod
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: notebook.Namespace, Name: notebook.Name + "-notebook"}, &pod)
		assert.NoError(t, err, "getting the notebook pod")
	}, timeout, interval, "waiting for the notebook pod to be created")
	require.Equal(t, "notebook", pod.Spec.Containers[0].Name)
	require.Contains(t, strings.Join(pod.Spec.Containers[0].Command, " "), "notebook.sh")

	fakePodReady(t, &pod)
	t.Cleanup(debugObject(t, &pod))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notebook), notebook)
		assert.NoError(t, err, "getting the notebook")
		assert.True(t, meta.IsStatusConditionTrue(notebook.Status.Conditions, apiv1.ConditionDeployed))
		assert.True(t, notebook.Status.Ready)
	}, timeout, interval, "waiting for the notebook to be ready")
}
