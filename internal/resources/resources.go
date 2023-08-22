package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/substratusai/substratus/api/v1"
)

func Apply(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec, containerName string, cloudName string, res *apiv1.Resources) error {
	// TODO: Auto-determine resources if nil.
	if res == nil {
		// TODO(nstogner): Cloud-specific conditional should go away...
		// Most likely this stuff will all go into a ConfigMap that contains cloud-specific
		// information.
		if cloudName == "kind" {
			res = &apiv1.Resources{}
		} else {
			res = &apiv1.Resources{
				CPU:    2,
				Memory: 4,
			}
		}
	}

	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}

	resources.Requests[corev1.ResourceCPU] = *resource.NewQuantity(res.CPU, resource.DecimalSI)
	resources.Requests[corev1.ResourceMemory] = *resource.NewQuantity(res.Memory*gigabyte, resource.BinarySI)

	if res.GPU != nil {
		gpuInfo, ok := cloudGPUs[cloudName][res.GPU.Type]
		if !ok {
			return fmt.Errorf("GPU %s is not supported on cloud %s", res.GPU.Type, cloudName)
		}

		resources.Requests[gpuInfo.ResourceName] = *resource.NewQuantity(res.GPU.Count, resource.DecimalSI)
		resources.Limits[gpuInfo.ResourceName] = *resource.NewQuantity(res.GPU.Count, resource.DecimalSI)

		if podSpec.NodeSelector == nil {
			podSpec.NodeSelector = map[string]string{}
		}

		// TODO: Move this GCP code into cloud-specific configuration.
		// Toleration is needed to trigger NAP
		// https://cloud.google.com/kubernetes-engine/docs/how-to/node-auto-provisioning#support_for_spot_vms
		podSpec.Tolerations = append(podSpec.Tolerations, corev1.Toleration{
			Key:      "cloud.google.com/gke-spot",
			Operator: corev1.TolerationOpEqual,
			Value:    "true",
			Effect:   corev1.TaintEffectNoSchedule,
		})

		for k, v := range gpuInfo.NodeSelector {
			podSpec.NodeSelector[k] = v
		}
	}

	if !setContainerResources(containerName, podSpec, resources) {
		return fmt.Errorf("container %s not found in pod", containerName)
	}

	return nil
}

func ContainerBuilderResources(cloudName string) corev1.ResourceRequirements {
	// TODO(nstogner): Cloud-specific conditional should go away...
	// Most likely this stuff will all go into a ConfigMap that contains cloud-specific
	// information.
	if cloudName == "kind" {
		return corev1.ResourceRequirements{}
	}

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

func setContainerResources(containerName string, podSpec *corev1.PodSpec, resources corev1.ResourceRequirements) bool {
	for i, container := range podSpec.InitContainers {
		if container.Name == string(containerName) {
			setContainerResourcesByIndex(i, podSpec.InitContainers, resources)
			// json.NewEncoder(os.Stdout).Encode(spec)
			return true

		}
	}
	for i, container := range podSpec.Containers {
		if container.Name == string(containerName) {
			setContainerResourcesByIndex(i, podSpec.Containers, resources)
			// json.NewEncoder(os.Stdout).Encode(spec)
			return true
		}
	}
	return false
}

func setContainerResourcesByIndex(i int, containers []corev1.Container, resources corev1.ResourceRequirements) {
	if containers[i].Resources.Requests == nil {
		containers[i].Resources.Requests = corev1.ResourceList{}
	}
	if containers[i].Resources.Limits == nil {
		containers[i].Resources.Limits = corev1.ResourceList{}
	}
	for k, v := range resources.Requests {
		containers[i].Resources.Requests[k] = v
	}
	for k, v := range resources.Limits {
		containers[i].Resources.Limits[k] = v
	}
}
