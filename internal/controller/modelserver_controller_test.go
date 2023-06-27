package controller_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/controller"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestModelServer(t *testing.T) {
	name := strings.ToLower(t.Name())

	model := &apiv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-mdl",
			Namespace: "default",
		},
		Spec: apiv1.ModelSpec{
			Source: apiv1.ModelSource{
				Git: &apiv1.GitSource{
					URL: "test.com/test/test.git",
				},
			},
			Compute: apiv1.ModelCompute{
				Types: []apiv1.ComputeType{apiv1.ComputeTypeCPU},
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, model), "create a model to be referenced by the modelserver")
	modelWithUpdatedStatus := model.DeepCopy()
	modelWithUpdatedStatus.Status.ContainerImage = "test"
	meta.SetStatusCondition(&modelWithUpdatedStatus.Status.Conditions, metav1.Condition{
		Type:   controller.ConditionReady,
		Status: metav1.ConditionTrue,
		Reason: "FakedByTheTest",
	})
	require.NoError(t, k8sClient.Status().Patch(ctx, modelWithUpdatedStatus, client.MergeFrom(model)), "patching the model with a fake ready status")

	modelServer := &apiv1.ModelServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-nb",
			Namespace: "default",
		},
		Spec: apiv1.ModelServerSpec{
			ModelName: model.Name,
		},
	}
	require.NoError(t, k8sClient.Create(ctx, modelServer), "creating a modelserver")

	// Test that a model server Service gets created by the controller.
	var service corev1.Service
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: modelServer.Namespace, Name: modelServer.Name + "-modelserver"}, &service)
		assert.NoError(t, err, "getting the modelserver service")
	}, timeout, interval, "waiting for the server service to be created")
	require.Equal(t, "http-app", service.Spec.Ports[0].TargetPort.String())

	// Test that a model server Deployment gets created by the controller.
	var deploy appsv1.Deployment
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		err := k8sClient.Get(ctx, types.NamespacedName{Namespace: modelServer.Namespace, Name: modelServer.Name + "-modelserver"}, &deploy)
		assert.NoError(t, err, "getting the modelserver deployment")
	}, timeout, interval, "waiting for the server deployment to be created")
	require.Equal(t, "server", deploy.Spec.Template.Spec.Containers[0].Name)
	require.Contains(t, strings.Join(deploy.Spec.Template.Spec.Containers[0].Command, " "), "serve.sh")
}
