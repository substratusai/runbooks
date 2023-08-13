package controller

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/resources"
	"github.com/substratusai/substratus/internal/sci"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const latestUploadPath = "uploads/latest.tar.gz"

type BuildableObject interface {
	client.Object

	GetBuild() *apiv1.Build
	GetImage() string
	SetImage(string)

	GetConditions() *[]metav1.Condition
	SetStatusReady(bool)
	GetStatusUpload() apiv1.UploadStatus
	SetStatusUpload(apiv1.UploadStatus)
}

//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// BuildReconciler builds container images.
type BuildReconciler struct {
	Scheme *runtime.Scheme
	Client client.Client

	Kind      string
	NewObject func() BuildableObject

	Cloud cloud.Cloud
	SCI   sci.ControllerClient
}

func (r *BuildReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	obj := r.NewObject()
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if obj.GetBuild() == nil {
		return ctrl.Result{}, nil
	}
	if obj.GetImage() == r.Cloud.ObjectBuiltImageURL(obj) {
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling build")
	defer log.Info("Done reconciling build")

	// Service account used for building and pushing the image.
	if result, err := reconcileServiceAccount(ctx, r.Cloud, r.SCI, r.Client, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      containerBuilderServiceAccountName,
			Namespace: obj.GetNamespace(),
		},
	}); !result.success {
		return result.Result, err
	}

	var buildJob *batchv1.Job

	if obj.GetBuild().Upload != nil {
		if result, err := r.reconcileUploadFile(ctx, obj); !result.success {
			return result.Result, err
		}

		var err error
		buildJob, err = r.storageBuildJob(ctx, obj)
		if err != nil {
			log.Error(err, "unable to construct storage image-builder Job")
			// No use in retrying...
			return ctrl.Result{}, nil
		}
	} else if obj.GetBuild().Git != nil {
		var err error
		buildJob, err = r.gitBuildJob(ctx, obj)
		if err != nil {
			log.Error(err, "unable to construct git image-builder Job")
			// No use in retrying...
			return ctrl.Result{}, nil
		}
	}

	if buildJob.Name == "" {
		err := errors.New("no build job was created")
		log.Error(err, "no build job was created")
		return ctrl.Result{}, nil
	}

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(buildJob), buildJob); err != nil {
		if apierrors.IsNotFound(err) {
			// No Job exists, create one.
			if err := r.Client.Create(ctx, buildJob); client.IgnoreAlreadyExists(err) != nil {
				return ctrl.Result{}, fmt.Errorf("creating builder Job: %w", err)
			}
		} else {
			return ctrl.Result{}, fmt.Errorf("getting builder Job: %w", err)
		}
	}

	if buildJob.Annotations["image"] != r.Cloud.ObjectBuiltImageURL(obj) {
		// Out of date, recreate.
		if err := r.Client.Delete(ctx, buildJob); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, fmt.Errorf("deleting out of date builder Job: %w", err)
		}
		if err := r.Client.Create(ctx, buildJob); err != nil {
			return ctrl.Result{}, fmt.Errorf("creating builder Job: %w", err)
		}
	}

	if buildJob.Status.Succeeded < 1 {
		log.Info("The builder Job has not succeeded yet")

		obj.SetStatusReady(false)
		meta.SetStatusCondition(obj.GetConditions(), metav1.Condition{
			Type:               apiv1.ConditionBuilt,
			Status:             metav1.ConditionFalse,
			Reason:             apiv1.ReasonJobNotComplete,
			ObservedGeneration: obj.GetGeneration(),
			Message:            fmt.Sprintf("Waiting for builder Job to complete: %v", buildJob.Name),
		})
		if err := r.Client.Status().Update(ctx, obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
		}

		// Allow Job watch to requeue.
		return ctrl.Result{}, nil
	}

	obj.SetImage(r.Cloud.ObjectBuiltImageURL(obj))
	if err := r.Client.Update(ctx, obj); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating container image: %w", err)
	}

	meta.SetStatusCondition(obj.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionBuilt,
		Status:             metav1.ConditionTrue,
		Reason:             apiv1.ReasonJobComplete,
		ObservedGeneration: obj.GetGeneration(),
		Message:            fmt.Sprintf("Builder Job completed: %v", buildJob.Name),
	})
	if err := r.Client.Status().Update(ctx, obj); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *BuildReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(r.NewObject()).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *BuildReconciler) reconcileUploadFile(ctx context.Context, obj BuildableObject) (result, error) {
	log := log.FromContext(ctx)

	spec := obj.GetBuild().Upload
	status := obj.GetStatusUpload()

	if spec.RequestID != status.RequestID {
		// Account for the edge-case where an uploaded file matching the checksum
		// already exists in storage.
		// For example: This can happen if a Notebook is deleted and recreated
		// but the underlying storage was not cleared.
		existingUploadChecksum, _ := r.storageObjectMd5(obj, r.SCI)
		if existingUploadChecksum == spec.MD5Checksum {
			obj.SetStatusUpload(apiv1.UploadStatus{
				StoredMD5Checksum: spec.MD5Checksum,
			})
			meta.SetStatusCondition(obj.GetConditions(), metav1.Condition{
				Type:               apiv1.ConditionUploaded,
				Status:             metav1.ConditionTrue,
				Reason:             apiv1.ReasonUploadFound,
				ObservedGeneration: obj.GetGeneration(),
				Message:            fmt.Sprintf("Existing upload found in storage with specified checksum: %s", spec.MD5Checksum),
			})
			if err := r.Client.Status().Update(ctx, obj); err != nil {
				return result{}, fmt.Errorf("updating status: %w", err)
			}
			return result{success: true}, nil
		}

		url, expiration, err := r.generateSignedURL(obj)
		if err != nil {
			return result{}, fmt.Errorf("generating upload url: %w", err)
		}

		obj.SetStatusUpload(apiv1.UploadStatus{
			SignedURL:  url,
			RequestID:  spec.RequestID,
			Expiration: metav1.Time{Time: expiration},
		})
		meta.SetStatusCondition(obj.GetConditions(), metav1.Condition{
			Type:               apiv1.ConditionUploaded,
			Status:             metav1.ConditionFalse,
			Reason:             apiv1.ReasonAwaitingUpload,
			ObservedGeneration: obj.GetGeneration(),
			Message:            fmt.Sprintf("Waiting for upload with md5 checksum: %s", spec.MD5Checksum),
		})
		if err := r.Client.Status().Update(ctx, obj); err != nil {
			return result{}, fmt.Errorf("updating status: %w", err)
		}

		// Client is expected to trigger a change to the object after uploading
		// which will trigger this function again.
		return result{}, nil
	}

	// Verify the object has been uploaded to storage.
	storageMD5, err := r.storageObjectMd5(obj, r.SCI)
	if err != nil {
		return result{}, fmt.Errorf("getting storage object md5: %w", err)
	}
	if storageMD5 != spec.MD5Checksum {
		log.Info("The object's md5 does not match the spec md5. An upload may be in progress.")
		// Allow the client to trigger a retry (they can update an annotation).
		return result{}, nil
	}

	obj.SetStatusUpload(apiv1.UploadStatus{
		SignedURL:         "",
		RequestID:         spec.RequestID,
		Expiration:        metav1.Time{},
		StoredMD5Checksum: storageMD5,
	})
	meta.SetStatusCondition(obj.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionUploaded,
		Status:             metav1.ConditionTrue,
		Reason:             apiv1.ReasonUploadFound,
		ObservedGeneration: obj.GetGeneration(),
		Message:            fmt.Sprintf("Upload received with matching md5 checksum: %s", spec.MD5Checksum),
	})
	if err := r.Client.Status().Update(ctx, obj); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	return result{success: true}, nil
}

func (r *BuildReconciler) gitBuildJob(ctx context.Context, obj BuildableObject) (*batchv1.Job, error) {
	var job *batchv1.Job
	git := obj.GetBuild().Git

	annotations := map[string]string{}

	image := r.Cloud.ObjectBuiltImageURL(obj)

	buildArgs := []string{
		"--context=dir:///workspace",
		"--destination=" + image,
		// Cache will default to the image registry.
		"--cache=true",
		// Disable compressed caching to decrease memory usage.
		// (See https://github.com/GoogleContainerTools/kaniko/blob/main/README.md#flag---compressed-caching)
		"--compressed-caching=false",
		"--log-format=text",
	}

	var initContainers []corev1.Container
	var volumeMounts []corev1.VolumeMount
	var volumes []corev1.Volume

	cloneArgs := []string{
		"clone",
		git.URL,
	}
	if git.Tag != "" {
		// NOTE: --branch flag is used for tags too.
		cloneArgs = append(cloneArgs, "--branch", git.Tag)
	} else if git.Branch != "" {
		cloneArgs = append(cloneArgs, "--branch", git.Branch)
	}
	cloneArgs = append(cloneArgs, "/workspace")

	if git.Path != "" {
		buildArgs = append(buildArgs, "--context-sub-path="+git.Path)
	}

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

	const builderContainerName = "builder"
	annotations["kubectl.kubernetes.io/default-container"] = builderContainerName
	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			// TODO(nstogner): Ensure this name does not exceed the name character limit.
			Name: buildJobName(obj, r.Kind),
			// NOTE: Cross-Namespace owners not allowed, must be same as obj.
			Namespace: obj.GetNamespace(),
			Annotations: map[string]string{
				"image": image,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(1)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					InitContainers: initContainers,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  ptr.To(int64(0)),
						RunAsGroup: ptr.To(int64(0)),
						FSGroup:    ptr.To(int64(3003)),
					},
					ServiceAccountName: containerBuilderServiceAccountName,
					Containers: []corev1.Container{{
						Name:         builderContainerName,
						Image:        "gcr.io/kaniko-project/executor:latest",
						Args:         buildArgs,
						VolumeMounts: volumeMounts,
						Resources:    resources.ContainerBuilderResources(r.Cloud.Name()),
					}},
					RestartPolicy: "Never",
					Volumes:       volumes,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(obj, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	return job, nil
}

func (r *BuildReconciler) storageBuildJob(ctx context.Context, obj BuildableObject) (*batchv1.Job, error) {
	var job *batchv1.Job

	image := r.Cloud.ObjectBuiltImageURL(obj)

	podAnnotations := map[string]string{}
	buildArgs := []string{
		"--context=" + r.Cloud.ObjectArtifactURL(obj).String() + "/" + latestUploadPath,
		"--destination=" + image,
		// Cache will default to the image registry.
		"--cache=true",
		// Disable compressed caching to decrease memory usage.
		// (See https://github.com/GoogleContainerTools/kaniko/blob/main/README.md#flag---compressed-caching)
		"--compressed-caching=false",
		"--log-format=text",
	}

	var initContainers []corev1.Container
	var volumeMounts []corev1.VolumeMount
	var volumes []corev1.Volume

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

	// Dirty hack to support "tar://" URLs for Kaniko.
	// TODO(nstogner): Refactor this "cloud"-specific code. It does not
	// belong here.
	if r.Cloud.Name() == cloud.KindName {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "bucket",
			MountPath: "/bucket",
		})
		typ := corev1.HostPathDirectory
		volumes = append(volumes, corev1.Volume{
			Name: "bucket",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/bucket",
					Type: &typ,
				},
			},
		})
	}

	const builderContainerName = "builder"
	podAnnotations["kubectl.kubernetes.io/default-container"] = builderContainerName
	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: buildJobName(obj, r.Kind),
			// NOTE: Cross-Namespace owners not allowed, must be same as obj.
			Namespace: obj.GetNamespace(),
			Annotations: map[string]string{
				"image": image,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(1)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					InitContainers: initContainers,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  ptr.To(int64(0)),
						RunAsGroup: ptr.To(int64(0)),
						FSGroup:    ptr.To(int64(3003)),
					},
					ServiceAccountName: containerBuilderServiceAccountName,
					Containers: []corev1.Container{{
						Name:         builderContainerName,
						Image:        "gcr.io/kaniko-project/executor:latest",
						Args:         buildArgs,
						VolumeMounts: volumeMounts,
						Resources:    resources.ContainerBuilderResources(r.Cloud.Name()),
					}},
					RestartPolicy: "Never",
					Volumes:       volumes,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(obj, job, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}

	return job, nil
}

func (r *BuildReconciler) storageObjectMd5(obj BuildableObject, c sci.ControllerClient) (string, error) {
	u := r.Cloud.ObjectArtifactURL(obj)

	req := &sci.GetObjectMd5Request{
		BucketName: u.Bucket,
		ObjectName: filepath.Join(u.Path, latestUploadPath),
	}

	resp, err := c.GetObjectMd5(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("calling the sci service to GetObjectMd5: %w", err)
	}

	return resp.Md5Checksum, nil
}

func (r *BuildReconciler) generateSignedURL(obj BuildableObject) (string, time.Time, error) {
	u := r.Cloud.ObjectArtifactURL(obj)

	const expirationSeconds = 300
	// This expiration time will be conservative. It will be equal to or shorter than requested.
	// TODO: Grab the actual expiration time will be returned in the response
	// (not yet implemented in SCI).
	// NOTE: This could be parased from a GCS signed URL, but that would require
	// cloud-specific code and the SCI should abstract that.
	expirationTime := time.Now().Add(time.Duration(expirationSeconds) * time.Second)

	req := &sci.CreateSignedURLRequest{
		BucketName:        u.Bucket,
		ObjectName:        filepath.Join(u.Path, latestUploadPath),
		ExpirationSeconds: expirationSeconds,
		Md5Checksum:       obj.GetBuild().Upload.MD5Checksum,
	}
	resp, err := r.SCI.CreateSignedURL(context.Background(), req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("calling the sci service to CreateSignedURL: %w", err)
	}

	return resp.Url, expirationTime, nil
}

func buildJobName(obj client.Object, kind string) string {
	// NOTE: Suffix should be under 13 characters (for all Substratus kinds)
	// to avoid exceeding the name character limit.
	return obj.GetName() + "-" + strings.ToLower(kind) + "-bld"
}
