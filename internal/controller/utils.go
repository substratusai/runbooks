package controller

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ptr "k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// object is the interface for all Substratus API objects.
type object interface {
	client.Object
	GetConditions() *[]metav1.Condition
	GetStatusReady() bool
	SetStatusReady(bool)
}

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

const (
	thousand = 1000
	million  = 1000 * 1000
	billion  = 1000 * 1000 * 1000

	gigabyte = int64(1024 * 1024 * 1024)
)

func roundUpGB(bytes int64) int64 {
	return int64(math.Ceil(float64(bytes)/float64(gigabyte))) * gigabyte
}

func parseBucketURL(bucketURL string) (string, string, error) {
	u, err := url.Parse(bucketURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing bucket url: %w", err)
	}

	bucket := u.Host
	subpath := strings.TrimPrefix(filepath.Dir(u.Path), "/")

	return bucket, subpath, nil
}

func mountDataset(annotations map[string]string, volumes *[]corev1.Volume, volumeMounts *[]corev1.VolumeMount, datasetURL string, readOnly bool) error {
	annotations["gke-gcsfuse/volumes"] = "true"

	bucket, subpath, err := parseBucketURL(datasetURL)
	if err != nil {
		return fmt.Errorf("parsing dataset url: %w", err)
	}

	*volumes = append(*volumes, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   "gcsfuse.csi.storage.gke.io",
				ReadOnly: ptr.Bool(readOnly),
				VolumeAttributes: map[string]string{
					"bucketName":   bucket,
					"mountOptions": "implicit-dirs,uid=0,gid=3003",
				},
			},
		},
	})
	*volumeMounts = append(*volumeMounts, corev1.VolumeMount{
		Name:      "data",
		MountPath: "/data",
		SubPath:   subpath,
		ReadOnly:  readOnly,
	})

	return nil
}

func mountModel(annotations map[string]string, volumes *[]corev1.Volume, volumeMounts *[]corev1.VolumeMount, modelURL string, prefix string, readOnly bool) error {
	annotations["gke-gcsfuse/volumes"] = "true"

	bucket, subpath, err := parseBucketURL(modelURL)
	if err != nil {
		return fmt.Errorf("parsing dataset url: %w", err)
	}

	*volumes = append(*volumes, corev1.Volume{
		Name: prefix + "model",
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   "gcsfuse.csi.storage.gke.io",
				ReadOnly: ptr.Bool(readOnly),
				VolumeAttributes: map[string]string{
					"bucketName":   bucket,
					"mountOptions": "implicit-dirs,uid=0,gid=3003",
				},
			},
		},
	})

	*volumeMounts = append(*volumeMounts,
		corev1.VolumeMount{
			Name:      prefix + "model",
			MountPath: "/" + prefix + "model/logs",
			SubPath:   subpath + "/logs",
		},
		corev1.VolumeMount{
			Name:      prefix + "model",
			MountPath: "/" + prefix + "model/saved",
			SubPath:   subpath + "/saved",
			ReadOnly:  readOnly,
		},
	)

	return nil
}

func reconcileJob(ctx context.Context, c client.Client, obj object, job *batchv1.Job, condition string) (result, error) {
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
	// TODO(nstogner): Order by key to avoid randomness.
	for k, v := range params {
		envs = append(envs, corev1.EnvVar{Name: "PARAM_" + strings.ToUpper(k), Value: v.String()})
	}
	return envs
}
