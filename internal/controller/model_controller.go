package controller

import (
	"context"
	"errors"
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

	*ContainerReconciler

	CloudContext *cloud.Context
}

type ModelReconcilerConfig struct {
	ImageRegistry string
}

//+kubebuilder:rbac:groups=substratus.ai,resources=models,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=models/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=models/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch

func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling Model")
	defer log.Info("Done reconciling Model")

	var model apiv1.Model
	if err := r.Get(ctx, req.NamespacedName, &model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if result, err := r.ReconcileContainer(ctx, &model); !result.success {
		return result.Result, err
	}

	if model.Spec.Trainer != nil {
		if result, err := r.reconcileTrainer(ctx, &model); !result.success {
			return result.Result, err
		}
	} else if model.Spec.Loader != nil {
		if result, err := r.reconcileLoader(ctx, &model); !result.success {
			return result.Result, err
		}
	} else {
		log.Error(errors.New("no trainer or loader specified"), "this point should never have been reached if the model is valid")
		// No use in retrying (returning an error).
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ModelReconciler) reconcileTrainer(ctx context.Context, model *apiv1.Model) (result, error) {
	log := log.FromContext(ctx)

	if model.Status.URL != "" {
		return result{success: true}, nil
	}

	// ServiceAccount for the trainer job.
	// Within the context of GCP, this ServiceAccount will need IAM permissions
	// to read the GCS bucket containing the training data and read and write from
	// the bucket that contains base model artifacts.
	if result, err := reconcileCloudServiceAccount(ctx, r.CloudContext, r.Client, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      modelTrainerServiceAccountName,
			Namespace: model.Namespace,
		},
	}); !result.success {
		return result, err
	}

	var baseModel *apiv1.Model
	if model.Spec.Trainer.BaseModel != nil {
		sourceModel := &apiv1.Model{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Trainer.BaseModel.Name}, sourceModel); err != nil {
			if apierrors.IsNotFound(err) {
				// Update this Model's status.
				model.Status.Ready = false
				meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
					Type:               apiv1.ConditionTrained,
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
				Type:               apiv1.ConditionTrained,
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

	var dataset apiv1.Dataset
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Trainer.Dataset.Name}, &dataset); err != nil {
		if apierrors.IsNotFound(err) {
			// Update this Model's status.
			model.Status.Ready = false
			meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
				Type:               apiv1.ConditionTrained,
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
			Type:               apiv1.ConditionTrained,
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

	trainingJob, err := r.trainerJob(ctx, model, baseModel, &dataset)
	if err != nil {
		log.Error(err, "unable to construct training Job")
		// No use in retrying...
		return result{}, nil
	}

	model.Status.Ready = false
	meta.SetStatusCondition(model.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionTrained,
		Status:             metav1.ConditionFalse,
		Reason:             apiv1.ReasonJobNotComplete,
		ObservedGeneration: model.Generation,
		Message:            fmt.Sprintf("Waiting for training Job to complete"),
	})
	if err := r.Status().Update(ctx, model); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	if result, err := reconcileJob(ctx, r.Client, model, trainingJob, apiv1.ConditionTrained); !result.success {
		return result, err
	}

	model.Status.Ready = true
	model.Status.URL = r.modelStatusURL(model)
	meta.SetStatusCondition(model.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionTrained,
		Status:             metav1.ConditionTrue,
		Reason:             apiv1.ReasonJobComplete,
		ObservedGeneration: model.Generation,
	})
	if err := r.Status().Update(ctx, model); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	return result{success: true}, nil
}

func (r *ModelReconciler) reconcileLoader(ctx context.Context, model *apiv1.Model) (result, error) {
	log := log.FromContext(ctx)

	if model.Status.URL != "" {
		return result{success: true}, nil
	}

	// ServiceAccount for the trainer job.
	// Within the context of GCP, this ServiceAccount will need IAM write permissions
	// to the GCS bucket with models stored.
	if result, err := reconcileCloudServiceAccount(ctx, r.CloudContext, r.Client, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      modelLoaderServiceAccountName,
			Namespace: model.Namespace,
		},
	}); !result.success {
		return result, err
	}

	loaderJob, err := r.loaderJob(ctx, model)
	if err != nil {
		log.Error(err, "unable to construct loader Job")
		// No use in retrying...
		return result{}, nil
	}

	model.Status.Ready = false
	meta.SetStatusCondition(model.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionLoaded,
		Status:             metav1.ConditionFalse,
		Reason:             apiv1.ReasonJobNotComplete,
		ObservedGeneration: model.Generation,
		Message:            fmt.Sprintf("Waiting for loader Job to complete"),
	})
	if err := r.Status().Update(ctx, model); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	if result, err := reconcileJob(ctx, r.Client, model, loaderJob, apiv1.ConditionTrained); !result.success {
		return result, err
	}

	model.Status.Ready = true
	model.Status.URL = r.modelStatusURL(model)
	meta.SetStatusCondition(model.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionLoaded,
		Status:             metav1.ConditionTrue,
		Reason:             apiv1.ReasonJobComplete,
		ObservedGeneration: model.Generation,
	})
	if err := r.Status().Update(ctx, model); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	return result{success: true}, nil
}

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
		client.MatchingFields{"spec.trainer.baseModel.name": model.Name},
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
		client.MatchingFields{"spec.trainer.dataset.name": dataset.Name},
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

func (r *ModelReconciler) loaderJob(ctx context.Context, model *apiv1.Model) (*batchv1.Job, error) {
	var job *batchv1.Job

	annotations := make(map[string]string)
	volumes := []corev1.Volume{}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "model",
			MountPath: "/model/logs",
			SubPath:   string(model.UID) + "/logs",
		},
		{
			Name:      "model",
			MountPath: "/model/saved",
			SubPath:   string(model.UID) + "/trained",
		},
	}

	switch r.CloudContext.Name {
	case cloud.GCP:
		// GKE will injects GCS Fuse sidecar based on this annotation.
		annotations["gke-gcsfuse/volumes"] = "true"

		volumes = append(volumes, corev1.Volume{
			Name: "model",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver: "gcsfuse.csi.storage.gke.io",
					VolumeAttributes: map[string]string{
						"bucketName":   r.CloudContext.GCP.ProjectID + "-substratus-models",
						"mountOptions": "implicit-dirs,uid=0,gid=3003",
					},
				},
			},
		})
	}

	env := paramsToEnv(model.Spec.Loader.Params)

	const loaderContainerName = "loader"
	annotations["kubectl.kubernetes.io/default-container"] = loaderContainerName
	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: model.Name + "-model-loader",
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
					ServiceAccountName: modelLoaderServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  loaderContainerName,
							Image: model.Spec.Container.Image,
							// NOTE: tini should be installed as the ENTRYPOINT of the image and will be used
							// to execute this script.
							Args:         []string{"load.sh"},
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

	if err := resources.Apply(&job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, loaderContainerName,
		r.CloudContext.Name, model.Spec.Resources); err != nil {
		return nil, fmt.Errorf("applying resources: %w", err)
	}

	return job, nil
}

// trainerJob returns a Job that will train the Model. While this function
// has a lot in common with loaderJob, the two functions were not factored together
// because the differences will grow over time (for example: distributed training).
func (r *ModelReconciler) trainerJob(ctx context.Context, model, baseModel *apiv1.Model, dataset *apiv1.Dataset) (*batchv1.Job, error) {
	var job *batchv1.Job

	annotations := make(map[string]string)
	volumes := []corev1.Volume{
		{
			Name: "model",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver: "gcsfuse.csi.storage.gke.io",
					VolumeAttributes: map[string]string{
						"bucketName":   r.CloudContext.GCP.ProjectID + "-substratus-models",
						"mountOptions": "implicit-dirs,uid=0,gid=3003",
					},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "model",
			MountPath: "/model/logs",
			SubPath:   string(model.UID) + "/logs",
		},
		{
			Name:      "model",
			MountPath: "/model/trained",
			SubPath:   string(model.UID) + "/trained",
		},
	}

	switch r.CloudContext.Name {
	case cloud.GCP:
		// GKE will injects GCS Fuse sidecar based on this annotation.
		annotations["gke-gcsfuse/volumes"] = "true"

		if err := mountDataset(volumes, volumeMounts, dataset); err != nil {
			return nil, fmt.Errorf("appending dataset volume: %w", err)
		}

		if baseModel != nil {
			if err := mountSavedModel(volumes, volumeMounts, baseModel); err != nil {
				return nil, fmt.Errorf("appending base model volume: %w", err)
			}
		}
	}

	env := append(paramsToEnv(model.Spec.Trainer.Params),
		corev1.EnvVar{
			Name:  "DATA_PATH",
			Value: "/data/" + dataset.Spec.Filename,
		},
		corev1.EnvVar{
			Name:  "DATA_LIMIT",
			Value: fmt.Sprintf("%v", model.Spec.Trainer.DataLimit),
		},
		corev1.EnvVar{
			Name:  "BATCH_SIZE",
			Value: fmt.Sprintf("%v", model.Spec.Trainer.BatchSize),
		},
		corev1.EnvVar{
			Name:  "EPOCHS",
			Value: fmt.Sprintf("%v", model.Spec.Trainer.Epochs),
		},
	)

	const trainerContainerName = "trainer"
	annotations["kubectl.kubernetes.io/default-container"] = trainerContainerName
	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: model.Name + "-model-trainer",
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
					ServiceAccountName: modelTrainerServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  trainerContainerName,
							Image: model.Spec.Container.Image,
							// NOTE: tini should be installed as the ENTRYPOINT the image and will be used
							// to execute this script.
							Args:         []string{"train.sh"},
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

	if err := resources.Apply(&job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, trainerContainerName,
		r.CloudContext.Name, model.Spec.Resources); err != nil {
		return nil, fmt.Errorf("applying resources: %w", err)
	}

	return job, nil
}

func (r *ModelReconciler) modelStatusURL(model *apiv1.Model) string {
	return "gs://" + r.CloudContext.GCP.ProjectID + "-substratus-models" +
		"/" + string(model.UID) + "/trained/"
}
