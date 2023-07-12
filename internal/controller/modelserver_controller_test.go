package controller_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestModelServerFromGit(t *testing.T) {
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
	require.NoError(t, k8sClient.Create(ctx, model), "create a model to be referenced by the modelserver")
	t.Cleanup(debugObject(t, model))

	testModelLoad(t, model)

	modelServer := &apiv1.ModelServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-nb",
			Namespace: "default",
		},
		Spec: apiv1.ModelServerSpec{
			Command: []string{"serve.sh"},
			Image: apiv1.Image{
				Git: &apiv1.GitSource{
					URL: "https://github.com/substratusai/some-modelserver",
				},
			},
			Model: apiv1.ObjectRef{
				Name: model.Name,
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, modelServer), "creating a modelserver")
	t.Cleanup(debugObject(t, modelServer))

	testContainerBuild(t, modelServer, "ModelServer")

	// Test that a model server Service gets created by the controller.
	var service corev1.Service
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: modelServer.Namespace, Name: modelServer.Name + "-modelserver"}, &service)
		assert.NoError(t, err, "getting the modelserver service")
	}, timeout, interval, "waiting for the server service to be created")
	require.Equal(t, "http-serve", service.Spec.Ports[0].TargetPort.String())

	// Test that a model server Deployment gets created by the controller.
	var deploy appsv1.Deployment
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: modelServer.Namespace, Name: modelServer.Name + "-modelserver"}, &deploy)
		assert.NoError(t, err, "getting the modelserver deployment")
	}, timeout, interval, "waiting for the server deployment to be created")
	require.Equal(t, "serve", deploy.Spec.Template.Spec.Containers[0].Name)
	require.Contains(t, strings.Join(deploy.Spec.Template.Spec.Containers[0].Command, " "), "serve.sh")
}
