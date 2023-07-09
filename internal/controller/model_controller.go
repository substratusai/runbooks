package controller

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/builder"
	"github.com/substratusai/substratus/internal/cloud"
)

const (
	ConditionReady = "Ready"

	ReasonTraining       = "Training"
	ReasonBuilding       = "Building"
	ReasonBuiltAndPushed = "BuiltAndPushed"

	ReasonSourceModelNotFound = "SourceModelNotFound"
	ReasonSourceModelNotReady = "SourceModelNotReady"
	TrainingDatasetNotReady   = "TrainingDatasetNotReady"

	modelTrainerServiceAccountName = "model-trainer"
)

// ModelReconciler reconciles a Model object.
type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	*builder.ContainerReconciler

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

	if result, err := r.ReconcileContainer(ctx, &model); !result.Complete {
		return result.Result, err
	}

	if model.Status.URL != "" {
		return ctrl.Result{}, nil
	}

	if model.Spec.Trainer != nil {
		if result, err := r.reconcileTrainer(ctx, &model); !result.Complete {
			return result.Result, err
		}
	} else if model.Spec.Loader != nil {
		if result, err := r.reconcileLoader(ctx, &model); !result.Complete {
			return result.Result, err
		}
	} else {
		log.Error(errors.New("no trainer or loader specified"), "this point should never have been reached if the model is valid")
		// No use in retrying (returning an error).
		return ctrl.Result{}, nil
	}

	meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonBuiltAndPushed,
		ObservedGeneration: model.Generation,
	})

	if err := r.Status().Update(ctx, &model); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status with Ready=%v, Reason=%v: %w", metav1.ConditionTrue, ReasonBuiltAndPushed, err)
	}

	return ctrl.Result{}, nil
}

type Result struct {
	ctrl.Result
	Complete bool
}

func (r *ModelReconciler) reconcileTrainer(ctx context.Context, model *apiv1.Model) (Result, error) {
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
		return Result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	var sourceModel apiv1.Model
	if model.Spec.Trainer.BaseModel != nil {
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Trainer.BaseModel.Name}, &sourceModel); err != nil {
			if apierrors.IsNotFound(err) {
				// Update this Model's status.
				meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
					Type:               ConditionReady,
					Status:             metav1.ConditionFalse,
					Reason:             ReasonSourceModelNotFound,
					ObservedGeneration: model.Generation,
				})
				if err := r.Status().Update(ctx, model); err != nil {
					return Result{}, fmt.Errorf("failed to update model status: %w", err)
				}

				// TODO: Implement watch on source Model.
				return Result{Result: ctrl.Result{RequeueAfter: 3 * time.Second}}, nil
			}

			return Result{}, fmt.Errorf("getting source model: %w", err)
		}
		if !isReady(&sourceModel) {
			// Update this Model's status.
			meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
				Type:               ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonSourceModelNotReady,
				ObservedGeneration: model.Generation,
			})
			if err := r.Status().Update(ctx, model); err != nil {
				return Result{}, fmt.Errorf("failed to update model status: %w", err)
			}

			// TODO: Instead of RequeueAfter, add a watch that maps to .spec.source.modelName
			return Result{Result: ctrl.Result{RequeueAfter: time.Minute}}, nil
		}
	}

	// Run training Job first, results will be stored in a volume and used by the builder Job.

	trainingJob, err := r.trainingJob(ctx, model, &sourceModel)
	if err != nil {
		log.Error(err, "unable to create training Job")
		// No use in retrying...
		return Result{}, nil
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(trainingJob), trainingJob); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, trainingJob); client.IgnoreAlreadyExists(err) != nil {
				return Result{}, fmt.Errorf("creating Job: %w", err)
			}
		} else {
			return Result{}, fmt.Errorf("getting Job: %w", err)
		}
	}
	if trainingJob.Status.Succeeded < 1 {
		log.Info("Training Job has not succeeded yet")

		meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             ReasonTraining,
			ObservedGeneration: model.Generation,
		})
		if err := r.Status().Update(ctx, model); err != nil {
			return Result{}, fmt.Errorf("updating status with Ready=%v, Reason=%v: %w", metav1.ConditionFalse, ReasonTraining, err)
		}

		// Allow Job watch to requeue.
		return Result{}, nil
	}

	return Result{Complete: true}, nil
}

func (r *ModelReconciler) reconcileLoader(ctx context.Context, model *apiv1.Model) (Result, error) {
	log := log.FromContext(ctx)
	_ = log
	return Result{Complete: true}, nil
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

func (r *ModelReconciler) trainingJob(ctx context.Context, model *apiv1.Model, sourceModel *apiv1.Model) (*batchv1.Job, error) {
	var job *batchv1.Job

	// TODO: Validate the Model before this stage to ensure .spec.training is set alongside .spec.source.modelName

	var dataset apiv1.Dataset
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Trainer.Dataset.Name}, &dataset); err != nil {
		return nil, fmt.Errorf("getting source model: %w", err)
	}
	if ready := meta.FindStatusCondition(dataset.Status.Conditions, ConditionReady); ready == nil || ready.Status != metav1.ConditionTrue || dataset.Status.URL == "" {
		// Update this Model's status.
		meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             TrainingDatasetNotReady,
			ObservedGeneration: model.Generation,
		})
		if err := r.Status().Update(ctx, model); err != nil {
			return nil, fmt.Errorf("failed to update model status: %w", err)
		}

		return nil, nil
	}

	annotations := make(map[string]string)
	volumes := []corev1.Volume{
		{
			Name: "training",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver: "gcsfuse.csi.storage.gke.io",
					VolumeAttributes: map[string]string{
						"bucketName":   r.CloudContext.GCP.ProjectID + "-substratus-training",
						"mountOptions": "implicit-dirs,uid=0,gid=3003",
					},
				},
			},
		},
	}

	var dataSubpath string
	switch r.CloudContext.Name {
	case cloud.GCP:
		u, err := url.Parse(dataset.Status.URL)
		if err != nil {
			return nil, fmt.Errorf("parsing dataset url: %w", err)
		}
		bucket := u.Host
		dataSubpath = strings.TrimPrefix(filepath.Dir(u.Path), "/")

		// GKE will injects GCS Fuse sidecar based on this annotation.
		annotations["gke-gcsfuse/volumes"] = "true"
		volumes = append(volumes, corev1.Volume{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   "gcsfuse.csi.storage.gke.io",
					ReadOnly: boolPtr(true),
					VolumeAttributes: map[string]string{
						"bucketName":   bucket,
						"mountOptions": "implicit-dirs,uid=0,gid=3003",
					},
				},
			},
		})
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
			BackoffLimit: int32Ptr(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  int64Ptr(0),
						RunAsGroup: int64Ptr(0),
						FSGroup:    int64Ptr(3003),
					},
					ServiceAccountName: modelTrainerServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  trainerContainerName,
							Image: sourceModel.Spec.Container.Image,
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
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/data",
									SubPath:   dataSubpath,
									ReadOnly:  true,
								},
								{
									Name:      "training",
									MountPath: "/model/logs",
									SubPath:   string(model.UID) + "/logs",
								},
								{
									Name:      "training",
									MountPath: "/model/trained",
									SubPath:   string(model.UID) + "/trained",
								},
							},
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
