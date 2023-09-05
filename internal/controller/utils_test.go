package controller

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_resolveEnv(t *testing.T) {
	testCases := []struct {
		input    map[string]intstr.IntOrString
		expected []corev1.EnvVar
	}{
		// Test case with basic strings
		{map[string]intstr.IntOrString{
			"TEST": intstr.FromString("test"),
		}, []corev1.EnvVar{
			{Name: "TEST", Value: "test"},
		}},

		// Test case with secret ref
		{map[string]intstr.IntOrString{
			"TEST": intstr.FromString("${{ secrets.ai.MYKEY }}"),
		}, []corev1.EnvVar{
			{Name: "TEST", Value: "ai"},
		}},

		// Test case with secret ref no spaces
		{map[string]intstr.IntOrString{
			"TEST": intstr.FromString("${{secrets.ai.MYKEY}}"),
		}, []corev1.EnvVar{
			{Name: "TEST", Value: "ai"},
		}},

		// Test case with secret ref some spaces
		{map[string]intstr.IntOrString{
			"TEST": intstr.FromString("${{ secrets.ai.MYKEY}}"),
		}, []corev1.EnvVar{
			{Name: "TEST", Value: "ai"},
		}},

		// Test case with secret ref some spaces
		{map[string]intstr.IntOrString{
			"TEST": intstr.FromString("${{secrets.ai.MYKEY }}"),
		}, []corev1.EnvVar{
			{Name: "TEST", Value: "ai"},
		}},
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ai", Namespace: "default"},
		Data:       map[string][]byte{"MYKEY": []byte("ai")},
	}
	client := fake.NewClientBuilder().WithObjects(&secret).Build()
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
