package controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ptr "k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/resources"
)

// DatasetReconciler reconciles a Dataset object.
type DatasetReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	*ContainerImageReconciler

	Cloud cloud.Cloud
}

//+kubebuilder:rbac:groups=substratus.ai,resources=datasets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=datasets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=datasets/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch

func (r *DatasetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling Dataset")
	defer log.Info("Done reconciling Dataset")

	var dataset apiv1.Dataset
	if err := r.Get(ctx, req.NamespacedName, &dataset); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if result, err := r.ReconcileContainerImage(ctx, &dataset); !result.success {
		return result.Result, err
	}

	if result, err := r.reconcileData(ctx, &dataset); !result.success {
		return result.Result, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatasetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Dataset{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *DatasetReconciler) reconcileData(ctx context.Context, dataset *apiv1.Dataset) (result, error) {
	log := log.FromContext(ctx)

	if dataset.Status.URL != "" {
		return result{success: true}, nil
	}

	// ServiceAccount for the loader job.
	// Within the context of GCP, this ServiceAccount will need IAM permissions
	// to write the GCS bucket containing training data.
	if result, err := reconcileCloudServiceAccount(ctx, r.Cloud, r.Client, &corev1.ServiceAccount{
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
		Type:               apiv1.ConditionLoaded,
		Status:             metav1.ConditionFalse,
		Reason:             apiv1.ReasonJobNotComplete,
		ObservedGeneration: dataset.Generation,
		Message:            fmt.Sprintf("Waiting for data loader Job to complete"),
	})
	if err := r.Status().Update(ctx, dataset); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	if result, err := reconcileJob(ctx, r.Client, loadJob, apiv1.ConditionLoaded); !result.success {
		return result, err
	}

	dataset.Status.Ready = true
	dataset.Status.URL = r.Cloud.ObjectArtifactURL(dataset)
	meta.SetStatusCondition(dataset.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionLoaded,
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
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataset.Name + "-data-loader",
			// Cross-Namespace owners not allowed, must be same as dataset:
			Namespace: dataset.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"kubectl.kubernetes.io/default-container": containerName,
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  ptr.Int64(1001),
						RunAsGroup: ptr.Int64(2002),
						FSGroup:    ptr.Int64(3003),
					},
					ServiceAccountName: dataLoaderServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:    containerName,
							Image:   dataset.Spec.Image.Name,
							Command: dataset.Spec.Command,
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	if err := r.Cloud.MountBucket(&job.Spec.Template.ObjectMeta, &job.Spec.Template.Spec, dataset, cloud.MountBucketConfig{
		Name: "dataset",
		Mounts: []cloud.BucketMount{
			{BucketSubdir: "data", ContentSubdir: "data"},
			{BucketSubdir: "logs", ContentSubdir: "logs"},
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
