package resources

import (
	"fmt"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GPUInfo struct {
	Memory       int64
	ResourceName corev1.ResourceName
	NodeSelector map[string]string
}

var cloudGPUs = map[cloud.Name]map[apiv1.GPUType]*GPUInfo{
	cloud.GCP: {
		// https://cloud.google.com/compute/docs/gpus#nvidia_t4_gpus
		apiv1.GPUTypeNvidiaTeslaT4: {
			Memory:       16 * gigabyte,
			ResourceName: corev1.ResourceName("nvidia.com/gpu"),
			NodeSelector: map[string]string{
				"cloud.google.com/gke-accelerator": "nvidia-tesla-t4",
			},
		},
		// https://cloud.google.com/compute/docs/gpus#l4-gpus
		apiv1.GPUTypeNvidiaL4: {
			Memory:       24 * gigabyte,
			ResourceName: corev1.ResourceName("nvidia.com/gpu"),
			NodeSelector: map[string]string{
				"cloud.google.com/gke-accelerator": "nvidia-l4",
			},
		},
	},
}

func ApplyGPU(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec, containerName string, cloudName cloud.Name, res *apiv1.GPUResources) error {
	gpuInfo, ok := cloudGPUs[cloudName][res.Type]
	if !ok {
		return fmt.Errorf("GPU %s is not supported on cloud %s", res.Type, cloudName)
	}

	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}
	resources.Requests[gpuInfo.ResourceName] = *resource.NewQuantity(res.Count, resource.DecimalSI)
	resources.Limits[gpuInfo.ResourceName] = *resource.NewQuantity(res.Count, resource.DecimalSI)

	if podSpec.NodeSelector == nil {
		podSpec.NodeSelector = map[string]string{}
	}

	// TODO: Make spot configurable.
	podSpec.NodeSelector["cloud.google.com/gke-spot"] = "true"
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

	if !setContainerResources(containerName, podSpec, resources) {
		return fmt.Errorf("container %s not found in pod", containerName)
	}

	return nil
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
