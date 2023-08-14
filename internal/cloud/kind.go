package cloud

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const KindName = "kind"

type Kind struct {
	// RegistryDiscoveryIP environment variable comes from the registry Service in the same namespace.
	// See: https://kubernetes.io/docs/concepts/services-networking/service/#environment-variables
	RegistryDiscoveryIP string `env:"REGISTRY_PORT_5000_TCP_ADDR" required:"true"`

	Common
}

func (k *Kind) Name() string { return KindName }

func (k *Kind) AutoConfigure(ctx context.Context) error {
	if k.ArtifactBucketURL == nil {
		// This is the base of the URL that Substratus objects will report
		// in their status.artifacts.url field. It references a host path
		// mount created in containers running on a Kind cluster.
		//
		// Translates to: "tar:///bucket"
		//
		// NOTE: kaniko interacts with this address and works because the
		// /bucket directory is mounted in the builder container.
		//
		// See: https://github.com/GoogleContainerTools/kaniko#kaniko-build-contexts
		//
		k.ArtifactBucketURL = &BucketURL{
			Scheme: "tar",
			Bucket: "",
			Path:   "/bucket",
		}
	}

	if k.RegistryURL == "" {
		k.RegistryURL = k.RegistryDiscoveryIP + ":5000"
	}

	return nil
}

func (k *Kind) MountBucket(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec, obj ArtifactObject, req MountBucketConfig) error {
	var bktURL *BucketURL
	if statusURL := obj.GetStatusArtifacts().URL; statusURL != "" {
		var err error
		bktURL, err = ParseBucketURL(statusURL)
		if err != nil {
			return fmt.Errorf("parsing status bucket url: %w", err)
		}
	} else {
		bktURL = k.ObjectArtifactURL(obj)
	}

	hostPathType := corev1.HostPathDirectoryOrCreate
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: req.Name,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: bktURL.Path,
				Type: &hostPathType,
			},
		},
	})

	for i := range podSpec.Containers {
		if podSpec.Containers[i].Name == req.Container {
			for _, mount := range req.Mounts {
				podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts,
					corev1.VolumeMount{
						Name:      req.Name,
						MountPath: "/content/" + mount.ContentSubdir,
						SubPath:   mount.BucketSubdir,
						ReadOnly:  req.ReadOnly,
					},
				)
			}
			return nil
		}
	}

	return fmt.Errorf("container not found: %s", req.Container)
}

func (k *Kind) AssociatePrincipal(*corev1.ServiceAccount) {}

func (k Kind) GetPrincipal(*corev1.ServiceAccount) (string, bool) { return "", true }
