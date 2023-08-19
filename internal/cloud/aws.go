package cloud

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	AWSName                  = "aws"
	AWSWorkloadIdentityLabel = "eks.amazonaws.com/role-arn"
)

type AWS struct {
	Common
	AwsAccountId string `env:"AWS_ACCOUNT_ID" required:"true"`
	AwsRegion    string `env:"AWS_REGION" required:"true"`
	ClusterName  string `env:"CLUSTER_NAME" required:"true"`
}

func (aws *AWS) Name() string { return AWSName }

func (aws *AWS) AutoConfigure(ctx context.Context) error {
	if aws.AwsAccountId == "" {
		return fmt.Errorf("failed to get cluster name from env var ")
	}

	if aws.ClusterName == "" {
		return fmt.Errorf("failed to get cluster name from env var CLUSTER_NAME")
	}

	if aws.AwsRegion == "" {
		return fmt.Errorf("failed to get cluster name from env var AWS_REGION")
	}

	if aws.RegistryURL == "" {
		aws.RegistryURL = fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/substratus",
			aws.AwsAccountId, aws.AwsRegion)
	}

	if aws.ArtifactBucketURL == nil {
		aws.ArtifactBucketURL = &BucketURL{
			Scheme: "s3",
			Bucket: fmt.Sprintf("%s-substratus-artifacts", aws.AwsAccountId),
		}
	}

	if aws.Principal == "" {
		aws.Principal = fmt.Sprintf("arn:aws:iam::%s:role/substratus", aws.AwsAccountId)
	}

	return nil
}

// TODO(bjb): pick up here
func (aws *AWS) MountBucket(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec, obj ArtifactObject, req MountBucketConfig) error {
	if podMetadata.Annotations == nil {
		podMetadata.Annotations = map[string]string{}
	}
	podMetadata.Annotations["gke-gcsfuse/volumes"] = "true"
	podMetadata.Annotations["gke-gcsfuse/cpu-limit"] = "2"
	podMetadata.Annotations["gke-gcsfuse/memory-limit"] = "800Mi"
	podMetadata.Annotations["gke-gcsfuse/ephemeral-storage-limit"] = "100Gi"

	var bktURL *BucketURL
	if statusURL := obj.GetStatusArtifacts().URL; statusURL != "" {
		var err error
		bktURL, err = ParseBucketURL(statusURL)
		if err != nil {
			return fmt.Errorf("parsing status bucket url: %w", err)
		}
	} else {
		bktURL = aws.ObjectArtifactURL(obj)
	}

	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: req.Name,
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   "gcsfuse.csi.storage.gke.io",
				ReadOnly: ptr.To(req.ReadOnly),
				VolumeAttributes: map[string]string{
					"bucketName":   bktURL.Bucket,
					"mountOptions": "implicit-dirs,uid=0,gid=3003",
				},
			},
		},
	})

	for i := range podSpec.Containers {
		if podSpec.Containers[i].Name == req.Container {
			for _, mount := range req.Mounts {
				podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts,
					corev1.VolumeMount{
						Name:      req.Name,
						MountPath: "/content/" + mount.ContentSubdir,
						SubPath:   strings.TrimPrefix(bktURL.Path+"/"+mount.BucketSubdir, "/"),
						ReadOnly:  req.ReadOnly,
					},
				)
			}
			return nil
		}
	}

	return fmt.Errorf("container not found: %s", req.Container)
}

func (aws *AWS) GetPrincipal(sa *corev1.ServiceAccount) (string, bool) {
	principalBound := true
	if val, exist := sa.Annotations[AWSWorkloadIdentityLabel]; !exist || val != aws.Principal {
		principalBound = false
	}
	return aws.Principal, principalBound
}

func (aws *AWS) AssociatePrincipal(sa *corev1.ServiceAccount) {
	if sa.Annotations == nil {
		sa.Annotations = map[string]string{}
	}
	principal, _ := aws.GetPrincipal(sa)
	sa.Annotations[AWSWorkloadIdentityLabel] = principal
}
