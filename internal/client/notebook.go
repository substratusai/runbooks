package client

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	apiv1 "github.com/substratusai/substratus/api/v1"
)

func PodForNotebook(nb *apiv1.Notebook) types.NamespacedName {
	// TODO: Pull Pod info from status of Notebook.
	return types.NamespacedName{
		Namespace: nb.GetNamespace(),
		Name:      nb.GetName() + "-notebook",
	}
}

func NotebookForObject(obj Object) (*apiv1.Notebook, error) {
	var nb *apiv1.Notebook

	switch obj := obj.DeepCopyObject().(type) {
	case *apiv1.Notebook:
		nb = obj

	case *apiv1.Model:
		nb = &apiv1.Notebook{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obj.Name + "-model",
				Namespace: obj.Namespace,
			},
			Spec: apiv1.NotebookSpec{
				Image:     obj.Spec.Image,
				Env:       obj.Spec.Env,
				Params:    obj.Spec.Params,
				Model:     obj.Spec.Model,
				Dataset:   obj.Spec.Dataset,
				Resources: obj.Spec.Resources,
			},
		}

	case *apiv1.Server:
		nb = &apiv1.Notebook{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obj.Name + "-server",
				Namespace: obj.Namespace,
			},
			Spec: apiv1.NotebookSpec{
				Image:     obj.Spec.Image,
				Env:       obj.Spec.Env,
				Params:    obj.Spec.Params,
				Model:     &obj.Spec.Model,
				Resources: obj.Spec.Resources,
			},
		}

	case *apiv1.Dataset:
		nb = &apiv1.Notebook{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obj.Name + "-dataset",
				Namespace: obj.Namespace,
			},
			Spec: apiv1.NotebookSpec{
				Image:     obj.Spec.Image,
				Env:       obj.Spec.Env,
				Params:    obj.Spec.Params,
				Resources: obj.Spec.Resources,
			},
		}

	default:
		return nil, fmt.Errorf("unknown object type: %T", obj)
	}

	// "This field is managed by the API server and should not be changed by the user."
	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management
	nb.ObjectMeta.ManagedFields = nil

	nb.TypeMeta = metav1.TypeMeta{
		APIVersion: "substratus.ai/v1",
		Kind:       "Notebook",
	}

	return nb, nil
}
