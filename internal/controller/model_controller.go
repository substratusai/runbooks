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
	ptr "k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/resources"
)

// ModelReconciler reconciles a Model object.
type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	*ContainerImageReconciler

	CloudContext *cloud.Context
}

type ModelReconcilerConfig struct {
	ImageRegistry string
}

func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling Model")
	defer log.Info("Done reconciling Model")

	var model apiv1.Model
	if err := r.Get(ctx, req.NamespacedName, &model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if result, err := r.ReconcileContainerImage(ctx, &model); !result.success {
		return result.Result, err
	}

	if result, err := r.reconcileModel(ctx, &model); !result.success {
		return result.Result, err
	}

	return ctrl.Result{}, nil
}

func (r *ModelReconciler) reconcileModel(ctx context.Context, model *apiv1.Model) (result, error) {
	log := log.FromContext(ctx)

	if model.Status.URL != "" {
		return result{success: true}, nil
	}

	// ServiceAccount for the model Job.
	// Within the context of GCP, this ServiceAccount will need IAM permissions
	// to read the GCS bucket containing the training data and read and write from
	// the bucket that contains base model artifacts.
	if result, err := reconcileCloudServiceAccount(ctx, r.CloudContext, r.Client, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      modellerServiceAccountName,
			Namespace: model.Namespace,
		},
	}); !result.success {
		return result, err
	}

	var baseModel *apiv1.Model
	if model.Spec.BaseModel != nil {
		sourceModel := &apiv1.Model{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.BaseModel.Name}, sourceModel); err != nil {
			if apierrors.IsNotFound(err) {
				// Update this Model's status.
				model.Status.Ready = false
				meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
					Type:               apiv1.ConditionModelled,
					Status:             metav1.ConditionFalse,
					Reason:             apiv1.ReasonBaseModelNotFound,
					ObservedGeneration: model.Generation,
				})
				if err := r.Status().Update(ctx, model); err != nil {
					return result{}, fmt.Errorf("failed to update model status: %w", err)
				}

				// Allow for watch to requeue.
				return result{}, nil
			}

			return result{}, fmt.Errorf("getting source model: %w", err)
		}
		if !sourceModel.Status.Ready {
			// Update this Model's status.
			model.Status.Ready = false
			meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
				Type:               apiv1.ConditionModelled,
				Status:             metav1.ConditionFalse,
				Reason:             apiv1.ReasonBaseModelNotReady,
				ObservedGeneration: model.Generation,
			})
			if err := r.Status().Update(ctx, model); err != nil {
				return result{}, fmt.Errorf("failed to update model status: %w", err)
			}

			// Allow for watch to requeue.
			return result{}, nil
		}
	}

	var dataset *apiv1.Dataset
	if model.Spec.TrainingDataset != nil {
		dataset = &apiv1.Dataset{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.TrainingDataset.Name}, dataset); err != nil {
			if apierrors.IsNotFound(err) {
				// Update this Model's status.
				model.Status.Ready = false
				meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
					Type:               apiv1.ConditionModelled,
					Status:             metav1.ConditionFalse,
					Reason:             apiv1.ReasonDatasetNotFound,
					ObservedGeneration: model.Generation,
				})
				if err := r.Status().Update(ctx, model); err != nil {
					return result{}, fmt.Errorf("failed to update model status: %w", err)
				}

				// Allow for watch to requeue.
				return result{}, nil
			}

			return result{}, fmt.Errorf("getting source model: %w", err)
		}
		if !dataset.Status.Ready {
			// Update this Model's status.
			model.Status.Ready = false
			meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
				Type:               apiv1.ConditionModelled,
				Status:             metav1.ConditionFalse,
				Reason:             apiv1.ReasonDatasetNotReady,
				ObservedGeneration: model.Generation,
			})
			if err := r.Status().Update(ctx, model); err != nil {
				return result{}, fmt.Errorf("failed to update model status: %w", err)
			}

			// Allow for watch to requeue.
			return result{}, nil
		}
	}

	modellerJob, err := r.modellerJob(ctx, model, baseModel, dataset)
	if err != nil {
		log.Error(err, "unable to construct modeller Job")
		// No use in retrying...
		return result{}, nil
	}

	model.Status.Ready = false
	meta.SetStatusCondition(model.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionModelled,
		Status:             metav1.ConditionFalse,
		Reason:             apiv1.ReasonJobNotComplete,
		ObservedGeneration: model.Generation,
		Message:            fmt.Sprintf("Waiting for modeller Job to complete"),
	})
	if err := r.Status().Update(ctx, model); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	if result, err := reconcileJob(ctx, r.Client, model, modellerJob, apiv1.ConditionModelled); !result.success {
		return result, err
	}

	model.Status.Ready = true
	model.Status.URL = r.modelURL(model)
	meta.SetStatusCondition(model.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionModelled,
		Status:             metav1.ConditionTrue,
		Reason:             apiv1.ReasonJobComplete,
		ObservedGeneration: model.Generation,
	})
	if err := r.Status().Update(ctx, model); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	return result{success: true}, nil
}

//+kubebuilder:rbac:groups=substratus.ai,resources=models,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=models/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=models/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch

// SetupWithManager sets up the controller with the Manager.
func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Model{}).
		Watches(&source.Kind{Type: &apiv1.Model{}}, handler.EnqueueRequestsFromMapFunc(handler.MapFunc(r.findModelsForBaseModel))).
		Watches(&source.Kind{Type: &apiv1.Dataset{}}, handler.EnqueueRequestsFromMapFunc(handler.MapFunc(r.findModelsForDataset))).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *ModelReconciler) findModelsForBaseModel(obj client.Object) []reconcile.Request {
	model := obj.(*apiv1.Model)

	var models apiv1.ModelList
	if err := r.List(context.Background(), &models,
		client.MatchingFields{modelBaseModelIndex: model.Name},
		client.InNamespace(obj.GetNamespace()),
	); err != nil {
		log.Log.Error(err, "unable to list models for base model")
		return nil
	}

	reqs := []reconcile.Request{}
	for _, mdl := range models.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      mdl.Name,
				Namespace: mdl.Namespace,
			},
		})
	}
	return reqs
}

func (r *ModelReconciler) findModelsForDataset(obj client.Object) []reconcile.Request {
	dataset := obj.(*apiv1.Dataset)

	var models apiv1.ModelList
	if err := r.List(context.Background(), &models,
		client.MatchingFields{modelTrainingDatasetIndex: dataset.Name},
		client.InNamespace(obj.GetNamespace()),
	); err != nil {
		log.Log.Error(err, "unable to list models for dataset")
		return nil
	}

	reqs := []reconcile.Request{}
	for _, mdl := range models.Items {
		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      mdl.Name,
				Namespace: mdl.Namespace,
			},
		})
	}
	return reqs
}

// modellerJob returns a Job that will train or load the Model.
func (r *ModelReconciler) modellerJob(ctx context.Context, model, baseModel *apiv1.Model, dataset *apiv1.Dataset) (*batchv1.Job, error) {
	var job *batchv1.Job

	annotations := make(map[string]string)
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	if err := mountModel(volumes, volumeMounts, r.modelURL(model), "", false); err != nil {
		return nil, fmt.Errorf("appending current model volume: %w", err)
	}

	switch r.CloudContext.Name {
	case cloud.GCP:
		// GKE will injects GCS Fuse sidecar based on this annotation.
		annotations["gke-gcsfuse/volumes"] = "true"

		if dataset != nil {
			if err := mountDataset(volumes, volumeMounts, dataset.Status.URL, true); err != nil {
				return nil, fmt.Errorf("appending dataset volume: %w", err)
			}
		}

		if baseModel != nil {
			if err := mountModel(volumes, volumeMounts, baseModel.Status.URL, "base-", true); err != nil {
				return nil, fmt.Errorf("appending base model volume: %w", err)
			}
		}
	}

	env := paramsToEnv(model.Spec.Params)
	if dataset != nil {
		env = append(env,
			corev1.EnvVar{
				Name:  "DATA_PATH",
				Value: "/data/" + dataset.Spec.Filename,
			})
	}

	const containerName = "model"
	annotations["kubectl.kubernetes.io/default-container"] = containerName
	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: model.Name + "-modeller",
			// Cross-Namespace owners not allowed, must be same as model:
			Namespace: model.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.Int32(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  ptr.Int64(0),
						RunAsGroup: ptr.Int64(0),
						FSGroup:    ptr.Int64(3003),
					},
					ServiceAccountName: modellerServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  containerName,
							Image: model.Spec.Image.Name,
							// NOTE: tini should be installed as the ENTRYPOINT the image and will be used
							// to execute this script.
							Args:         []string{"model.sh"},
							Env:          env,
							VolumeMounts: volumeMounts,
						},
					},
					Volumes:       volumes,
					RestartPolicy: "Never",
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(model, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	if err := resources.Apply(&job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, containerName,
		r.CloudContext.Name, model.Spec.Resources); err != nil {
		return nil, fmt.Errorf("applying resources: %w", err)
	}

	return job, nil
}

func (r *ModelReconciler) modelURL(model *apiv1.Model) string {
	return "gs://" + r.CloudContext.GCP.ProjectID + "-substratus-models" +
		"/" + string(model.UID) + "/"
}
