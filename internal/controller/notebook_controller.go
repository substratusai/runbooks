package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/substratusai/substratus/api/v1"
)

const (
	ReasonSuspended = "Suspended"
)

// NotebookReconciler reconciles a Notebook object.
type NotebookReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	*RuntimeManager
}

//+kubebuilder:rbac:groups=substratus.ai,resources=notebooks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=notebooks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=notebooks/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

func (r *NotebookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	lg.Info("Reconciling Notebook")
	defer lg.Info("Done reconciling Notebook")

	var notebook apiv1.Notebook
	if err := r.Get(ctx, req.NamespacedName, &notebook); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var model apiv1.Model
	if err := r.Get(ctx, client.ObjectKey{Name: notebook.Spec.ModelName, Namespace: notebook.Namespace}, &model); err != nil {
		return ctrl.Result{}, fmt.Errorf("model not found: %w", err)
	}

	if ready := meta.FindStatusCondition(model.Status.Conditions, ConditionReady); ready == nil || ready.Status != metav1.ConditionTrue {
		lg.Info("Model not ready", "model", model.Name)

		meta.SetStatusCondition(&notebook.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             ReasonModelNotReady,
			ObservedGeneration: notebook.Generation,
		})
		if err := r.Status().Update(ctx, &notebook); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update notebook status: %w", err)
		}

		return ctrl.Result{}, nil
	}

	//pvc, err := r.notebookPVC(&notebook)
	//if err != nil {
	//	return ctrl.Result{}, fmt.Errorf("failed to construct pvc: %w", err)
	//}

	//if err := r.Patch(ctx, pvc, client.Apply, client.FieldOwner("notebook-controller")); err != nil {
	//	return ctrl.Result{}, fmt.Errorf("failed to apply pvc: %w", err)
	//}

	pod, err := r.notebookPod(&notebook, &model)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to construct pod: %w", err)
	}
	if notebook.Spec.Suspend {
		meta.SetStatusCondition(&notebook.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             ReasonSuspended,
			ObservedGeneration: notebook.Generation,
		})
		if err := r.Status().Update(ctx, &notebook); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating notebook status: %w", err)
		}

		if err := r.Delete(ctx, pod); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	} else {
		if err := r.Patch(ctx, pod, client.Apply, client.FieldOwner("notebook-controller")); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to apply pod: %w", err)
		}
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get pod: %w", err)
	}

	ready := metav1.ConditionFalse
	reason := string(pod.Status.Phase)
	if pod.Status.Phase == corev1.PodRunning {
		for _, c := range pod.Status.Conditions {
			if c.Type == "Ready" {
				if c.Status == "True" {
					ready = metav1.ConditionTrue
					reason = reason + "AndReady"
				} else {
					reason = reason + "AndNotReady"
				}
			}
		}
	}

	meta.SetStatusCondition(&notebook.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             ready,
		Reason:             reason,
		ObservedGeneration: notebook.Generation,
	})
	if err := r.Status().Update(ctx, &notebook); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating notebook status: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NotebookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Notebook{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

func (r *NotebookReconciler) notebookPod(nb *apiv1.Notebook, model *apiv1.Model) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nb.Name + "-notebook",
			Namespace: nb.Namespace,
		},
		Spec: corev1.PodSpec{
			//SecurityContext: &corev1.PodSecurityContext{
			//	RunAsUser:  int64Ptr(1000),
			//	RunAsGroup: int64Ptr(100),
			//	FSGroup:    int64Ptr(100),
			//},
			Containers: []corev1.Container{
				{
					Name:  RuntimeNotebook,
					Image: model.Status.ContainerImage,
					Command: []string{
						"jupyter", "notebook", "--allow-root", "--ip=0.0.0.0", "--NotebookApp.token=''", "--notebook-dir='/model'",
					},
					//WorkingDir: "/home/jovyan",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8888,
						},
					},
					// TODO: GPUs
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/api",
								Port: intstr.FromInt(8888),
							},
						},
					},
					//VolumeMounts: []corev1.VolumeMount{
					//	{
					//		Name:      "notebook",
					//		MountPath: "/home/jovyan",
					//	},
					//},
				},
			},
			//Volumes: []corev1.Volume{
			//	{
			//		Name: "notebook",
			//		VolumeSource: corev1.VolumeSource{
			//			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			//				ClaimName: notebookPVCName(nb),
			//			},
			//		},
			//	},
			//},
		},
	}

	if err := r.SetResources(model, &pod.ObjectMeta, &pod.Spec, RuntimeNotebook); err != nil {
		return nil, fmt.Errorf("setting pod resources: %w", err)
	}

	if err := ctrl.SetControllerReference(nb, pod, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return pod, nil

}

func notebookPVCName(nb *apiv1.Notebook) string {
	return nb.Name + "-notebook"
}

func (r *NotebookReconciler) notebookPVC(nb *apiv1.Notebook) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      notebookPVCName(nb),
			Namespace: nb.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				"ReadWriteOnce",
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("10Gi"),
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(nb, pvc, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return pvc, nil
}
