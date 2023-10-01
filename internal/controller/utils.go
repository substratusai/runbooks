package controller

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// result allows for propogating controller reconcile information up the call stack.
// In particular, it allows the called to determine if it should return or not.
type result struct {
	ctrl.Result
	success bool
	failure bool
}

func reconcileJob(ctx context.Context, c client.Client, job *batchv1.Job) (result, error) {
	if err := c.Create(ctx, job); client.IgnoreAlreadyExists(err) != nil {
		return result{}, fmt.Errorf("creating Job: %w", err)
	}

	if err := c.Get(ctx, client.ObjectKeyFromObject(job), job); err != nil {
		return result{}, fmt.Errorf("geting Job: %w", err)
	}

	complete, failed := jobResult(job)

	return result{success: complete, failure: failed}, nil
}

func jobResult(job *batchv1.Job) (complete bool, failed bool) {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			complete = true
			return
		}
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			failed = true
			return
		}
	}
	return
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

func resolveEnv(env map[string]string) ([]corev1.EnvVar, error) {
	envs := []corev1.EnvVar{}

	for key, value := range env {
		// Format ${{ secrets.my-name.my-key }} and spaces optional, following syntax of GitHub actions
		secretRegex := regexp.MustCompile(`\${{ *secrets\.(.+)\.(.+) *}}`)
		if secretRegex.MatchString(value) {
			matches := secretRegex.FindStringSubmatch(value)
			if len(matches) != 3 {
				return nil, fmt.Errorf("error parsing environment key %s, expecting format ${{ secrets.name.key }} but got  %v", key, value)
			}
			secretName := strings.TrimSpace(matches[1])
			secretKeyName := strings.TrimSpace(matches[2])

			envVarSource := &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  secretKeyName,
				},
			}
			envs = append(envs, corev1.EnvVar{Name: key, ValueFrom: envVarSource})
		} else {
			envs = append(envs, corev1.EnvVar{Name: key, Value: value})
		}
	}
	return envs, nil
}
