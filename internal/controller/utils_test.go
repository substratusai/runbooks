package controller

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_resolveEnv(t *testing.T) {
	testCases := []struct {
		input    map[string]intstr.IntOrString
		expected []corev1.EnvVar
	}{
		{map[string]intstr.IntOrString{
			"TEST": intstr.FromString("test"),
		}, []corev1.EnvVar{
			{Name: "TEST", Value: "test"},
		}},
	}
	client := fake.NewClientBuilder().Build()
	for _, tc := range testCases {
		actual, err := resolveEnv(context.Background(), client, "default", tc.input)
		if err != nil {
			t.Errorf("error with case %v: %v", tc.input, err)
		}
		if !reflect.DeepEqual(actual, tc.expected) {
			t.Errorf("resolveEnv(%v): expected %v, actual %v", tc.input, tc.expected, actual)
		}
	}
}
