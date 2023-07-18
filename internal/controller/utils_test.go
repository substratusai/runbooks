package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_nextPowOf2(t *testing.T) {
	testCases := []struct {
		input    int64
		expected int64
	}{
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{6, 8},
		{7, 8},
		{8, 8},
	}
	for _, tc := range testCases {
		actual := nextPowOf2(tc.input)
		if actual != tc.expected {
			t.Errorf("nextPowOf(%d): expected %d, actual %d", tc.input, tc.expected, actual)
		}
	}
}

func Test_hashInputForObject(t *testing.T) {
	cases := []struct {
		name     string
		obj      object
		expected string
	}{
		{
			name: "model",
			obj: &apiv1.Model{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-model",
					Namespace: "my-ns",
				},
				TypeMeta: metav1.TypeMeta{
					Kind: "Model",
				},
			},
			expected: "clusters/my-cluster/namespaces/my-ns/models/my-model",
		},
		{
			name: "notebook",
			obj: &apiv1.Notebook{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-notebook",
					Namespace: "my-ns",
				},
				TypeMeta: metav1.TypeMeta{
					Kind: "Notebook",
				},
			},
			expected: "clusters/my-cluster/namespaces/my-ns/notebooks/my-notebook",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.expected, hashInputForObject("my-cluster", c.obj))
		})
	}
}
