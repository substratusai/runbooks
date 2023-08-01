package controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/resources"
)

// NotebookReconciler reconciles a Notebook object.
type NotebookReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Cloud cloud.Cloud

	*ParamsReconciler
}

func (r *NotebookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling Notebook")
	defer log.Info("Done reconciling Notebook")

	var notebook apiv1.Notebook
	if err := r.Get(ctx, req.NamespacedName, &notebook); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if notebook.GetImage() == "" {
		// Image must be building.
		return ctrl.Result{}, nil
	}

	if result, err := r.ReconcileParamsConfigMap(ctx, &notebook); !result.success {
		return result.Result, err
	}

	if result, err := r.reconcileNotebook(ctx, &notebook); !result.success {
		return result.Result, err
	}

	return ctrl.Result{}, nil
}

//+kubebuilder:rbac:groups=substratus.ai,resources=notebooks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=notebooks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=notebooks/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// SetupWithManager sets up the controller with the Manager.
func (r *NotebookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Notebook{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Pod{}).
		Watches(&apiv1.Model{}, handler.EnqueueRequestsFromMapFunc(handler.MapFunc(r.findNotebooksForModel))).
		Watches(&apiv1.Dataset{}, handler.EnqueueRequestsFromMapFunc(handler.MapFunc(r.findNotebooksForDataset))).
		Complete(r)
}

func (r *NotebookReconciler) findNotebooksForModel(ctx context.Context, obj client.Object) []reconcile.Request {
	model := obj.(*apiv1.Model)

	var notebooks apiv1.NotebookList
	if err := r.List(ctx, &notebooks,
		client.MatchingFields{notebookModelIndex: model.Name},
		client.InNamespace(obj.GetNamespace()),
	); err != nil {
		log.Log.Error(err, "unable to list notebooks for base model")
		return nil
	}

	reqs := []reconcile.Request{}
	for _, nb := range notebooks.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      nb.Name,
				Namespace: nb.Namespace,
			},
		})
	}
	return reqs
}

func (r *NotebookReconciler) findNotebooksForDataset(ctx context.Context, obj client.Object) []reconcile.Request {
	dataset := obj.(*apiv1.Dataset)

	var notebooks apiv1.NotebookList
	if err := r.List(ctx, &notebooks,
		client.MatchingFields{notebookDatasetIndex: dataset.Name},
		client.InNamespace(obj.GetNamespace()),
	); err != nil {
		log.Log.Error(err, "unable to list notebooks for dataset")
		return nil
	}

	reqs := []reconcile.Request{}
	for _, nb := range notebooks.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      nb.Name,
				Namespace: nb.Namespace,
			},
		})
	}
	return reqs
}

func (r *NotebookReconciler) reconcileNotebook(ctx context.Context, notebook *apiv1.Notebook) (result, error) {
	log := log.FromContext(ctx)

	if notebook.IsSuspended() {
		notebook.Status.Ready = false
		meta.SetStatusCondition(&notebook.Status.Conditions, metav1.Condition{
			Type:               apiv1.ConditionDeployed,
			Status:             metav1.ConditionFalse,
			Reason:             apiv1.ReasonSuspended,
			ObservedGeneration: notebook.Generation,
		})
		if err := r.Status().Update(ctx, notebook); err != nil {
			return result{}, fmt.Errorf("updating notebook status: %w", err)
		}

		var pod corev1.Pod
		pod.SetName(nbPodName(notebook))
		pod.SetNamespace(notebook.Namespace)
		if err := r.Delete(ctx, &pod); err != nil {
			if !apierrors.IsNotFound(err) {
				return result{}, err
			}
		}
		return result{}, nil
	}

	// ServiceAccount for the model Job.
	// Within the context of GCP, this ServiceAccount will need IAM permissions
	// to read the GCS buckets containing the training data and model artifacts.
	if result, err := reconcileCloudServiceAccount(ctx, r.Cloud, r.Client, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      notebookServiceAccountName,
			Namespace: notebook.Namespace,
		},
	}); !result.success {
		return result, err
	}

	var model *apiv1.Model
	if notebook.Spec.Model != nil {
		model = &apiv1.Model{}
		if err := r.Get(ctx, client.ObjectKey{Name: notebook.Spec.Model.Name, Namespace: notebook.Namespace}, model); err != nil {
			if apierrors.IsNotFound(err) {
				log.Error(err, "Model not found")
				// Update this Model's status.
				notebook.Status.Ready = false
				meta.SetStatusCondition(&notebook.Status.Conditions, metav1.Condition{
					Type:               apiv1.ConditionDeployed,
					Status:             metav1.ConditionFalse,
					Reason:             apiv1.ReasonModelNotFound,
					ObservedGeneration: notebook.Generation,
				})
				if err := r.Status().Update(ctx, notebook); err != nil {
					return result{}, fmt.Errorf("failed to update notebook status: %w", err)
				}

				// TODO: Implement watch on source Model.
				return result{}, nil
			}
			return result{}, fmt.Errorf("getting model: %w", err)
		}

		if !model.Status.Ready {
			log.Info("Model not ready", "model", model.Name)

			notebook.Status.Ready = false
			meta.SetStatusCondition(&notebook.Status.Conditions, metav1.Condition{
				Type:               apiv1.ConditionDeployed,
				Status:             metav1.ConditionFalse,
				Reason:             apiv1.ReasonModelNotReady,
				ObservedGeneration: notebook.Generation,
			})
			if err := r.Status().Update(ctx, notebook); err != nil {
				return result{}, fmt.Errorf("failed to update notebook status: %w", err)
			}

			return result{}, nil
		}

	}

	var dataset *apiv1.Dataset
	if notebook.Spec.Dataset != nil {
		dataset = &apiv1.Dataset{}
		if err := r.Get(ctx, client.ObjectKey{Name: notebook.Spec.Dataset.Name, Namespace: notebook.Namespace}, dataset); err != nil {
			if apierrors.IsNotFound(err) {
				log.Error(err, "Dataset not found")
				notebook.Status.Ready = false
				meta.SetStatusCondition(&notebook.Status.Conditions, metav1.Condition{
					Type:               apiv1.ConditionDeployed,
					Status:             metav1.ConditionFalse,
					Reason:             apiv1.ReasonDatasetNotFound,
					ObservedGeneration: notebook.Generation,
				})
				if err := r.Status().Update(ctx, notebook); err != nil {
					return result{}, fmt.Errorf("failed to update notebook status: %w", err)
				}

				// TODO: Implement watch on source Model.
				return result{}, nil
			}
			return result{}, fmt.Errorf("getting dataset: %w", err)
		}

		if !dataset.Status.Ready {
			log.Info("Dataset not ready", "dataset", dataset.Name)
			notebook.Status.Ready = false
			meta.SetStatusCondition(&notebook.Status.Conditions, metav1.Condition{
				Type:               apiv1.ConditionDeployed,
				Status:             metav1.ConditionFalse,
				Reason:             apiv1.ReasonDatasetNotReady,
				ObservedGeneration: notebook.Generation,
			})
			if err := r.Status().Update(ctx, notebook); err != nil {
				return result{}, fmt.Errorf("failed to update notebook status: %w", err)
			}

			return result{}, nil
		}

	}

	//pvc, err := r.notebookPVC(&notebook)
	//if err != nil {
	//	return result{}, fmt.Errorf("failed to construct pvc: %w", err)
	//}

	//if err := r.Patch(ctx, pvc, client.Apply, client.FieldOwner("notebook-controller")); err != nil {
	//	return result{}, fmt.Errorf("failed to apply pvc: %w", err)
	//}

	pod, err := r.notebookPod(notebook, model, dataset)
	if err != nil {
		return result{}, fmt.Errorf("failed to construct pod: %w", err)
	}
	if err := r.Patch(ctx, pod, client.Apply, client.FieldOwner("notebook-controller"), client.ForceOwnership); err != nil {
		// If attempt to change an immutable field will result in a Invalid
		// error with some text like:
		//
		// failed to apply pod: Pod \"example-notebook\" is invalid: spec: Forbidden: pod updates may not change fields other than
		// ...
		if apierrors.IsInvalid(err) {
			log.Error(err, "failed to apply pod, deleting pod")
			if err := r.Delete(ctx, pod); err != nil {
				return result{}, fmt.Errorf("failed to delete pod: %w", err)
			}
			// Allow requeue via Pod event.
			return result{}, nil
		}
		return result{}, fmt.Errorf("failed to apply pod: %w", err)
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
		if !apierrors.IsNotFound(err) {
			return result{}, fmt.Errorf("failed to get pod: %w", err)
		}
	}

	if isPodReady(pod) {
		notebook.Status.Ready = true
		meta.SetStatusCondition(&notebook.Status.Conditions, metav1.Condition{
			Type:               apiv1.ConditionDeployed,
			Status:             metav1.ConditionTrue,
			Reason:             apiv1.ReasonPodReady,
			ObservedGeneration: notebook.Generation,
		})
	} else {
		notebook.Status.Ready = false
		meta.SetStatusCondition(&notebook.Status.Conditions, metav1.Condition{
			Type:               apiv1.ConditionDeployed,
			Status:             metav1.ConditionFalse,
			Reason:             apiv1.ReasonPodNotReady,
			ObservedGeneration: notebook.Generation,
		})
	}
	if err := r.Status().Update(ctx, notebook); err != nil {
		return result{}, fmt.Errorf("updating notebook status: %w", err)
	}

	return result{success: true}, nil
}

func nbPodName(nb *apiv1.Notebook) string {
	return nb.Name + "-notebook"
}

// notebookPod constructs a Pod for the given Notebook.
func (r *NotebookReconciler) notebookPod(notebook *apiv1.Notebook, model *apiv1.Model, dataset *apiv1.Dataset) (*corev1.Pod, error) {
	const containerName = "notebook"

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nbPodName(notebook),
			Namespace: notebook.Namespace,
			Annotations: map[string]string{
				"kubectl.kubernetes.io/default-container": containerName,
			},
		},
		Spec: corev1.PodSpec{
			//SecurityContext: &corev1.PodSecurityContext{
			//	RunAsUser:  int64Ptr(1000),
			//	RunAsGroup: int64Ptr(100),
			//	FSGroup:    int64Ptr(100),
			//},
			ServiceAccountName: notebookServiceAccountName,
			Containers: []corev1.Container{
				{
					Name:  containerName,
					Image: notebook.GetImage(),
					// NOTE: tini should be installed as the ENTRYPOINT the image and will be used
					// to execute this script.
					Command: notebook.Spec.Command,
					//WorkingDir: "/home/jovyan",
					Ports: []corev1.ContainerPort{
						{
							Name:          "notebook",
							ContainerPort: 8888,
						},
					},
					Env: paramsToEnv(notebook.Spec.Params),
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

	if err := mountParamsConfigMap(&pod.Spec, notebook, containerName); err != nil {
		return nil, fmt.Errorf("mounting params configmap: %w", err)
	}

	if dataset != nil {
		if err := r.Cloud.MountBucket(&pod.ObjectMeta, &pod.Spec, dataset, cloud.MountBucketConfig{
			Name: "dataset",
			Mounts: []cloud.BucketMount{
				{BucketSubdir: "data", ContentSubdir: "data"},
				{BucketSubdir: "logs", ContentSubdir: "data-logs"},
			},
			Container: containerName,
			ReadOnly:  true,
		}); err != nil {
			return nil, fmt.Errorf("mounting dataset: %w", err)
		}
	}

	if model != nil {
		if err := r.Cloud.MountBucket(&pod.ObjectMeta, &pod.Spec, model, cloud.MountBucketConfig{
			Name: "basemodel",
			Mounts: []cloud.BucketMount{
				{BucketSubdir: "model", ContentSubdir: "saved-model"},
				{BucketSubdir: "logs", ContentSubdir: "saved-model-logs"},
			},
			Container: containerName,
			ReadOnly:  true,
		}); err != nil {
			return nil, fmt.Errorf("mounting model: %w", err)
		}
	}

	if err := ctrl.SetControllerReference(notebook, pod, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	if err := resources.Apply(&pod.ObjectMeta, &pod.Spec, containerName,
		r.Cloud.Name(), notebook.Spec.Resources); err != nil {
		return nil, fmt.Errorf("applying resources: %w", err)
	}

	return pod, nil
}

//func notebookPVCName(nb *apiv1.Notebook) string {
//	return nb.Name + "-notebook"
//}

//func (r *NotebookReconciler) notebookPVC(nb *apiv1.Notebook) (*corev1.PersistentVolumeClaim, error) {
//	pvc := &corev1.PersistentVolumeClaim{
//		TypeMeta: metav1.TypeMeta{
//			APIVersion: "v1",
//			Kind:       "PersistentVolumeClaim",
//		},
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      notebookPVCName(nb),
//			Namespace: nb.Namespace,
//		},
//		Spec: corev1.PersistentVolumeClaimSpec{
//			AccessModes: []corev1.PersistentVolumeAccessMode{
//				"ReadWriteOnce",
//			},
//			Resources: corev1.ResourceRequirements{
//				Requests: corev1.ResourceList{
//					"storage": resource.MustParse("10Gi"),
//				},
//			},
//		},
//	}
//
//	if err := ctrl.SetControllerReference(nb, pvc, r.Scheme); err != nil {
//		return nil, fmt.Errorf("failed to set controller reference: %w", err)
//	}
//
//	return pvc, nil
//}
