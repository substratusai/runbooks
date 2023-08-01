package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// result allows for propogating controller reconcile information up the call stack.
// In particular, it allows the called to determine if it should return or not.
type result struct {
	ctrl.Result
	success bool
}

func nextPowOf2(n int64) int64 {
	k := int64(1)
	for k < n {
		k = k << 1
	}
	return k
}

func reconcileJob(ctx context.Context, c client.Client, job *batchv1.Job, condition string) (result, error) {
	if err := c.Create(ctx, job); client.IgnoreAlreadyExists(err) != nil {
		return result{}, fmt.Errorf("creating Job: %w", err)
	}

	if err := c.Get(ctx, client.ObjectKeyFromObject(job), job); err != nil {
		return result{}, fmt.Errorf("geting Job: %w", err)
	}
	if job.Status.Succeeded < 1 {
		// Allow Job watch to requeue.
		return result{}, nil
	}

	return result{success: true}, nil
}

func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, c := range pod.Status.Conditions {
		if c.Type == "Ready" {
			if c.Status == "True" {
				return true
			}
		}
	}

	return false
}

func paramsToEnv(params map[string]intstr.IntOrString) []corev1.EnvVar {
	envs := []corev1.EnvVar{}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := params[k]
		envs = append(envs, corev1.EnvVar{Name: "PARAM_" + strings.ToUpper(k), Value: p.String()})
	}
	return envs
}
