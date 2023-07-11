package resources

import (
	"fmt"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Apply(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec, containerName string, cloudName cloud.Name, res *apiv1.Resources) error {
	// TODO: Auto-determine resources if nil.
	if res == nil {
		res = &apiv1.Resources{
			CPU: &apiv1.CPUResources{
				Count:  2,
				Memory: 4,
			},
		}
	}

	if err := ApplyCPU(podSpec, containerName, res.CPU); err != nil {
		return fmt.Errorf("applying cpu resources: %w", err)
	}
	if res.GPU != nil {
		if err := ApplyGPU(podMetadata, podSpec, containerName, cloudName, res.GPU); err != nil {
			return fmt.Errorf("applying gpu resources: %w", err)
		}
	}

	return nil
}

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
