package client

import (
	"fmt"

	apiv1 "github.com/substratusai/substratus/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NotebookForObject(obj Object) (*apiv1.Notebook, error) {
	var nb *apiv1.Notebook

	switch obj := obj.DeepCopyObject().(type) {
	case *apiv1.Notebook:
		nb = obj

	case *apiv1.Model:
		nb = &apiv1.Notebook{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obj.Name,
				Namespace: obj.Namespace,
			},
			Spec: apiv1.NotebookSpec{
				// TODO: How to map base model / saved model to notebook mounts?
				Image:  obj.Spec.Image,
				Params: obj.Spec.Params,
			},
		}

	case *apiv1.Server:
		nb = &apiv1.Notebook{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obj.Name,
				Namespace: obj.Namespace,
			},
			Spec: apiv1.NotebookSpec{
				Image: obj.Spec.Image,
				Model: &obj.Spec.Model,
			},
		}
	case *apiv1.Dataset:
		nb = &apiv1.Notebook{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obj.Name,
				Namespace: obj.Namespace,
			},
			Spec: apiv1.NotebookSpec{
				Image: obj.Spec.Image,
				Dataset: &apiv1.ObjectRef{
					Name: obj.Name,
				},
				Params: obj.Spec.Params,
			},
		}
	default:
		return nil, fmt.Errorf("unknown object type: %T", obj)
	}

	nb.ObjectMeta.ManagedFields = nil
	nb.TypeMeta = metav1.TypeMeta{
		APIVersion: "substratus.ai/v1",
		Kind:       "Notebook",
	}

	return nb, nil
}
