package controller

import (
	"context"
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
	modelBuilderServiceAccountName = "model-builder"
)

// ModelReconciler reconciles a Model object.
type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	CloudContext *CloudContext
	*RuntimeManager
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
	lg := log.FromContext(ctx)

	lg.Info("Reconciling Model")
	defer lg.Info("Done reconciling Model")

	var model apiv1.Model
	if err := r.Get(ctx, req.NamespacedName, &model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var sourceModel apiv1.Model
	if model.Spec.Source.ModelName != "" {
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Source.ModelName}, &sourceModel); err != nil {
			if apierrors.IsNotFound(err) {
				// Update this Model's status.
				meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
					Type:               ConditionReady,
					Status:             metav1.ConditionFalse,
					Reason:             ReasonSourceModelNotFound,
					ObservedGeneration: model.Generation,
				})
				if err := r.Status().Update(ctx, &model); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to update model status: %w", err)
				}

				// TODO: Implement watch on source Model.
				return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
			}

			return ctrl.Result{}, fmt.Errorf("getting source model: %w", err)
		}
		if ready := meta.FindStatusCondition(sourceModel.Status.Conditions, ConditionReady); ready == nil || ready.Status != metav1.ConditionTrue || sourceModel.Status.ContainerImage == "" {
			// Update this Model's status.
			meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
				Type:               ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonSourceModelNotReady,
				ObservedGeneration: model.Generation,
			})
			if err := r.Status().Update(ctx, &model); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to update model status: %w", err)
			}

			// TODO: Instead of RequeueAfter, add a watch that maps to .spec.source.modelName
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
	}

	if model.Status.ContainerImage != "" {
		// TODO: Check container registry directly instead.
		return ctrl.Result{}, nil
	}

	trainerSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      modelTrainerServiceAccountName,
			Namespace: model.Namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, trainerSA, func() error {
		return r.authNServiceAccount(trainerSA)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}
	builderSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      modelBuilderServiceAccountName,
			Namespace: model.Namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, builderSA, func() error {
		return r.authNServiceAccount(builderSA)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	if model.Spec.Training != nil {
		// Run training Job first, results will be stored in a volume and used by the builder Job.

		trainingJob, err := r.trainingJob(ctx, &model, &sourceModel)
		if err != nil {
			lg.Error(err, "unable to create training Job")
			// No use in retrying...
			return ctrl.Result{}, nil
		}
		if err := r.Get(ctx, client.ObjectKeyFromObject(trainingJob), trainingJob); err != nil {
			if apierrors.IsNotFound(err) {
				if err := r.Create(ctx, trainingJob); client.IgnoreAlreadyExists(err) != nil {
					return ctrl.Result{}, fmt.Errorf("creating Job: %w", err)
				}
			} else {
				return ctrl.Result{}, fmt.Errorf("getting Job: %w", err)
			}
		}
		if trainingJob.Status.Succeeded < 1 {
			lg.Info("Training Job has not succeeded yet")

			meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
				Type:               ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonTraining,
				ObservedGeneration: model.Generation,
			})
			if err := r.Status().Update(ctx, &model); err != nil {
				return ctrl.Result{}, fmt.Errorf("updating status with Ready=%v, Reason=%v: %w", metav1.ConditionFalse, ReasonTraining, err)
			}

			// Allow Job watch to requeue.
			return ctrl.Result{}, nil
		}
	}

	// Create a Job that will build a container image with the model in it.
	// If a trainer Job was run, it will use the results of that Job to build the image.
	buildJob, err := r.buildJob(ctx, &model, &sourceModel)
	if err != nil {
		lg.Error(err, "unable to create builder Job")
		// No use in retrying...
		return ctrl.Result{}, nil
	}

	if err := r.Create(ctx, buildJob); client.IgnoreAlreadyExists(err) != nil {
		return ctrl.Result{}, fmt.Errorf("creating Job: %w", err)
	}

	meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             ReasonBuilding,
		ObservedGeneration: model.Generation,
	})
	if err := r.Status().Update(ctx, &model); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status with Ready=%v, Reason=%v: %w", metav1.ConditionFalse, ReasonBuilding, err)
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(buildJob), buildJob); err != nil {
		return ctrl.Result{}, fmt.Errorf("geting Job: %w", err)
	}
	if buildJob.Status.Succeeded < 1 {
		lg.Info("Job has not succeeded yet")

		// Allow Job watch to requeue.
		return ctrl.Result{}, nil
	}

	meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonBuiltAndPushed,
		ObservedGeneration: model.Generation,
	})
	model.Status.ContainerImage = r.modelImage(&model)

	if err := r.Status().Update(ctx, &model); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status with Ready=%v, Reason=%v: %w", metav1.ConditionTrue, ReasonBuiltAndPushed, err)
	}

	return ctrl.Result{}, nil
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
	switch typ := r.CloudContext.CloudType; typ {
	case CloudTypeGCP:
		sa.Annotations["iam.gke.io/gcp-service-account"] = fmt.Sprintf("substratus-%s@%s.iam.gserviceaccount.com", sa.Name, r.CloudContext.GCP.ProjectID)
	default:
		return fmt.Errorf("unsupported cloud type: %q", r.CloudContext.CloudType)
	}
	return nil
}

func (r *ModelReconciler) modelImage(m *apiv1.Model) string {
	switch typ := r.CloudContext.CloudType; typ {
	case CloudTypeGCP:
		// Assuming this is Google Artifact Registry named "substratus".
		return fmt.Sprintf("%s-docker.pkg.dev/%s/substratus/model-%s-%s", r.CloudContext.GCP.Region(), r.CloudContext.GCP.ProjectID, m.Namespace, m.Name)
	default:
		panic("unsupported cloud type: " + typ)
	}
}

func (r *ModelReconciler) buildJob(ctx context.Context, model *apiv1.Model, sourceModel *apiv1.Model) (*batchv1.Job, error) {

	var job *batchv1.Job

	annotations := map[string]string{}

	buildArgs := []string{
		"--dockerfile=Dockerfile",
		"--destination=" + r.modelImage(model),
		// Cache will default to the image registry.
		"--cache=true",
		// Disable compressed caching to decrease memory usage.
		// (See https://github.com/GoogleContainerTools/kaniko/blob/main/README.md#flag---compressed-caching)
		"--compressed-caching=false",
	}

	var initContainers []corev1.Container
	var volumeMounts []corev1.VolumeMount
	var volumes []corev1.Volume

	if model.Spec.Training != nil {
		const trainedDockerfile = `# Dockerfile for copying trained model into a new image.
ARG SRC_IMG
FROM $SRC_IMG
COPY trained /model/saved
COPY logs /model/logs
RUN rm -rf /model/trained
`
		// Add an init container that will create a Dockerfile.
		initContainers = append(initContainers, corev1.Container{
			Name:  "dockerfile-templater",
			Image: "busybox",
			Args: []string{
				"sh",
				"-c",
				"echo '" + trainedDockerfile + "' > /workspace/Dockerfile",
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "workspace",
					MountPath: "/workspace",
				},
			},
		})

		buildArgs = append(buildArgs, fmt.Sprintf("--build-arg=SRC_IMG=%v", sourceModel.Status.ContainerImage))
		volumeMounts = []corev1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/workspace/Dockerfile",
				SubPath:   "Dockerfile",
			},
			{
				Name:      "training",
				MountPath: "/workspace/trained",
				SubPath:   string(model.UID) + "/trained",
			},
			{
				Name:      "training",
				MountPath: "/workspace/logs",
				SubPath:   string(model.UID) + "/logs",
			},
		}

		volumes = []corev1.Volume{
			{
				Name: "workspace",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
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

		annotations["gke-gcsfuse/volumes"] = "true"
	} else {
		const dockerfileWithTini = `
# Add Tini
ENV TINI_VERSION v0.19.0
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini /tini
RUN chmod +x /tini
ENTRYPOINT ["/tini", "--"]
`
		cloneArgs := []string{
			"clone",
			model.Spec.Source.Git.URL,
		}
		if model.Spec.Source.Git.Branch != "" {
			cloneArgs = append(cloneArgs, "--branch", model.Spec.Source.Git.Branch)
		}
		cloneArgs = append(cloneArgs, "/workspace")

		if model.Spec.Source.Git.Path != "" {
			buildArgs = append(buildArgs, "--context-sub-path="+model.Spec.Source.Git.Path)
		}

		// Add an init container that will clone the Git repo and
		// another that will append tini to the Dockerfile.
		initContainers = append(initContainers,
			corev1.Container{
				Name:  "git-clone",
				Image: "alpine/git",
				Args:  cloneArgs,
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "workspace",
						MountPath: "/workspace",
					},
				},
			},
			corev1.Container{
				Name:  "dockerfile-tini-appender",
				Image: "busybox",
				Args: []string{
					"sh",
					"-c",
					"echo '" + dockerfileWithTini + "' >> /workspace/Dockerfile",
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "workspace",
						MountPath: "/workspace",
					},
				},
			},
		)

		volumeMounts = []corev1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/workspace",
			},
		}
		volumes = []corev1.Volume{
			{
				Name: "workspace",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}
	}

	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: model.Name + "-model-builder",
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
					InitContainers: initContainers,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  int64Ptr(0),
						RunAsGroup: int64Ptr(0),
						FSGroup:    int64Ptr(3003),
					},
					ServiceAccountName: modelBuilderServiceAccountName,
					Containers: []corev1.Container{{
						Name:         RuntimeBuilder,
						Image:        "gcr.io/kaniko-project/executor:latest",
						Args:         buildArgs,
						VolumeMounts: volumeMounts,
					}},
					RestartPolicy: "Never",
					Volumes:       volumes,
				},
			},
		},
	}

	if err := r.SetResources(model, &job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, RuntimeBuilder); err != nil {
		return nil, fmt.Errorf("setting pod resources: %w", err)
	}

	if err := controllerutil.SetControllerReference(model, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	return job, nil
}

func (r *ModelReconciler) trainingJob(ctx context.Context, model *apiv1.Model, sourceModel *apiv1.Model) (*batchv1.Job, error) {
	var job *batchv1.Job

	// TODO: Validate the Model before this stage to ensure .spec.training is set alongside .spec.source.modelName

	var dataset apiv1.Dataset
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Training.DatasetName}, &dataset); err != nil {
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
	switch r.CloudContext.CloudType {
	case CloudTypeGCP:
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
							Name:  RuntimeTrainer,
							Image: sourceModel.Status.ContainerImage,
							// NOTE: tini should be installed as the ENTRYPOINT the image and will be used
							// to execute this script.
							Args: []string{"train.sh"},
							Env: []corev1.EnvVar{
								{
									Name:  "TRAINING_DATA_PATH",
									Value: "/data/" + dataset.Spec.Source.Filename,
								},
								{
									Name:  "TRAINING_DATA_LIMIT",
									Value: fmt.Sprintf("%v", model.Spec.Training.Params.DataLimit),
								},
								{
									Name:  "TRAINING_BATCH_SIZE",
									Value: fmt.Sprintf("%v", model.Spec.Training.Params.BatchSize),
								},
								{
									Name:  "TRAINING_EPOCHS",
									Value: fmt.Sprintf("%v", model.Spec.Training.Params.Epochs),
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

	if err := r.SetResources(model, &job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, RuntimeTrainer); err != nil {
		return nil, fmt.Errorf("setting pod resources: %w", err)
	}

	if err := controllerutil.SetControllerReference(model, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	return job, nil
}
