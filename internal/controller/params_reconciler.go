package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ParameterizedObject interface {
	client.Object
	GetParams() map[string]intstr.IntOrString
}

type ParamsReconciler struct {
	Scheme *runtime.Scheme
	Client client.Client
}

func (r *ParamsReconciler) ReconcileParamsConfigMap(ctx context.Context, obj ParameterizedObject) (result, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      paramsConfigMapName(obj),
		},
	}

	configure := func() error {
		var contents []byte
		if params := obj.GetParams(); len(params) > 0 {
			var err error
			contents, err = json.MarshalIndent(obj.GetParams(), "", "  ")
			if err != nil {
				return fmt.Errorf("marshalling params to json: %w", err)
			}
		} else {
			// At least pass params.json through as an empty object: {}
			contents = []byte("{}")
		}

		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data["params.json"] = string(contents)

		if err := ctrl.SetControllerReference(obj, cm, r.Scheme); err != nil {
			if _, already := err.(*controllerutil.AlreadyOwnedError); !already {
				return fmt.Errorf("setting controller reference: %w", err)
			}
		}

		return nil
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, configure); err != nil {
		return result{}, fmt.Errorf("failed to create or update configmap: %w", err)
	}

	return result{success: true}, nil
}

func paramsConfigMapName(obj client.Object) string {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		panic("empty kind")
	}
	return obj.GetName() + "-" + strings.ToLower(kind) + "-params"
}

func mountParamsConfigMap(podSpec *corev1.PodSpec, obj ParameterizedObject, container string) error {
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "params",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: paramsConfigMapName(obj),
				},
			},
		},
	})

	for i := range podSpec.Containers {
		if podSpec.Containers[i].Name == container {
			podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts,
				corev1.VolumeMount{
					Name:      "params",
					MountPath: "/content/params.json",
					SubPath:   "params.json",
				},
			)
			return nil
		}
	}

	return fmt.Errorf("container not found: %s", container)
}
