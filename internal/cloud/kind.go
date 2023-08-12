package cloud

import (
	"context"
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const KindName = "kind"

type Kind struct {
	Common
}

func (k *Kind) Name() string { return KindName }

func (k *Kind) AutoConfigure(ctx context.Context) error {
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
				Path: filepath.Join("/bucket", bktURL.Path),
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

func (k *Kind) AssociateServiceAccount(sa *corev1.ServiceAccount) {
}
