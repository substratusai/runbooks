package controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/resources"
	"github.com/substratusai/substratus/internal/sci"
)

// DatasetReconciler reconciles a Dataset object.
type DatasetReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	*ParamsReconciler

	Cloud cloud.Cloud
	SCI   sci.ControllerClient
}

func (r *DatasetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling Dataset")
	defer log.Info("Done reconciling Dataset")

	var dataset apiv1.Dataset
	if err := r.Get(ctx, req.NamespacedName, &dataset); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if dataset.GetImage() == "" {
		// Image must be building.
		return ctrl.Result{}, nil
	}

	if result, err := r.ReconcileParamsConfigMap(ctx, &dataset); !result.success {
		return result.Result, err
	}

	if result, err := r.reconcileData(ctx, &dataset); !result.success {
		return result.Result, err
	}

	return ctrl.Result{}, nil
}

//+kubebuilder:rbac:groups=substratus.ai,resources=datasets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=datasets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=datasets/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// SetupWithManager sets up the controller with the Manager.
func (r *DatasetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Dataset{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *DatasetReconciler) reconcileData(ctx context.Context, dataset *apiv1.Dataset) (result, error) {
	log := log.FromContext(ctx)

	if dataset.Status.Ready {
		return result{success: true}, nil
	}

	dataset.Status.Artifacts.URL = r.Cloud.ObjectArtifactURL(dataset).String()

	// ServiceAccount for the loader job.
	// Within the context of GCP, this ServiceAccount will need IAM permissions
	// to write the GCS bucket containing training data.
	if result, err := reconcileServiceAccount(ctx, r.Cloud, r.SCI, r.Client, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataLoaderServiceAccountName,
			Namespace: dataset.Namespace,
		},
	}); !result.success {
		return result, err
	}

	// Job that will run the data-loader image that was built by the previous Job.
	loadJob, err := r.loadJob(ctx, dataset)
	if err != nil {
		log.Error(err, "unable to construct data-loader Job")
		// No use in retrying...
		return result{}, nil
	}

	dataset.Status.Ready = false
	meta.SetStatusCondition(dataset.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionComplete,
		Status:             metav1.ConditionFalse,
		Reason:             apiv1.ReasonJobNotComplete,
		ObservedGeneration: dataset.Generation,
		Message:            "Waiting for data loader Job to complete",
	})
	if err := r.Status().Update(ctx, dataset); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	jobResult, err := reconcileJob(ctx, r.Client, loadJob)
	if !jobResult.success {
		if jobResult.failure {
			meta.SetStatusCondition(dataset.GetConditions(), metav1.Condition{
				Type:               apiv1.ConditionComplete,
				Status:             metav1.ConditionFalse,
				Reason:             apiv1.ReasonJobFailed,
				ObservedGeneration: dataset.Generation,
			})
			if err := r.Status().Update(ctx, dataset); err != nil {
				return result{}, fmt.Errorf("updating status: %w", err)
			}
		}
		return jobResult, err
	}

	dataset.Status.Ready = true
	meta.SetStatusCondition(dataset.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionComplete,
		Status:             metav1.ConditionTrue,
		Reason:             apiv1.ReasonJobComplete,
		ObservedGeneration: dataset.Generation,
	})
	if err := r.Status().Update(ctx, dataset); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	return result{success: true}, nil
}

func (r *DatasetReconciler) loadJob(ctx context.Context, dataset *apiv1.Dataset) (*batchv1.Job, error) {
	const containerName = "load"
	envVars, err := resolveEnv(dataset.Spec.Env)
	if err != nil {
		return nil, fmt.Errorf("resolving env: %w", err)
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataset.Name + "-data-loader",
			// Cross-Namespace owners not allowed, must be same as dataset:
			Namespace: dataset.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(2)), // TotalRetries = BackoffLimit + 1
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"kubectl.kubernetes.io/default-container": containerName,
					},
					Labels: map[string]string{
						"dataset": dataset.Name,
						"role":    "run",
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: ptr.To(int64(3003)),
					},
					ServiceAccountName: dataLoaderServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:    containerName,
							Image:   dataset.GetImage(),
							Command: dataset.Spec.Command,
							Env:     envVars,
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	if err := mountParamsConfigMap(&job.Spec.Template.Spec, dataset, containerName); err != nil {
		return nil, fmt.Errorf("mounting params configmap: %w", err)
	}

	if err := r.Cloud.MountBucket(&job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, dataset, cloud.MountBucketConfig{
		Name: "artifacts",
		Mounts: []cloud.BucketMount{
			{BucketSubdir: "artifacts", ContentSubdir: "artifacts"},
		},
		Container: containerName,
		ReadOnly:  false,
	}); err != nil {
		return nil, fmt.Errorf("mounting bucket: %w", err)
	}

	if err := controllerutil.SetControllerReference(dataset, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	if err := resources.Apply(&job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, containerName,
		r.Cloud.Name(), dataset.Spec.Resources); err != nil {
		return nil, fmt.Errorf("applying resources: %w", err)
	}

	return job, nil
}
