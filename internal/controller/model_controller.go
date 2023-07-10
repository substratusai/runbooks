package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
)

const (
	//ConditionReady = "Ready"

	//ReasonTraining       = "Training"
	//ReasonBuilding       = "Building"
	//ReasonBuiltAndPushed = "BuiltAndPushed"

	//ReasonSourceModelNotFound = "SourceModelNotFound"
	//ReasonSourceModelNotReady = "SourceModelNotReady"
	//TrainingDatasetNotReady   = "TrainingDatasetNotReady"

	modelTrainerServiceAccountName = "model-trainer"
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
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

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

	result, err := reconcileReadiness(ctx, r.Client, &model, map[string]bool{
		"TODO": true,
	})
	if result.success {
		log.Info("Model is ready")
	}

	return result.Result, err
}

func (r *ModelReconciler) reconcileTrainer(ctx context.Context, model *apiv1.Model) (result, error) {
	log := log.FromContext(ctx)

	trainerSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      modelTrainerServiceAccountName,
			Namespace: model.Namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, trainerSA, func() error {
		return r.authNServiceAccount(trainerSA)
	}); err != nil {
		return result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	var baseModel *apiv1.Model
	if model.Spec.Trainer.BaseModel != nil {
		sourceModel := &apiv1.Model{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Trainer.BaseModel.Name}, sourceModel); err != nil {
			if apierrors.IsNotFound(err) {
				// Update this Model's status.
				meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
					Type:               apiv1.ConditionDependenciesReady,
					Status:             metav1.ConditionFalse,
					Reason:             apiv1.ReasonBaseModelNotFound,
					ObservedGeneration: model.Generation,
				})
				if err := r.Status().Update(ctx, model); err != nil {
					return result{}, fmt.Errorf("failed to update model status: %w", err)
				}

				// TODO: Implement watch on source Model.
				return result{Result: ctrl.Result{RequeueAfter: 3 * time.Second}}, nil
			}

			return result{}, fmt.Errorf("getting source model: %w", err)
		}
		if !sourceModel.Status.Ready {
			// Update this Model's status.
			meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
				Type:               apiv1.ConditionDependenciesReady,
				Status:             metav1.ConditionFalse,
				Reason:             apiv1.ReasonBaseModelNotReady,
				ObservedGeneration: model.Generation,
			})
			if err := r.Status().Update(ctx, model); err != nil {
				return result{}, fmt.Errorf("failed to update model status: %w", err)
			}

			// TODO: Instead of RequeueAfter, add a watch mapper.
			return result{Result: ctrl.Result{RequeueAfter: time.Minute}}, nil
		}
	}

	var dataset apiv1.Dataset
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Trainer.Dataset.Name}, &dataset); err != nil {
		if apierrors.IsNotFound(err) {
			// Update this Model's status.
			meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
				Type:               apiv1.ConditionDependenciesReady,
				Status:             metav1.ConditionFalse,
				Reason:             apiv1.ReasonDatasetNotFound,
				ObservedGeneration: model.Generation,
			})
			if err := r.Status().Update(ctx, model); err != nil {
				return result{}, fmt.Errorf("failed to update model status: %w", err)
			}

			// TODO: Implement watch on source Model.
			return result{Result: ctrl.Result{RequeueAfter: 3 * time.Second}}, nil
		}

		return result{}, fmt.Errorf("getting source model: %w", err)
	}
	if !dataset.Status.Ready {
		// Update this Model's status.
		meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
			Type:               apiv1.ConditionDependenciesReady,
			Status:             metav1.ConditionFalse,
			Reason:             apiv1.ReasonDatasetNotReady,
			ObservedGeneration: model.Generation,
		})
		if err := r.Status().Update(ctx, model); err != nil {
			return result{}, fmt.Errorf("failed to update model status: %w", err)
		}

		// TODO: Instead of RequeueAfter, add a watch mapper.
		return result{Result: ctrl.Result{RequeueAfter: time.Minute}}, nil
	}

	trainingJob, err := r.trainerJob(ctx, model, baseModel, &dataset)
	if err != nil {
		log.Error(err, "unable to construct training Job")
		// No use in retrying...
		return result{}, nil
	}
	if result, err := reconcileJob(ctx, r.Client, model, trainingJob, apiv1.ConditionModelReady); !result.success {
		return result, err
	}

	return result{success: true}, nil
}

func (r *ModelReconciler) reconcileLoader(ctx context.Context, model *apiv1.Model) (result, error) {
	log := log.FromContext(ctx)

	loaderJob, err := r.loaderJob(ctx, model)
	if err != nil {
		log.Error(err, "unable to construct loader Job")
		// No use in retrying...
		return result{}, nil
	}

	if result, err := reconcileJob(ctx, r.Client, model, loaderJob, apiv1.ConditionModelReady); !result.success {
		return result, err
	}

	return result{success: true}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Model{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *ModelReconciler) authNServiceAccount(sa *corev1.ServiceAccount) error {
	if sa.Annotations == nil {
		sa.Annotations = make(map[string]string)
	}
	switch name := r.CloudContext.Name; name {
	case cloud.GCP:
		sa.Annotations["iam.gke.io/gcp-service-account"] = fmt.Sprintf("substratus-%s@%s.iam.gserviceaccount.com", sa.Name, r.CloudContext.GCP.ProjectID)
	default:
		return fmt.Errorf("unsupported cloud type: %q", name)
	}
	return nil
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

	env := []corev1.EnvVar{}
	// TODO(nstogner): Order by key to avoid randomness.
	for k, v := range model.Spec.Loader.Params {
		env = append(env, corev1.EnvVar{Name: "PARAM_" + strings.ToUpper(k), Value: v})
	}

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
					ServiceAccountName: modelTrainerServiceAccountName,
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
							Args: []string{"train.sh"},
							Env: []corev1.EnvVar{
								{
									Name:  "TRAIN_DATA_PATH",
									Value: "/data/" + dataset.Spec.Filename,
								},
								{
									Name:  "TRAIN_DATA_LIMIT",
									Value: fmt.Sprintf("%v", model.Spec.Trainer.Params.DataLimit),
								},
								{
									Name:  "TRAIN_BATCH_SIZE",
									Value: fmt.Sprintf("%v", model.Spec.Trainer.Params.BatchSize),
								},
								{
									Name:  "TRAIN_EPOCHS",
									Value: fmt.Sprintf("%v", model.Spec.Trainer.Params.Epochs),
								},
							},
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

	return job, nil
}
