package resources

import (
	"fmt"

	apiv1 "github.com/substratusai/substratus/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func ApplyCPU(podSpec *corev1.PodSpec, containerName string, res *apiv1.CPUResources) error {
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}
	resources.Requests[corev1.ResourceCPU] = *resource.NewQuantity(res.Count, resource.DecimalSI)
	resources.Requests[corev1.ResourceMemory] = *resource.NewQuantity(res.Memory*gigabyte, resource.BinarySI)

	if !setContainerResources(containerName, podSpec, resources) {
		return fmt.Errorf("container %s not found in pod", containerName)
	}

	return nil
}
