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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/substratusai/substratus/api/v1"
)

const (
	ReasonLoading = "Loading"
	ReasonLoaded  = "Loaded"
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

	bldrSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: dataLoaderBuilderServiceAccountName, Namespace: dataset.Namespace},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, bldrSA, func() error {
		return r.authNServiceAccount(bldrSA)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	loaderSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: dataLoaderServiceAccountName, Namespace: dataset.Namespace},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, loaderSA, func() error {
		return r.authNServiceAccount(loaderSA)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	///////////////

	// Create a Job that will build a container image for the dataset fetcher
	buildJob, err := r.buildJob(ctx, &dataset)
	if err != nil {
		lg.Error(err, "unable to create builder Job")
		// No use in retrying...
		return ctrl.Result{}, nil
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(buildJob), buildJob); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, buildJob); client.IgnoreAlreadyExists(err) != nil {
				return ctrl.Result{}, fmt.Errorf("creating Job: %w", err)
			}
		} else {
			return ctrl.Result{}, fmt.Errorf("getting Job: %w", err)
		}
	}

	if buildJob.Status.Succeeded < 1 {
		lg.Info("Job has not succeeded yet")

		meta.SetStatusCondition(&dataset.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             ReasonBuilding,
			ObservedGeneration: dataset.Generation,
		})
		if err := r.Status().Update(ctx, &dataset); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating status with Ready=%v, Reason=%v: %w", metav1.ConditionFalse, ReasonBuilding, err)
		}

		// Allow Job watch to requeue.
		return ctrl.Result{}, nil
	}

	// Create a Job that will run the data loader image that was just built.
	loadJob, err := r.loadJob(ctx, &dataset)
	if err != nil {
		lg.Error(err, "unable to create load Job")
		// No use in retrying...
		return ctrl.Result{}, nil
	}

	if err := r.Create(ctx, loadJob); client.IgnoreAlreadyExists(err) != nil {
		return ctrl.Result{}, fmt.Errorf("creating Job: %w", err)
	}

	meta.SetStatusCondition(&dataset.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             ReasonLoading,
		ObservedGeneration: dataset.Generation,
	})
	if err := r.Status().Update(ctx, &dataset); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status with Ready=%v, Reason=%v: %w", metav1.ConditionFalse, ReasonBuilding, err)
	}

	if err := r.Get(ctx, client.ObjectKeyFromObject(loadJob), loadJob); err != nil {
		return ctrl.Result{}, fmt.Errorf("geting Job: %w", err)
	}
	if loadJob.Status.Succeeded < 1 {
		lg.Info("Job has not succeeded yet")

		// Allow Job watch to requeue.
		return ctrl.Result{}, nil
	}

	meta.SetStatusCondition(&dataset.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonLoaded,
		ObservedGeneration: dataset.Generation,
	})

	if err := r.Status().Update(ctx, &dataset); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status with Ready=%v, Reason=%v: %w", metav1.ConditionTrue, ReasonLoaded, err)
	}
	///////////////

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DatasetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Dataset{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

const (
	dataLoaderServiceAccountName        = "data-loader"
	dataLoaderBuilderServiceAccountName = "data-loader-builder"
)

func (r *DatasetReconciler) authNServiceAccount(sa *corev1.ServiceAccount) error {
	if sa.Annotations == nil {
		sa.Annotations = make(map[string]string)
	}
	switch typ := r.CloudContext.CloudType; typ {
	case CloudTypeGCP:
		sa.Annotations["iam.gke.io/gcp-service-account"] = fmt.Sprintf("substratus-%s@%s.iam.gserviceaccount.com", sa.GetName(), r.CloudContext.GCP.ProjectID)
	default:
		return fmt.Errorf("unsupported cloud type: %q", r.CloudContext.CloudType)
	}
	return nil
}

func (r *DatasetReconciler) buildJob(ctx context.Context, dataset *apiv1.Dataset) (*batchv1.Job, error) {

	var job *batchv1.Job

	annotations := map[string]string{}

	buildArgs := []string{
		"--dockerfile=Dockerfile",
		"--destination=" + r.loaderImage(dataset),
		// Cache will default to the image registry.
		"--cache=true",
		// Disable compressed caching to decrease memory usage.
		// (See https://github.com/GoogleContainerTools/kaniko/blob/main/README.md#flag---compressed-caching)
		"--compressed-caching=false",
	}

	var initContainers []corev1.Container
	var volumeMounts []corev1.VolumeMount
	var volumes []corev1.Volume

	const dockerfileWithTini = `
# Add Tini
ENV TINI_VERSION v0.19.0
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini /tini
RUN chmod +x /tini
ENTRYPOINT ["/tini", "--"]
`
	cloneArgs := []string{
		"clone",
		dataset.Spec.Source.Git.URL,
	}
	if dataset.Spec.Source.Git.Branch != "" {
		cloneArgs = append(cloneArgs, "--branch", dataset.Spec.Source.Git.Branch)
	}
	cloneArgs = append(cloneArgs, "/workspace")

	if dataset.Spec.Source.Git.Path != "" {
		buildArgs = append(buildArgs, "--context-sub-path="+dataset.Spec.Source.Git.Path)
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

	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataset.Name + "-data-loader-builder",
			// Cross-Namespace owners not allowed, must be same as dataset:
			Namespace: dataset.Namespace,
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
					ServiceAccountName: dataLoaderBuilderServiceAccountName,
					Containers: []corev1.Container{{
						Name:         "loader-builder",
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

	if err := controllerutil.SetControllerReference(dataset, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	return job, nil
}

func (r *DatasetReconciler) loadJob(ctx context.Context, dataset *apiv1.Dataset) (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataset.Name + "-data-loader",
			// Cross-Namespace owners not allowed, must be same as dataset:
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
					ServiceAccountName: dataLoaderServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "loader",
							Image: r.loaderImage(dataset),
							Args:  []string{"load.sh"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/data",
									SubPath:   string(dataset.UID) + "/data",
								},
								{
									Name:      "data",
									MountPath: "/dataset/logs",
									SubPath:   string(dataset.UID) + "/logs",
								},
							},
						},
					},
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
			"/" + string(dataset.UID) + "/data/" + dataset.Spec.Source.Filename
	}

	if err := controllerutil.SetControllerReference(dataset, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	return job, nil
}

func (r *DatasetReconciler) loaderImage(d *apiv1.Dataset) string {
	switch typ := r.CloudContext.CloudType; typ {
	case CloudTypeGCP:
		// Assuming this is Google Artifact Registry named "substratus".
		return fmt.Sprintf("%s-docker.pkg.dev/%s/substratus/dataset-%s-%s", r.CloudContext.GCP.Region(), r.CloudContext.GCP.ProjectID, d.Namespace, d.Name)
	default:
		panic("unsupported cloud type: " + typ)
	}
}
