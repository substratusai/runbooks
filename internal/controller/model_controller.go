package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

	ReasonBuilding       = "Building"
	ReasonBuiltAndPushed = "BuiltAndPushed"

	ReasonSourceModelNotReady = "SourceModelNotReady"
	TrainingDatasetNotReady   = "TrainingDatasetNotReady"
)

// ModelReconciler reconciles a Model object
type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Config       ModelReconcilerConfig
	CloudContext CloudContext
	GPUType      GPUType
}

type ModelReconcilerConfig struct {
	ImageRegistry string
}

//+kubebuilder:rbac:groups=substratus.ai,resources=models,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=models/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=models/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Model object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	lg.Info("Reconciling Model")
	defer lg.Info("Done reconciling Model")

	var model apiv1.Model
	if err := r.Get(ctx, req.NamespacedName, &model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if model.Status.ContainerImage != "" {
		// TODO: Check container registry directly instead.
		return ctrl.Result{}, nil
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildServiceAccountName,
			Namespace: model.Namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		if err := r.CloudContext.AuthNServiceAccount(RuntimeBuilder, sa); err != nil {
			return fmt.Errorf("failed to authenticate service account with cloud: %w", err)
		}
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	// Create a Job that will build an image with the model in it using kaniko.
	// Use server-side apply.
	job, err := r.buildJob(ctx, &model)
	if err != nil {
		lg.Error(err, "unable to create builder Job")
		// No use in retrying...
		return ctrl.Result{}, nil
	}
	if job == nil {
		// TODO: Instead of RequeueAfter, add a watch that maps to .spec.source.modelName
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	if err := r.Create(ctx, job); client.IgnoreAlreadyExists(err) != nil {
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

	if err := r.Get(ctx, client.ObjectKeyFromObject(job), job); err != nil {
		return ctrl.Result{}, fmt.Errorf("geting Job: %w", err)
	}
	if job.Status.Succeeded < 1 {
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
	model.Status.ContainerImage = modelImage(&model, r.Config.ImageRegistry)

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

func modelImage(m *apiv1.Model, registry string) string {
	return registry + "/" + m.Namespace + "-" + m.Name + ":" + m.Spec.Version
}

const buildServiceAccountName = "image-builder"

//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *ModelReconciler) buildJob(ctx context.Context, model *apiv1.Model) (*batchv1.Job, error) {
	buildArgs := []string{
		"--dockerfile=Dockerfile",
		"--destination=" + modelImage(model, r.Config.ImageRegistry),
		// Cache will default to the image registry.
		"--cache=true",
		// Disable compressed caching to decrease memory usage.
		// (See https://github.com/GoogleContainerTools/kaniko/blob/main/README.md#flag---compressed-caching)
		"--compressed-caching=false",
	}

	builderMounts := []corev1.VolumeMount{}
	volumes := []corev1.Volume{}

	var initContainers []corev1.Container

	switch model.Spec.Source.Type() {
	case apiv1.ModelSourceTypeGit:
		url := model.Spec.Source.Git.URL
		if model.Spec.Source.Git.Branch != "" {
			url = url + "#refs/heads/" + model.Spec.Source.Git.Branch
		}
		buildArgs = append(buildArgs, "--context="+"git://"+url)
		if model.Spec.Source.Git.Path != "" {
			buildArgs = append(buildArgs, "--context-sub-path="+model.Spec.Source.Git.Path)
		}
	case apiv1.ModelSourceTypeModel:
		var otherModel apiv1.Model
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Source.ModelName}, &otherModel); err != nil {
			return nil, fmt.Errorf("getting source model: %w", err)
		}
		if ready := meta.FindStatusCondition(otherModel.Status.Conditions, ConditionReady); ready == nil || ready.Status != metav1.ConditionTrue || otherModel.Status.ContainerImage == "" {
			// Update this Model's status.
			meta.SetStatusCondition(&model.Status.Conditions, metav1.Condition{
				Type:               ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonSourceModelNotReady,
				ObservedGeneration: model.Generation,
			})
			if err := r.Status().Update(ctx, model); err != nil {
				return nil, fmt.Errorf("failed to update model status: %w", err)
			}

			return nil, nil
		}

		var dataset apiv1.Dataset
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: model.Spec.Training.DatasetName}, &dataset); err != nil {
			return nil, fmt.Errorf("getting source model: %w", err)
		}
		if ready := meta.FindStatusCondition(dataset.Status.Conditions, ConditionReady); ready == nil || ready.Status != metav1.ConditionTrue || dataset.Status.PVCName == "" {
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

		buildArgs = append(buildArgs,
			fmt.Sprintf("--build-arg=SRC_IMG=%v", otherModel.Status.ContainerImage),
			//"--context=/build-contexts/model-source",
		)

		builderMounts = append(builderMounts,
			corev1.VolumeMount{
				Name:      "builder-model-source-context",
				MountPath: "/workspace/Dockerfile",
				SubPath:   "Dockerfile",
			},
			corev1.VolumeMount{
				Name:      "trained",
				MountPath: "/workspace/trained",
			},
			corev1.VolumeMount{
				Name:      "logs",
				MountPath: "/workspace/logs",
			},
		)

		volumes = append(volumes,
			corev1.Volume{
				Name: "logs",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			corev1.Volume{
				Name: "trained",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			corev1.Volume{
				Name: "builder-model-source-context",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "builder-model-source-context"},
					},
				},
			},
			corev1.Volume{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: dataset.Status.PVCName,
					},
				},
			},
		)

		// TODO: Validate the Model before this stage to ensure .spec.training is set alongside .spec.source.modelName
		if model.Spec.Training != nil {
			initContainers = append(initContainers,
				corev1.Container{
					Name:    RuntimeTrainer,
					Image:   otherModel.Status.ContainerImage,
					Command: []string{"train.sh"},
					Env: []corev1.EnvVar{
						{
							Name:  "DATA_PATH",
							Value: "/data/" + dataset.Spec.Source.Filename,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/data",
						},
						{
							Name:      "logs",
							MountPath: "/model/logs",
						},
						{
							Name:      "trained",
							MountPath: "/model/trained",
						},
					},
				},
			)
		}
	}

	// TODO: These Job Pods probably need to be constrained to Nodes in the same zone as the Dataset
	// if one is being referenced.
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: model.Name + "-image-builder",
			// Cross-Namespace owners not allowed, must be same as model:
			Namespace: model.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(1),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: buildServiceAccountName,
					InitContainers:     initContainers,
					Containers: []corev1.Container{{
						Name:         RuntimeBuilder,
						Image:        "gcr.io/kaniko-project/executor:latest",
						Args:         buildArgs,
						VolumeMounts: builderMounts,
					}},
					Volumes:       volumes,
					RestartPolicy: "Never",
				},
			},
		},
	}

	if model.Spec.Training != nil {
		if err := setRuntimeResources(model, &job.Spec.Template.Spec, r.GPUType, RuntimeTrainer); err != nil {
			return nil, fmt.Errorf("setting pod resources: %w", err)
		}
	}
	if err := setRuntimeResources(model, &job.Spec.Template.Spec, r.GPUType, RuntimeBuilder); err != nil {
		return nil, fmt.Errorf("setting pod resources: %w", err)
	}

	if err := controllerutil.SetControllerReference(model, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	return job, nil
}

func waitForJobToComplete(ctx context.Context, c client.Client, job *batchv1.Job) error {
	for i := 0; i < 120; i++ {

		if err := c.Get(ctx, client.ObjectKeyFromObject(job), job); err != nil {
			return err
		}

		if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
			return nil
		}

		time.Sleep(time.Second)
	}

	return errors.New("timed out")
}
