package controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	ReasonCloning = "Cloning"
	ReasonPulling = "Pulling"
	ReasonPulled  = "Pulled"
)

// DatasetReconciler reconciles a Dataset object
type DatasetReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	CloudContext CloudContext
}

//+kubebuilder:rbac:groups=substratus.ai,resources=datasets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=datasets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=datasets/finalizers,verbs=update

func (r *DatasetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	lg.Info("Reconciling Dataset")
	defer lg.Info("Done reconciling Dataset")

	var dataset apiv1.Dataset
	if err := r.Get(ctx, req.NamespacedName, &dataset); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if dataset.Status.PVCName != "" {
		return ctrl.Result{}, nil
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataPullerServiceAccountName,
			Namespace: dataset.Namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		if err := r.CloudContext.AuthNServiceAccount(RuntimeDataPuller, sa); err != nil {
			return fmt.Errorf("failed to authenticate service account with cloud: %w", err)
		}
		return nil
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	rwoPVC, err := r.rwoDatasetPVC(&dataset)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to construct rwo pvc: %w", err)
	}
	job, err := r.pullerJob(ctx, &dataset)
	if err != nil {
		lg.Error(err, "unable to create builder Job")
		// No use in retrying...
		return ctrl.Result{}, nil
	}
	roxPVC, err := r.roxDatasetPVC(&dataset)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to construct rox pvc: %w", err)
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(roxPVC), &corev1.PersistentVolumeClaim{}); err != nil {
		// If the PVC intended for training does not exist yet, create the Job and initial PVC.
		if apierrors.IsNotFound(err) {
			if err := r.Patch(ctx, rwoPVC, client.Apply, client.FieldOwner("dataset-controller")); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to apply rwo pvc: %w", err)
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
		} else {
			return ctrl.Result{}, fmt.Errorf("getting rox pvc: %w", err)
		}
	}

	// If the Job has succeeded, create a ReadOnlyMany PVC for training that
	// will copy the data that was populated into the ReadWriteOnce PVC from
	// the Job.
	if err := r.Patch(ctx, roxPVC, client.Apply, client.FieldOwner("dataset-controller")); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to apply rox pvc: %w", err)
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(roxPVC), roxPVC); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting rox pvc: %w", err)
	}
	if roxPVC.Status.Phase != corev1.ClaimBound {
		lg.Info("ReadOnlyMany PVC not bound yet")

		meta.SetStatusCondition(&dataset.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             ReasonCloning,
			ObservedGeneration: dataset.Generation,
			Message:            "Waiting for dataset to be cloned from the ReadWriteOnce PersistentVolume into the ReadOnlyMany PersistentVolume.",
		})
		if err := r.Status().Update(ctx, &dataset); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// If the ReadOnlyMany PVC is bound, the dataset should be ready and it should be
	// OK to delete the ReadWriteOnce PVC. In order to do that, we need to delete the Pod
	// that was created by the Job. We rely on cascading deletion from the Job to delete
	// the completed Pod.
	bg := metav1.DeletePropagationBackground
	if err := r.Delete(ctx, job, &client.DeleteOptions{PropagationPolicy: &bg}); err != nil {
		return ctrl.Result{}, fmt.Errorf("deleting job: %w", err)
	}
	if err := r.Delete(ctx, rwoPVC); err != nil {
		return ctrl.Result{}, fmt.Errorf("deleting rwo pvc: %w", err)
	}

	meta.SetStatusCondition(&dataset.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonPulled,
		ObservedGeneration: dataset.Generation,
		Message:            "Dataset is ready (pulled and cloned into the ReadOnlyMany PersistentVolume).",
	})
	dataset.Status.PVCName = roxDatasetPVCName(&dataset)

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

//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *DatasetReconciler) pullerJob(ctx context.Context, dataset *apiv1.Dataset) (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataset.Name + "-data-puller",
			// Cross-Namespace owners not allowed, must be same as model:
			Namespace: dataset.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: int64Ptr(1000),
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
					Volumes: []corev1.Volume{{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: rwoDatasetPVCName(dataset),
							},
						},
					}},
					RestartPolicy: "Never",
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(dataset, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	return job, nil
}

func rwoDatasetPVCName(ds *apiv1.Dataset) string {
	return "rwo-dataset-" + ds.Name
}

func (r *DatasetReconciler) rwoDatasetPVC(dataset *apiv1.Dataset) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      rwoDatasetPVCName(dataset),
			Namespace: dataset.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: strPtr(datasetStorageClassName),
			AccessModes: []corev1.PersistentVolumeAccessMode{
				"ReadWriteOnce",
				//				"ReadOnlyMany",
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": dataset.Spec.Size,
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(dataset, pvc, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return pvc, nil
}

func roxDatasetPVCName(ds *apiv1.Dataset) string {
	return "dataset-" + ds.Name
}

const datasetStorageClassName = "substratus-training-data-standard"

func (r *DatasetReconciler) roxDatasetPVC(dataset *apiv1.Dataset) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      roxDatasetPVCName(dataset),
			Namespace: dataset.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: strPtr(datasetStorageClassName),
			DataSource: &corev1.TypedLocalObjectReference{
				Name: rwoDatasetPVCName(dataset),
				Kind: "PersistentVolumeClaim",
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				"ReadOnlyMany",
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": dataset.Spec.Size,
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(dataset, pvc, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return pvc, nil
}
