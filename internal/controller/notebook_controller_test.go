package controller_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNotebookFromGitWithModelOnly(t *testing.T) {
	name := strings.ToLower(t.Name())

	model := &apiv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-mdl",
			Namespace: "default",
		},
		Spec: apiv1.ModelSpec{
			Container: apiv1.Container{
				Image: "some-image",
			},
			Loader: &apiv1.ModelLoader{},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, model), "create a model to be referenced by the notebook")

	fakeModelLoad(t, model)

	notebook := &apiv1.Notebook{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-nb",
			Namespace: "default",
		},
		Spec: apiv1.NotebookSpec{
			Container: apiv1.Container{
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

	fakeContainerBuild(t, notebook)

	// Test that a notebook Pod gets created by the controller.
	var pod corev1.Pod
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: notebook.Namespace, Name: notebook.Name + "-notebook"}, &pod)
		assert.NoError(t, err, "getting the notebook pod")
	}, timeout, interval, "waiting for the notebook pod to be created")
	require.Equal(t, "notebook", pod.Spec.Containers[0].Name)
	require.Contains(t, strings.Join(pod.Spec.Containers[0].Args, " "), "notebook.sh")
}
