package controller

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/resources"
	"github.com/substratusai/substratus/internal/sci"
	"google.golang.org/grpc"
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

	CloudContext         *cloud.Context
	CloudManagerGrpcConn *grpc.ClientConn
	Kind                 string
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

	if r.Kind == "Notebook" {
		nb := obj.(*apiv1.Notebook)

		// this is a signed url request for a file with a different md5 checksum
		if nb.Spec.Image.Upload.Md5Checksum != nb.Status.LastGeneratedMd5Checksum {
			url, err := r.callSignedUrlGenerator(nb, r.CloudManagerGrpcConn)
			if err != nil {
				return result{}, fmt.Errorf("generating upload url: %w", err)
			}

			nb.Status.UploadURL = url
			nb.Status.LastGeneratedMd5Checksum = nb.Spec.Image.Upload.Md5Checksum
			// TODO(bjb): shouldn't this happen in the notebook_controller?
			// The reconciler here doesn't have Status()
			// if err := r.Status().Update(ctx, &nb); err != nil {
			// 	return result{}, fmt.Errorf("updating notebook status: %w", err)
			// }
		}
		if nb.Status.UploadURL != "" {
			expirationTime, err := r.getExpirationTime(nb.Status.UploadURL)
			if err != nil {
				return result{}, fmt.Errorf("getting URL expiration time: %w", err)
			}

			if time.Now().After(expirationTime) {
				log.Info("The signed URL has expired. Clearing .Status.UploadURL")
				nb.Status.UploadURL = ""
				// TODO(bjb): another case where we need to update the notebook status. How to do it properly?
			}
		}

		// TODO(bjb): how should we signal that contents have been successfully uploaded and should be built?
	}

	// The Job that will build the container image.
	buildJob, err := r.buildJob(ctx, obj)
	if err != nil {
		log.Error(err, "unable to construct image-builder Job")
		// No use in retrying...
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

func (r *ContainerImageReconciler) buildJob(ctx context.Context, obj ContainerizedObject) (*batchv1.Job, error) {
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

	const dockerfileWithTini = `
# Add Tini
ENV TINI_VERSION v0.19.0
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini /tini
RUN chmod +x /tini
ENTRYPOINT ["/tini", "--"]
`
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

func (r *ContainerImageReconciler) imageName(obj ContainerizedObject) string {
	switch name := r.CloudContext.Name; name {
	case cloud.GCP:
		// Assuming this is Google Artifact Registry named "substratus".
		return fmt.Sprintf("%s-docker.pkg.dev/%s/substratus/%s-%s-%s", r.CloudContext.GCP.Region(), r.CloudContext.GCP.ProjectID, strings.ToLower(r.Kind), obj.GetNamespace(), obj.GetName())
	default:
		panic("unsupported cloud: " + name)
	}
}

func (r *ContainerImageReconciler) callSignedUrlGenerator(notebook *apiv1.Notebook, conn *grpc.ClientConn) (string, error) {

	// create the request object
	req := &sci.CreateSignedURLRequest{
		BucketName:        r.CloudContext.GCP.ProjectID + "-substratus-notebooks",
		ObjectName:        "notebook.zip",
		ExpirationSeconds: 300,
		Md5Checksum:       notebook.Spec.Image.Upload.Md5Checksum,
	}

	// Create a client using the connection
	c := sci.NewControllerClient(conn)

	// Call the service
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
