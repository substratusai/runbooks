package controller

import (
	"fmt"
	"math"

	apiv1 "github.com/substratusai/substratus/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Runtime string

const (
	RuntimeTrainer    = "trainer"
	RuntimeServer     = "server"
	RuntimeNotebook   = "notebook"
	RuntimeBuilder    = "builder"
	RuntimeDataPuller = "data-puller"
)

type GPUType string

const (
	GPUTypeNone          = GPUType("none")
	GPUTypeNvidiaTeslaT4 = GPUType("nvidia-tesla-t4")
	GPUTypeNvidiaL4      = GPUType("nvidia-l4")
)

func ToGPUType(s string) (GPUType, error) {
	switch typ := GPUType(s); typ {
	case GPUTypeNone, GPUTypeNvidiaTeslaT4, GPUTypeNvidiaL4:
		return typ, nil
	default:
		return "", fmt.Errorf("unknown GPU type: %q", s)
	}
}

type GPUInfo struct {
	Memory       int64
	ResourceName corev1.ResourceName
	NodeSelector map[string]string
}

type CloudType string

const (
	CloudTypeGCP = CloudType("gcp")
)

var gpus = map[CloudType]map[GPUType]GPUInfo{
	CloudTypeGCP: {
		// https://cloud.google.com/compute/docs/gpus#nvidia_t4_gpus
		GPUTypeNvidiaTeslaT4: {
			Memory:       16 * gigabyte,
			ResourceName: corev1.ResourceName("nvidia.com/gpu"),
			NodeSelector: map[string]string{
				"cloud.google.com/gke-accelerator": "nvidia-tesla-t4",
			},
		},
		// https://cloud.google.com/compute/docs/gpus#l4-gpus
		GPUTypeNvidiaL4: {
			Memory:       24 * gigabyte,
			ResourceName: corev1.ResourceName("nvidia.com/gpu"),
			NodeSelector: map[string]string{
				"cloud.google.com/gke-accelerator": "nvidia-l4",
			},
		},
	},
}

func setRuntimeResources(model *apiv1.Model, spec *corev1.PodSpec, gpuType GPUType, runtime Runtime) error {
	// TODO: On failure, keep track of previous values and add more resources
	// base on OOM, etc.

	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}

	// Approximate the model size in memory.
	modelBytes := model.Spec.Size.ParameterCount * int64(model.Spec.Size.ParameterBits) / 8

	// TODO: Determine GPU from configured profiles / some sort of quota query.
	// NOTE: L4 does not appear to be supported by GKE NAP yet. So nodepools have
	//       to be precreated in order to use L4
	gpu := gpus[CloudTypeGCP][gpuType]

	var gpuMemory, gpuCount int64
	if gpuType != GPUTypeNone {
		// Use a 10% threshold.
		gpuMemory = int64(1.1 * float64(modelBytes))

		gpuCount = int64(math.Ceil(float64(gpuMemory) / float64(gpu.Memory)))
		// TODO: Limit the max size of a model by validating the Model API until
		// distributed training is supported.
		gpuCount = nextPowOf2(gpuCount)
	}

	// Use a 10% threshold plus a fixed size amount for runtime memory.
	ramMemory := int64(1.1*float64(modelBytes)) + 2*gigabyte

	switch runtime {
	case RuntimeNotebook, RuntimeTrainer, RuntimeServer:
		cpuCount := int64(3)
		if gpuType != GPUTypeNone {
			cpuCount = 2 * gpuCount
		}
		// Set requests for CPU and Memory, but don't limit.
		resources.Requests[corev1.ResourceCPU] = *resource.NewQuantity(cpuCount, resource.DecimalSI)
		resources.Requests[corev1.ResourceMemory] = *resource.NewQuantity(roundUpGB(ramMemory), resource.BinarySI)

		// GPU
		if gpuType != GPUTypeNone {
			resources.Requests[gpu.ResourceName] = *resource.NewQuantity(gpuCount, resource.DecimalSI)
			resources.Limits[gpu.ResourceName] = *resource.NewQuantity(gpuCount, resource.DecimalSI)
			if spec.NodeSelector == nil {
				spec.NodeSelector = map[string]string{}
			}

			// TODO make spot VM configurable
			spec.NodeSelector["cloud.google.com/gke-spot"] = "true"
			// Toleration is needed to trigger NAP
			// https://cloud.google.com/kubernetes-engine/docs/how-to/node-auto-provisioning#support_for_spot_vms
			spec.Tolerations = append(spec.Tolerations, corev1.Toleration{
				Key:      "cloud.google.com/gke-spot",
				Operator: corev1.TolerationOpEqual,
				Value:    "true",
				Effect:   corev1.TaintEffectNoSchedule,
			})

			for k, v := range gpu.NodeSelector {
				spec.NodeSelector[k] = v
			}
		}

	case RuntimeBuilder:
		// Container build requires very little CPU.
		resources.Requests[corev1.ResourceCPU] = *resource.NewQuantity(2, resource.DecimalSI)
		resources.Requests[corev1.ResourceMemory] = *resource.NewQuantity(12*gigabyte, resource.BinarySI)

	default:
		return fmt.Errorf("unknown runtime: %s", runtime)
	}

	var ephStorage int64
	switch runtime {
	case RuntimeNotebook:
		// Model is already stored in the container.
		// Ephemeral storage is just needed for what the user downloads.
		ephStorage = 100 * gigabyte
	case RuntimeTrainer:
		// Model artifacts are stored using volumes outside of container.
		ephStorage = 100 * gigabyte
	case RuntimeBuilder:
		// Use 2x the model size because kaniko takes snapshots
		// Add a fixed cushion.
		ephStorage = int64(2*float64(modelBytes)) + 100*gigabyte
	case RuntimeServer:
		// Model is already stored in the container. Server should need minimal ephemeral storage.
		ephStorage = 100 * gigabyte
	}
	resources.Requests[corev1.ResourceEphemeralStorage] = *resource.NewQuantity(roundUpGB(ephStorage), resource.BinarySI)

	setRes := func(i int, containers []corev1.Container) {
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

	for i, container := range spec.InitContainers {
		if container.Name == string(runtime) {
			setRes(i, spec.InitContainers)
			// json.NewEncoder(os.Stdout).Encode(spec)
			return nil

		}
	}
	for i, container := range spec.Containers {
		if container.Name == string(runtime) {
			setRes(i, spec.Containers)
			// json.NewEncoder(os.Stdout).Encode(spec)
			return nil
		}
	}

	return fmt.Errorf("container name not found: %s", runtime)
}
