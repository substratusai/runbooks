package client

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/substratusai/substratus/api/v1"
)

type NotebookPurpose int

const (
	// NotebookPurposeDevelop simulates the environment an Object will run in.
	NotebookPurposeDevelop = iota
	// NotebookPurposeReview mounts existing artifacts.
	NotebookPurposeReview = iota
)

func NotebookForObject(obj Object, purpose NotebookPurpose) (*apiv1.Notebook, error) {
	var nb *apiv1.Notebook

	switch obj := obj.DeepCopyObject().(type) {
	case *apiv1.Notebook:
		nb = obj

	case *apiv1.Model:
		switch purpose {
		case NotebookPurposeDevelop:
			nb = &apiv1.Notebook{
				ObjectMeta: metav1.ObjectMeta{
					Name:      obj.Name + "-model-develop",
					Namespace: obj.Namespace,
				},
				Spec: apiv1.NotebookSpec{
					Image:  obj.Spec.Image,
					Params: obj.Spec.Params,
					// Empty ObjectRef signals that we would like to mount an empty volume.
					Model:     &apiv1.ObjectRef{},
					BaseModel: obj.Spec.Model,
					Dataset:   obj.Spec.Dataset,
				},
			}
		case NotebookPurposeReview:
			nb = &apiv1.Notebook{
				ObjectMeta: metav1.ObjectMeta{
					Name:      obj.Name + "-model-review",
					Namespace: obj.Namespace,
				},
				Spec: apiv1.NotebookSpec{
					Image:  obj.Spec.Image,
					Params: obj.Spec.Params,
					Model: &apiv1.ObjectRef{
						Name: obj.Name,
					},
				},
			}
		}

	case *apiv1.Server:
		return nil, fmt.Errorf("notebooks for servers are not yet supported")
		/*
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
		*/

	case *apiv1.Dataset:
		return nil, fmt.Errorf("notebooks for datasets are not yet supported")
		// NOTE: For this to work for Dataset development purposes, the Notebook
		// controllers needs to mount a directory to receive the dataset.
		// (/content/data).
		/*
			nb = &apiv1.Notebook{
				ObjectMeta: metav1.ObjectMeta{
					Name:      obj.Name,
					Namespace: obj.Namespace,
				},
				Spec: apiv1.NotebookSpec{
					Build: obj.Spec.Build,
					//Dataset: &apiv1.ObjectRef{
					//	Name: obj.Name,
					//},
					Params: obj.Spec.Params,
				},
			}
		*/

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
