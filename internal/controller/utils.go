package controller

import (
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"strings"

	apiv1 "github.com/substratusai/substratus/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ptr "k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// result allows for propogating controller reconcile information up the call stack.
// In particular, it allows the called to determine if it should return or not.
type result struct {
	ctrl.Result
	Complete bool
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

type Object interface {
	client.Object
	GetConditions() *[]metav1.Condition
}

func isReady(obj Object) bool {
	condition := meta.FindStatusCondition(*obj.GetConditions(), apiv1.ConditionReady)
	return condition != nil && condition.Status == metav1.ConditionTrue
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

func mountDataset(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, dataset *apiv1.Dataset) error {
	bucket, subpath, err := parseBucketURL(dataset.Status.URL)
	if err != nil {
		return fmt.Errorf("parsing dataset url: %w", err)
	}

	volumes = append(volumes, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   "gcsfuse.csi.storage.gke.io",
				ReadOnly: ptr.Bool(true),
				VolumeAttributes: map[string]string{
					"bucketName":   bucket,
					"mountOptions": "implicit-dirs,uid=0,gid=3003",
				},
			},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "data",
		MountPath: "/data",
		SubPath:   subpath,
		ReadOnly:  true,
	})

	return nil
}

func mountSavedModel(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, savedModel *apiv1.Model) error {
	bucket, subpath, err := parseBucketURL(savedModel.Status.URL)
	if err != nil {
		return fmt.Errorf("parsing dataset url: %w", err)
	}

	volumes = append(volumes, corev1.Volume{
		Name: "saved-model",
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   "gcsfuse.csi.storage.gke.io",
				ReadOnly: ptr.Bool(true),
				VolumeAttributes: map[string]string{
					"bucketName":   bucket,
					"mountOptions": "implicit-dirs,uid=0,gid=3003",
				},
			},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "saved-model",
		MountPath: "/model/saved",
		SubPath:   subpath,
		ReadOnly:  true,
	})

	return nil
}
