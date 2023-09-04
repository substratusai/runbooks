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
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/resources"
	"github.com/substratusai/substratus/internal/sci"
)

// ModelReconciler reconciles a Model object.
type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	*ParamsReconciler

	Cloud cloud.Cloud
	SCI   sci.ControllerClient
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

	if model.GetImage() == "" {
		// Image must be building.
		return ctrl.Result{}, nil
	}

	if result, err := r.ReconcileParamsConfigMap(ctx, &model); !result.success {
		return result.Result, err
	}

	if result, err := r.reconcileModel(ctx, &model); !result.success {
		return result.Result, err
	}

	return ctrl.Result{}, nil
}

func (r *ModelReconciler) reconcileModel(ctx context.Context, model *apiv1.Model) (result, error) {
	log := log.FromContext(ctx)

	if model.Status.Ready {
		return result{success: true}, nil
	}

	model.Status.Artifacts.URL = r.Cloud.ObjectArtifactURL(model).String()

	// ServiceAccount for the model Job.
	// Within the context of GCP, this ServiceAccount will need IAM permissions
	// to read the GCS bucket containing the training data and read and write from
	// the bucket that contains base model artifacts.
	if result, err := reconcileServiceAccount(ctx, r.Cloud, r.SCI, r.Client, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      modellerServiceAccountName,
			Namespace: model.Namespace,
		},
	}); !result.success {
		return result, err
	}

	var baseModel *apiv1.Model
	if model.Spec.Model != nil {
		baseModel = &apiv1.Model{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Model.Name}, baseModel); err != nil {
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
		if !baseModel.Status.Ready {
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
	if model.Spec.Dataset != nil {
		dataset = &apiv1.Dataset{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Dataset.Name}, dataset); err != nil {
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
		Message:            "Waiting for modeller Job to complete",
	})
	if err := r.Status().Update(ctx, model); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	if result, err := reconcileJob(ctx, r.Client, modellerJob, apiv1.ConditionModelled); !result.success {
		return result, err
	}

	model.Status.Ready = true
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
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// SetupWithManager sets up the controller with the Manager.
func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Model{}).
		Watches(&apiv1.Model{}, handler.EnqueueRequestsFromMapFunc(handler.MapFunc(r.findModelsForBaseModel))).
		Watches(&apiv1.Dataset{}, handler.EnqueueRequestsFromMapFunc(handler.MapFunc(r.findModelsForDataset))).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *ModelReconciler) findModelsForBaseModel(ctx context.Context, obj client.Object) []reconcile.Request {
	model := obj.(*apiv1.Model)

	var models apiv1.ModelList
	if err := r.List(ctx, &models,
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

func (r *ModelReconciler) findModelsForDataset(ctx context.Context, obj client.Object) []reconcile.Request {
	dataset := obj.(*apiv1.Dataset)

	var models apiv1.ModelList
	if err := r.List(ctx, &models,
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

	envVars, err := resolveEnv(model.Spec.Env)
	if err != nil {
		return nil, fmt.Errorf("resolving env: %w", err)
	}

	const containerName = "model"
	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: model.Name + "-modeller",
			// Cross-Namespace owners not allowed, must be same as model:
			Namespace: model.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(1)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"kubectl.kubernetes.io/default-container": containerName,
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: ptr.To(int64(3003)),
					},
					ServiceAccountName: modellerServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:    containerName,
							Image:   model.GetImage(),
							Command: model.Spec.Command,
							Env:     envVars,
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	if err := mountParamsConfigMap(&job.Spec.Template.Spec, model, containerName); err != nil {
		return nil, fmt.Errorf("mounting params configmap: %w", err)
	}

	if err := r.Cloud.MountBucket(&job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, model, cloud.MountBucketConfig{
		Name: "output",
		Mounts: []cloud.BucketMount{
			{BucketSubdir: "artifacts", ContentSubdir: "output"},
		},
		Container: containerName,
		ReadOnly:  false,
	}); err != nil {
		return nil, fmt.Errorf("mounting model: %w", err)
	}

	if dataset != nil {
		if err := r.Cloud.MountBucket(&job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, dataset, cloud.MountBucketConfig{
			Name: "dataset",
			Mounts: []cloud.BucketMount{
				{BucketSubdir: "artifacts", ContentSubdir: "data"},
			},
			Container: containerName,
			ReadOnly:  true,
		}); err != nil {
			return nil, fmt.Errorf("mounting dataset: %w", err)
		}
	}

	if baseModel != nil {
		if err := r.Cloud.MountBucket(&job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, baseModel, cloud.MountBucketConfig{
			Name: "model",
			Mounts: []cloud.BucketMount{
				{BucketSubdir: "artifacts", ContentSubdir: "model"},
			},
			Container: containerName,
			ReadOnly:  true,
		}); err != nil {
			return nil, fmt.Errorf("mounting base model: %w", err)
		}
	}

	if err := controllerutil.SetControllerReference(model, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	if err := resources.Apply(&job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, containerName,
		r.Cloud.Name(), model.Spec.Resources); err != nil {
		return nil, fmt.Errorf("applying resources: %w", err)
	}

	return job, nil
}
