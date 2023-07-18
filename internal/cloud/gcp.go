package cloud

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"cloud.google.com/go/compute/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ptr "k8s.io/utils/pointer"
)

const GCPName = "gcp"

type GCP struct {
	Common
	ProjectID       string `env:"PROJECT_ID" required:"true"`
	ClusterLocation string `env:"CLUSTER_LOCATION" required:"true"`
}

func (gcp *GCP) Name() string { return GCPName }

func (gcp *GCP) AutoConfigure(ctx context.Context) error {
	md := metadata.NewClient(&http.Client{})

	var err error
	if gcp.ProjectID == "" {
		gcp.ProjectID, err = md.ProjectID()
		if err != nil {
			return fmt.Errorf("failed to get project ID from metadata server: %w", err)
		}
	}

	if gcp.ClusterName == "" {
		gcp.ClusterName, err = md.InstanceAttributeValue("cluster-name")
		if err != nil {
			return fmt.Errorf("failed to get cluster name from metadata server: %w", err)
		}
	}

	if gcp.ClusterLocation == "" {
		gcp.ClusterLocation, err = md.InstanceAttributeValue("cluster-location")
		if err != nil {
			return fmt.Errorf("failed to get cluster location from metadata server: %w", err)
		}
	}

	if gcp.RegistryURL == "" {
		gcp.RegistryURL = fmt.Sprintf("%s-docker.pkg.dev/%s/substratus", gcp.region(), gcp.ProjectID)
	}

	if gcp.ArtifactBucketURL == "" {
		gcp.ArtifactBucketURL = fmt.Sprintf("gs://%s-substratus-artifacts", gcp.ProjectID)
	}

	return nil
}

func (gcp *GCP) MountBucket(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec, obj ArtifactObject, req MountBucketConfig) error {
	if podMetadata.Annotations == nil {
		podMetadata.Annotations = map[string]string{}
	}
	podMetadata.Annotations["gke-gcsfuse/volumes"] = "true"

	bucket, subpath, err := parseBucketURL(gcp.ObjectArtifactURL(obj))
	if err != nil {
		return fmt.Errorf("parsing dataset url: %w", err)
	}

	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: req.Name,
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   "gcsfuse.csi.storage.gke.io",
				ReadOnly: ptr.Bool(req.ReadOnly),
				VolumeAttributes: map[string]string{
					"bucketName":   bucket,
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
						SubPath:   subpath + "/" + mount.BucketSubdir,
						ReadOnly:  req.ReadOnly,
					},
				)
			}
			break
		}
	}

	return nil
}

func (gcp *GCP) AssociateServiceAccount(sa *corev1.ServiceAccount) {
	if sa.Annotations == nil {
		sa.Annotations = map[string]string{}
	}
	sa.Annotations["iam.gke.io/gcp-service-account"] = fmt.Sprintf("substratus-%s@%s.iam.gserviceaccount.com", sa.Name, gcp.ProjectID)
}

func (gcp *GCP) region() string {
	split := strings.Split(gcp.ClusterLocation, "-")
	if len(split) < 2 {
		panic("invalid cluster location: " + gcp.ClusterLocation)
	}
	return strings.Join(split[:2], "-")
}
