package controller

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func Test_resolveEnv(t *testing.T) {
	envVarSource := &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "ai"},
			Key:                  "MYKEY",
		},
	}

	testCases := []struct {
		input    map[string]string
		expected []corev1.EnvVar
	}{
		// Test case with basic strings
		{map[string]string{
			"TEST": "test",
		}, []corev1.EnvVar{
			{Name: "TEST", Value: "test"},
		}},

		// Test case with secret ref
		{map[string]string{
			"TEST": "${{ secrets.ai.MYKEY }}",
		}, []corev1.EnvVar{
			{Name: "TEST", ValueFrom: envVarSource},
		}},

		// Test case with secret ref no spaces
		{map[string]string{
			"TEST": "${{secrets.ai.MYKEY}}",
		}, []corev1.EnvVar{
			{Name: "TEST", ValueFrom: envVarSource},
		}},

		// Test case with secret ref some spaces
		{map[string]string{
			"TEST": "${{ secrets.ai.MYKEY}}",
		}, []corev1.EnvVar{
			{Name: "TEST", ValueFrom: envVarSource},
		}},

		// Test case with secret ref some spaces
		{map[string]string{
			"TEST": "${{secrets.ai.MYKEY }}",
		}, []corev1.EnvVar{
			{Name: "TEST", ValueFrom: envVarSource},
		}},
	}

	for _, tc := range testCases {
		t.Log("running Test_resolveEnv with input", tc.input)
		actual, err := resolveEnv(tc.input)
		require.NoErrorf(t, err, "error with case %v: %v", tc.input, err)
		require.Truef(t, reflect.DeepEqual(actual, tc.expected), "resolveEnv(%v): expected %v, actual %v", tc.input, tc.expected, actual)
	}
}
