package resources

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func ContainerBuilderResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(12*gigabyte, resource.BinarySI),
			// TODO(nstogner): Should ephemeral storage requests account for whether
			// we are building a Model? and how large that model is?
			corev1.ResourceEphemeralStorage: *resource.NewQuantity(100*gigabyte, resource.BinarySI),
		},
	}
}
