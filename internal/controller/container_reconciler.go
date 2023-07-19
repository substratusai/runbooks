package controller

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	apiv1 "github.com/substratusai/substratus/api/v1"
	ssv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/resources"
	"github.com/substratusai/substratus/internal/sci"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ptr "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ContainerizedObject interface {
	object
	GetImage() *apiv1.Image
}

//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// ContainerImageReconciler builds container images. It is intended to be called from other top-level reconcilers.
type ContainerImageReconciler struct {
	Scheme *runtime.Scheme
	Client client.Client

	CloudContext           *cloud.Context
	CloudManagerGrpcClient sci.ControllerClient
	Kind                   string
}

func (r *ContainerImageReconciler) ReconcileContainerImage(ctx context.Context, obj ContainerizedObject) (result, error) {
	log := log.FromContext(ctx)

	if obj.GetImage().Name != "" {
		return result{success: true}, nil
	}

	log.Info("Reconciling container")
	defer log.Info("Done reconciling container")

	// Service account used for building and pushing the image.
	if result, err := reconcileCloudServiceAccount(ctx, r.CloudContext, r.Client, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      containerBuilderServiceAccountName,
			Namespace: obj.GetNamespace(),
		},
	}); !result.success {
		return result, err
	}

	var buildJob *batchv1.Job

	if specUpload := obj.GetImage().Upload; specUpload != nil && specUpload.Md5Checksum != "" {
		statusMd5, statusUploadURL := obj.GetStatusImage().Md5Checksum, obj.GetStatusImage().UploadURL

		// an upload object md5 has been declared and doesn't match the current spec
		// generate a signed URL
		if specUpload.Md5Checksum != statusMd5 {
			url, err := r.callSignedUrlGenerator(obj, r.CloudManagerGrpcClient)
			if err != nil {
				return result{}, fmt.Errorf("generating upload url: %w", err)
			}

			obj.SetStatusImage(ssv1.ImageStatus{
				UploadURL:   url,
				Md5Checksum: specUpload.Md5Checksum,
			})
			meta.SetStatusCondition(obj.GetConditions(), metav1.Condition{
				Type:               apiv1.ConditionUploaded,
				Status:             metav1.ConditionFalse,
				Reason:             apiv1.ReasonUploadIncomplete,
				ObservedGeneration: obj.GetGeneration(),
				Message:            fmt.Sprintf("Waiting for object upload to complete: %v", obj.GetName()),
			})
			if err := r.Client.Status().Update(ctx, obj); err != nil {
				return result{}, fmt.Errorf("updating status: %w", err)
			}
			return result{}, nil
		}

		// if the upload URL has expired, clear it from the status leaving the md5 checksum
		if statusUploadURL != "" {
			expirationTime, err := r.getExpirationTime(statusUploadURL)
			if err != nil {
				return result{}, fmt.Errorf("getting URL expiration time: %w", err)
			}

			if time.Now().After(expirationTime) {
				log.Info("The signed URL has expired. Clearing .Status.ImageURL")
				// TODO(bjb): why doesn't this work?
				obj.SetStatusImage(ssv1.ImageStatus{
					UploadURL:   "",
					Md5Checksum: statusMd5,
				})
				return result{}, nil
			}
		}

		// verify the object has been uploaded to storage
		storageMd5, err := r.storageObjectMd5(obj, r.CloudManagerGrpcClient)
		if err != nil {
			return result{}, fmt.Errorf("getting storage object md5: %w", err)
		}

		// verify the object's md5 matches the spec md5
		if storageMd5 != specUpload.Md5Checksum {
			log.Info("The object's md5 does not match the spec md5. An upload may be in progress.")
			return result{}, nil
		}

		meta.SetStatusCondition(obj.GetConditions(), metav1.Condition{
			Type:               apiv1.ConditionUploaded,
			Status:             metav1.ConditionTrue,
			Reason:             apiv1.ReasonUploadComplete,
			ObservedGeneration: obj.GetGeneration(),
			Message:            fmt.Sprintf("Object upload is complete: %v", obj.GetName()),
		})

		// create the build job pointing to the storage location
		buildJob, err = r.storageBuildJob(ctx, obj)
		if err != nil {
			log.Error(err, "unable to construct storage image-builder Job")
			// No use in retrying...
			return result{}, nil
		}
	}

	if obj.GetImage().Git != nil {
		var err error
		buildJob, err = r.gitBuildJob(ctx, obj)
		if err != nil {
			log.Error(err, "unable to construct git image-builder Job")
			// No use in retrying...
			return result{}, nil
		}
	}

	if buildJob.Name == "" {
		err := errors.New("no build job was created")
		log.Error(err, "no build job was created")
		return result{}, nil
	}

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(buildJob), buildJob); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Client.Create(ctx, buildJob); client.IgnoreAlreadyExists(err) != nil {
				return result{}, fmt.Errorf("creating builder Job: %w", err)
			}
		} else {
			return result{}, fmt.Errorf("getting builder Job: %w", err)
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
			return result{}, fmt.Errorf("updating status: %w", err)
		}

		// Allow Job watch to requeue.
		return result{}, nil
	}

	container := obj.GetImage()
	container.Name = r.imageName(obj)
	if err := r.Client.Update(ctx, obj); err != nil {
		return result{}, fmt.Errorf("updating container image: %w", err)
	}

	meta.SetStatusCondition(obj.GetConditions(), metav1.Condition{
		Type:               apiv1.ConditionBuilt,
		Status:             metav1.ConditionTrue,
		Reason:             apiv1.ReasonJobComplete,
		ObservedGeneration: obj.GetGeneration(),
		Message:            fmt.Sprintf("Builder Job completed: %v", buildJob.Name),
	})
	if err := r.Client.Status().Update(ctx, obj); err != nil {
		return result{}, fmt.Errorf("updating status: %w", err)
	}

	return result{success: true}, nil
}

func tiniInitContainer() corev1.Container {
	const dockerfileWithTini = `
# Add Tini
ENV TINI_VERSION v0.19.0
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini /tini
RUN chmod +x /tini
ENTRYPOINT ["/tini", "--"]
`
	return corev1.Container{
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
	}
}

func (r *ContainerImageReconciler) gitBuildJob(ctx context.Context, obj ContainerizedObject) (*batchv1.Job, error) {
	var job *batchv1.Job
	git := obj.GetImage().Git

	annotations := map[string]string{}

	buildArgs := []string{
		"--dockerfile=Dockerfile",
		"--destination=" + r.imageName(obj),
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
	if git.Branch != "" {
		cloneArgs = append(cloneArgs, "--branch", git.Branch)
	}
	cloneArgs = append(cloneArgs, "/workspace")

	if git.Path != "" {
		buildArgs = append(buildArgs, "--context-sub-path="+git.Path)
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
		tiniInitContainer(),
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
			Name: obj.GetName() + "-" + strings.ToLower(r.Kind) + "-container-builder",
			// NOTE: Cross-Namespace owners not allowed, must be same as obj.
			Namespace: obj.GetNamespace(),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.Int32(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					InitContainers: initContainers,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  ptr.Int64(0),
						RunAsGroup: ptr.Int64(0),
						FSGroup:    ptr.Int64(3003),
					},
					ServiceAccountName: containerBuilderServiceAccountName,
					Containers: []corev1.Container{{
						Name:         builderContainerName,
						Image:        "gcr.io/kaniko-project/executor:latest",
						Args:         buildArgs,
						VolumeMounts: volumeMounts,
						Resources:    resources.ContainerBuilderResources(),
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

func (r *ContainerImageReconciler) storageBuildJob(ctx context.Context, obj ContainerizedObject) (*batchv1.Job, error) {
	var job *batchv1.Job

	annotations := map[string]string{}

	buildArgs := []string{
		"--context=gs://" + r.bucketName() + "/" + r.signedUrlObjectName(obj),
		// NOTE: the dockerfile must be at the root of the tarball for this to work
		"--dockerfile=/Dockerfile",
		"--destination=" + r.imageName(obj),
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

	// Add an init container that will clone the Git repo and
	// another that will append tini to the Dockerfile.
	initContainers = append(initContainers, tiniInitContainer())

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
			// TODO(any): Ensure this name does not exceed the name character limit.
			Name: obj.GetName() + "-" + strings.ToLower(r.Kind) + "-container-builder",
			// NOTE: Cross-Namespace owners not allowed, must be same as obj.
			Namespace: obj.GetNamespace(),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.Int32(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					InitContainers: initContainers,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  ptr.Int64(0),
						RunAsGroup: ptr.Int64(0),
						FSGroup:    ptr.Int64(3003),
					},
					ServiceAccountName: containerBuilderServiceAccountName,
					Containers: []corev1.Container{{
						Name:         builderContainerName,
						Image:        "gcr.io/kaniko-project/executor:latest",
						Args:         buildArgs,
						VolumeMounts: volumeMounts,
						Resources:    resources.ContainerBuilderResources(),
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

func (r *ContainerImageReconciler) imageName(obj ContainerizedObject) string {
	switch name := r.CloudContext.Name; name {
	case cloud.GCP:
		// Assuming this is Google Artifact Registry named "substratus".
		return fmt.Sprintf("%s-docker.pkg.dev/%s/substratus/%s-%s-%s",
			r.CloudContext.GCP.Region(),
			r.CloudContext.GCP.ProjectID,
			strings.ToLower(r.Kind),
			obj.GetNamespace(),
			obj.GetName(),
		)
	default:
		panic("unsupported cloud: " + name)
	}
}

func (r *ContainerImageReconciler) signedUrlObjectName(obj ContainerizedObject) string {
	return fmt.Sprintf("uploads/%s/%s/%s-%s-%s.tar.gz",
		r.CloudContext.GCP.Region(),
		r.CloudContext.GCP.ProjectID,
		strings.ToLower(r.Kind),
		obj.GetNamespace(),
		obj.GetName(),
	)
}

func (r *ContainerImageReconciler) bucketName() string {
	return r.CloudContext.GCP.ProjectID + "-substratus-" + strings.ToLower(r.Kind) + "s"
}

func (r *ContainerImageReconciler) storageObjectMd5(obj ContainerizedObject, c sci.ControllerClient) (string, error) {
	req := &sci.GetObjectMd5Request{
		BucketName: r.bucketName(),
		ObjectName: r.signedUrlObjectName(obj),
	}

	resp, err := c.GetObjectMd5(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("calling the sci service to GetObjectMd5: %w", err)
	}

	return resp.Md5Checksum, nil
}

func (r *ContainerImageReconciler) callSignedUrlGenerator(obj ContainerizedObject, c sci.ControllerClient) (string, error) {
	req := &sci.CreateSignedURLRequest{
		BucketName:        r.bucketName(),
		ObjectName:        r.signedUrlObjectName(obj),
		ExpirationSeconds: 300,
		Md5Checksum:       obj.GetImage().Upload.Md5Checksum,
	}

	resp, err := c.CreateSignedURL(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("calling the sci service to CreateSignedURL: %w", err)
	}

	return resp.Url, nil
}

func (r *ContainerImageReconciler) getExpirationTime(signedUrl string) (time.Time, error) {
	u, err := url.Parse(signedUrl)
	if err != nil {
		return time.Time{}, err
	}

	queryParams := u.Query()
	date := queryParams.Get("X-Goog-Date")
	expires, err := strconv.Atoi(queryParams.Get("X-Goog-Expires"))
	if err != nil {
		return time.Time{}, err
	}

	t, err := time.Parse("20060102T150405Z", date)
	if err != nil {
		return time.Time{}, err
	}
	expirationTime := t.Add(time.Duration(expires) * time.Second)
	return expirationTime, nil
}
