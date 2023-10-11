package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Apply(t *testing.T) {
	objectMeta := &metav1.ObjectMeta{Name: "test", Namespace: "test"}
	podSpec := &corev1.PodSpec{Containers: []corev1.Container{
		{Name: "test"},
	}}

	testCases := []struct {
		Name      string
		Resources *apiv1.Resources
		Expected  *apiv1.Resources
	}{
		{
			Name:      "not nil",
			Resources: &apiv1.Resources{CPU: 8, Memory: 8, Disk: 300},
			Expected:  &apiv1.Resources{CPU: 8, Memory: 8, Disk: 300},
		},
		{
			Name:      "nil",
			Resources: nil,
			Expected:  &apiv1.Resources{CPU: 2, Memory: 4, Disk: 100},
		},
	}

	for _, testCase := range testCases {
		t.Logf("Running test case %v", testCase.Name)
		err := Apply(objectMeta, podSpec, "test", cloud.GCPName, testCase.Resources)
		require.NoError(t, err, "Encountered error with case", testCase.Name)
		require.Equal(t, podSpec.Containers[0].Resources.Requests.Cpu(),
			resource.NewQuantity(testCase.Expected.CPU, resource.DecimalSI))
		require.Equal(t, podSpec.Containers[0].Resources.Requests.Memory(),
			resource.NewQuantity(testCase.Expected.Memory*gigabyte, resource.BinarySI))
		require.Equal(t, podSpec.Containers[0].Resources.Requests.StorageEphemeral(),
			resource.NewQuantity(testCase.Expected.Disk*gigabyte, resource.BinarySI))
	}
}
