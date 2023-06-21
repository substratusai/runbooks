package controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/substratusai/substratus/api/v1"
)

const (
	ReasonPulling = "Pulling"
	ReasonPulled  = "Pulled"
)

// DatasetReconciler reconciles a Dataset object.
type DatasetReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	CloudContext *CloudContext
}

//+kubebuilder:rbac:groups=substratus.ai,resources=datasets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=datasets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=datasets/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *DatasetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	lg.Info("Reconciling Dataset")
	defer lg.Info("Done reconciling Dataset")

	var dataset apiv1.Dataset
	if err := r.Get(ctx, req.NamespacedName, &dataset); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if ready := meta.FindStatusCondition(dataset.Status.Conditions, ConditionReady); ready != nil && ready.Status == metav1.ConditionTrue {
		return ctrl.Result{}, nil
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataPullerServiceAccountName,
			Namespace: dataset.Namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		return r.authNServiceAccount(sa)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	job, err := r.pullerJob(ctx, &dataset)
	if err != nil {
		lg.Error(err, "unable to create builder Job")
		// No use in retrying...
		return ctrl.Result{}, nil
	}
	if err := r.Create(ctx, job); client.IgnoreAlreadyExists(err) != nil {
		return ctrl.Result{}, fmt.Errorf("creating Job: %w", err)
	}

	meta.SetStatusCondition(&dataset.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             ReasonPulling,
		ObservedGeneration: dataset.Generation,
		Message:            "Waiting for dataset to be stored in the PersistentVolume by the data puller Job.",
	})
	if err := r.Status().Update(ctx, &dataset); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(job), job); err != nil {
		return ctrl.Result{}, fmt.Errorf("geting Job: %w", err)
	}
	if job.Status.Succeeded < 1 {
		lg.Info("Job has not succeeded yet")
		// Allow Job watch to requeue.
		return ctrl.Result{}, nil
	}

	meta.SetStatusCondition(&dataset.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonPulled,
		ObservedGeneration: dataset.Generation,
		Message:            "Dataset is ready (pulled and cloned into the ReadOnlyMany PersistentVolume).",
	})
	if err := r.Status().Update(ctx, &dataset); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatasetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Dataset{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

const dataPullerServiceAccountName = "data-puller"

func (r *DatasetReconciler) authNServiceAccount(sa *corev1.ServiceAccount) error {
	if sa.Annotations == nil {
		sa.Annotations = make(map[string]string)
	}
	switch typ := r.CloudContext.CloudType; typ {
	case CloudTypeGCP:
		sa.Annotations["iam.gke.io/gcp-service-account"] = fmt.Sprintf("substratus-data-puller@%s.iam.gserviceaccount.com", r.CloudContext.GCP.ProjectID)
	default:
		return fmt.Errorf("unsupported cloud type: %q", r.CloudContext.CloudType)
	}
	return nil
}

func (r *DatasetReconciler) pullerJob(ctx context.Context, dataset *apiv1.Dataset) (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataset.Name + "-data-puller",
			// Cross-Namespace owners not allowed, must be same as model:
			Namespace: dataset.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  int64Ptr(1001),
						RunAsGroup: int64Ptr(2002),
						FSGroup:    int64Ptr(3003),
					},
					ServiceAccountName: dataPullerServiceAccountName,
					Containers: []corev1.Container{{
						Name: "puller",
						// TODO: Support gcs:// and s3:// ... and others?
						// Consider using:
						// - Source-specific containers (i.e. gsutil, aws cli)
						// - A universal data puller cli (i.e. rclone).
						Image: "curlimages/curl",
						Args:  []string{"-o", "/data/" + dataset.Spec.Source.Filename, dataset.Spec.Source.URL},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "data",
							MountPath: "/data",
							SubPath:   string(dataset.UID),
						}},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					}},
					Volumes:       []corev1.Volume{},
					RestartPolicy: "Never",
				},
			},
		},
	}

	switch r.CloudContext.CloudType {
	case CloudTypeGCP:
		// GKE will injects GCS Fuse sidecar based on this annotation.
		job.Spec.Template.Annotations["gke-gcsfuse/volumes"] = "true"
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver: "gcsfuse.csi.storage.gke.io",
					VolumeAttributes: map[string]string{
						"bucketName":   r.CloudContext.GCP.ProjectID + "-substratus-datasets",
						"mountOptions": "implicit-dirs,uid=1001,gid=3003",
					},
				},
			},
		})
		dataset.Status.URL = "gcs://" + r.CloudContext.GCP.ProjectID + "-substratus-datasets" +
			"/" + string(dataset.UID) + "/" + dataset.Spec.Source.Filename
	}

	if err := controllerutil.SetControllerReference(dataset, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	return job, nil
}
