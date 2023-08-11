package cloud

import (
	"context"

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
	// TODO: HostPath mount.
	return nil
}

func (k *Kind) AssociateServiceAccount(sa *corev1.ServiceAccount) {
}
