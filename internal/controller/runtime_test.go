package controller

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func Test_setRuntimeResources(t *testing.T) {
	type expectation struct {
		spec     string
		metadata string
	}
	cases := []struct {
		name string

		gpuType GPUType

		model *apiv1.Model

		podMetadata metav1.ObjectMeta

		expected map[Runtime]expectation
	}{
		{
			name:    "125m-32bit-cpu",
			gpuType: GPUTypeNvidiaL4,
			model: &apiv1.Model{
				Spec: apiv1.ModelSpec{
					Size: apiv1.ModelSize{
						ParameterBits:  32,
						ParameterCount: 125 * million,
					},
					Compute: apiv1.ModelCompute{
						Types: []apiv1.ComputeType{apiv1.ComputeTypeCPU},
					},
				},
			},
			expected: map[Runtime]expectation{
				RuntimeTrainer: {spec: `
containers:
- name: trainer
  resources:
    requests:
      cpu: "3"
      ephemeral-storage: 100Gi
      memory: 3Gi
				`},
				RuntimeNotebook: {spec: `
containers:
- name: notebook
  resources:
    requests:
      cpu: "3"
      ephemeral-storage: 100Gi
      memory: 3Gi
				`},
				RuntimeServer: {spec: `
containers:
- name: server
  resources:
    requests:
      cpu: "3"
      ephemeral-storage: 100Gi
      memory: 3Gi
				`},
				RuntimeBuilder: {spec: `
containers:
- name: builder
  resources:
    requests:
      cpu: "2"
      ephemeral-storage: 101Gi
      memory: 12Gi
				`},
			},
		},
		{
			name:    "125m-32bit-gpu",
			gpuType: GPUTypeNvidiaL4,
			model: &apiv1.Model{
				Spec: apiv1.ModelSpec{
					Size: apiv1.ModelSize{
						ParameterBits:  32,
						ParameterCount: 125 * million,
					},
					Compute: apiv1.ModelCompute{
						Types: []apiv1.ComputeType{apiv1.ComputeTypeGPU},
					},
				},
			},
			expected: map[Runtime]expectation{
				RuntimeTrainer: {spec: `
containers:
- name: trainer
  resources:
    limits:
      nvidia.com/gpu: "2"
    requests:
      cpu: "2"
      ephemeral-storage: 100Gi
      memory: 3Gi
      nvidia.com/gpu: "2"
nodeSelector:
  cloud.google.com/gke-accelerator: nvidia-l4
  cloud.google.com/gke-spot: "true"
tolerations:
- effect: NoSchedule
  key: cloud.google.com/gke-spot
  operator: Equal
  value: "true"
				`},
				RuntimeNotebook: {spec: `
containers:
- name: notebook
  resources:
    limits:
      nvidia.com/gpu: "2"
    requests:
      cpu: "2"
      ephemeral-storage: 100Gi
      memory: 3Gi
      nvidia.com/gpu: "2"
nodeSelector:
  cloud.google.com/gke-accelerator: nvidia-l4
  cloud.google.com/gke-spot: "true"
tolerations:
- effect: NoSchedule
  key: cloud.google.com/gke-spot
  operator: Equal
  value: "true"
				`},
				RuntimeServer: {spec: `
containers:
- name: server
  resources:
    limits:
      nvidia.com/gpu: "1"
    requests:
      cpu: "2"
      ephemeral-storage: 100Gi
      memory: 3Gi
      nvidia.com/gpu: "1"
nodeSelector:
  cloud.google.com/gke-accelerator: nvidia-l4
  cloud.google.com/gke-spot: "true"
tolerations:
- effect: NoSchedule
  key: cloud.google.com/gke-spot
  operator: Equal
  value: "true"
				`},
				RuntimeBuilder: {spec: `
containers:
- name: builder
  resources:
    requests:
      cpu: "2"
      ephemeral-storage: 101Gi
      memory: 12Gi
				`},
			},
		},
		{
			name:    "7b-16bit-gpu",
			gpuType: GPUTypeNvidiaL4,
			model: &apiv1.Model{
				Spec: apiv1.ModelSpec{
					Size: apiv1.ModelSize{
						ParameterBits:  16,
						ParameterCount: 7 * billion,
					},
					Compute: apiv1.ModelCompute{
						Types: []apiv1.ComputeType{apiv1.ComputeTypeGPU},
					},
				},
			},
			expected: map[Runtime]expectation{
				RuntimeTrainer: {spec: `
containers:
- name: trainer
  resources:
    limits:
      nvidia.com/gpu: "2"
    requests:
      cpu: "2"
      ephemeral-storage: 100Gi
      memory: 17Gi
      nvidia.com/gpu: "2"
nodeSelector:
  cloud.google.com/gke-accelerator: nvidia-l4
  cloud.google.com/gke-spot: "true"
tolerations:
- effect: NoSchedule
  key: cloud.google.com/gke-spot
  operator: Equal
  value: "true"
				`},
				RuntimeNotebook: {spec: `
containers:
- name: notebook
  resources:
    limits:
      nvidia.com/gpu: "2"
    requests:
      cpu: "2"
      ephemeral-storage: 100Gi
      memory: 17Gi
      nvidia.com/gpu: "2"
nodeSelector:
  cloud.google.com/gke-accelerator: nvidia-l4
  cloud.google.com/gke-spot: "true"
tolerations:
- effect: NoSchedule
  key: cloud.google.com/gke-spot
  operator: Equal
  value: "true"
				`},
				RuntimeServer: {spec: `
containers:
- name: server
  resources:
    limits:
      nvidia.com/gpu: "1"
    requests:
      cpu: "2"
      ephemeral-storage: 100Gi
      memory: 17Gi
      nvidia.com/gpu: "1"
nodeSelector:
  cloud.google.com/gke-accelerator: nvidia-l4
  cloud.google.com/gke-spot: "true"
tolerations:
- effect: NoSchedule
  key: cloud.google.com/gke-spot
  operator: Equal
  value: "true"
				`},
				RuntimeBuilder: {spec: `
containers:
- name: builder
  resources:
    requests:
      cpu: "2"
      ephemeral-storage: 127Gi
      memory: 12Gi
				`},
			},
		},
		{
			name:    "125m-32bit-withfuse",
			gpuType: GPUTypeNvidiaL4,
			podMetadata: metav1.ObjectMeta{
				Annotations: map[string]string{
					"gke-gcsfuse/volumes": "true",
				},
			},
			model: &apiv1.Model{
				Spec: apiv1.ModelSpec{
					Size: apiv1.ModelSize{
						ParameterBits:  32,
						ParameterCount: 125 * million,
					},
					Compute: apiv1.ModelCompute{
						Types: []apiv1.ComputeType{apiv1.ComputeTypeCPU},
					},
				},
			},
			expected: map[Runtime]expectation{
				RuntimeBuilder: {
					spec: `
containers:
- name: builder
  resources:
    requests:
      cpu: "2"
      ephemeral-storage: 101Gi
      memory: 12Gi
				`,
					metadata: `
annotations:
  gke-gcsfuse/ephemeral-storage-limit: 101Gi
  gke-gcsfuse/volumes: "true"
creationTimestamp: null
				`,
				},
			},
		},
	}

	for _, c := range cases {
		t.Run("GPUType-"+string(c.gpuType)+"-Model-"+c.name, func(t *testing.T) {
			for runtime, expected := range c.expected {
				t.Run(string(runtime), func(t *testing.T) {
					mgr, err := NewRuntimeManager(c.gpuType)
					require.NoError(t, err)

					spec := testSpec(t, runtime)
					meta := c.podMetadata
					require.NoError(t, mgr.SetResources(c.model, &meta, spec, runtime))

					// Use YAML for comparison because it's easier to read
					// and makes generating expected output easier.
					actualSpecYAML, err := yaml.Marshal(spec)
					require.NoError(t, err)
					assert.Equal(t, strings.TrimSpace(expected.spec), strings.TrimSpace(string(actualSpecYAML)))

					if expected.metadata != "" {
						actualMetadataYAML, err := yaml.Marshal(meta)
						require.NoError(t, err)
						assert.Equal(t, strings.TrimSpace(expected.metadata), strings.TrimSpace(string(actualMetadataYAML)))
					}

					if t.Failed() {
						debugYAML, _ := yaml.Marshal(spec)
						fmt.Println("---outputted spec for debugging---")
						fmt.Println(string(debugYAML))
						fmt.Println("----------------------------------")
					}
				})
			}
		})
	}
}

func testSpec(t *testing.T, runtime Runtime) *corev1.PodSpec {
	switch runtime {
	case RuntimeNotebook, RuntimeServer, RuntimeBuilder, RuntimeTrainer:
		return &corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: string(runtime),
				},
			},
		}
	default:
		t.Fatalf("no test spec defined for runtime %v", runtime)
		return nil
	}
}
