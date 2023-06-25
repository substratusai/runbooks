package controller

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func Test_setRuntimeResources(t *testing.T) {
	cases := []struct {
		name string

		model *apiv1.Model

		expected map[Runtime]string
	}{
		{
			name: "125m-32bit",
			model: &apiv1.Model{
				Spec: apiv1.ModelSpec{
					Size: apiv1.ModelSize{
						ParameterBits:  32,
						ParameterCount: 125 * million,
					},
				},
			},
			expected: map[Runtime]string{
				RuntimeTrainer: `
containers:
- name: trainer
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
				`,
				RuntimeNotebook: `
containers:
- name: notebook
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
				`,
				RuntimeServer: `
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
				`,
				RuntimeBuilder: `
containers:
- name: builder
  resources:
    requests:
      cpu: "2"
      ephemeral-storage: 101Gi
      memory: 12Gi
				`,
			},
		},
		{
			name: "7b-16bit",
			model: &apiv1.Model{
				Spec: apiv1.ModelSpec{
					Size: apiv1.ModelSize{
						ParameterBits:  16,
						ParameterCount: 7 * billion,
					},
				},
			},
			expected: map[Runtime]string{
				RuntimeTrainer: `
containers:
- name: trainer
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
				`,
				RuntimeNotebook: `
containers:
- name: notebook
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
				`,
				RuntimeServer: `
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
				`,
				RuntimeBuilder: `
containers:
- name: builder
  resources:
    requests:
      cpu: "2"
      ephemeral-storage: 127Gi
      memory: 12Gi
				`,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for runtime, expectedSpecYAML := range c.expected {
				t.Run(string(runtime), func(t *testing.T) {
					spec := testSpec(t, runtime)
					require.NoError(t, setRuntimeResources(c.model, spec, GPUTypeNvidiaL4, runtime))

					// Use YAML for comparison because it's easier to read
					// and makes generating expected output easier.
					actualSpecYAML, err := yaml.Marshal(spec)
					require.NoError(t, err)
					assert.Equal(t, strings.TrimSpace(expectedSpecYAML), strings.TrimSpace(string(actualSpecYAML)))

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
