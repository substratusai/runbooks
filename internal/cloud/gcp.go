package cloud

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"cloud.google.com/go/compute/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

	if metadata.OnGCE() {
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
	}

	if gcp.RegistryURL == "" {
		gcp.RegistryURL = fmt.Sprintf("%s-docker.pkg.dev/%s/substratus", gcp.region(), gcp.ProjectID)
	}

	if gcp.ArtifactBucketURL == nil {
		gcp.ArtifactBucketURL = &BucketURL{
			Scheme: "gs",
			Bucket: fmt.Sprintf("%s-substratus-artifacts", gcp.ProjectID),
		}
	}

	return nil
}

func (gcp *GCP) MountBucket(podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec, obj ArtifactObject, req MountBucketConfig) error {
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
		bktURL = gcp.ObjectArtifactURL(obj)
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
						SubPath:   bktURL.Path + "/" + mount.BucketSubdir,
						ReadOnly:  req.ReadOnly,
					},
				)
			}
			return nil
		}
	}

	return fmt.Errorf("container not found: %s", req.Container)
}

func (gcp *GCP) GetPrincipal(sa *corev1.ServiceAccount) string {
	return fmt.Sprintf("substratus@%s.iam.gserviceaccount.com", gcp.ProjectID)
}

func (gcp *GCP) AssociatePrincipal(sa *corev1.ServiceAccount) {
	sa.Annotations["iam.gke.io/gcp-service-account"] = gcp.GetPrincipal(sa)
}

func (gcp *GCP) region() string {
	split := strings.Split(gcp.ClusterLocation, "-")
	if len(split) < 2 {
		panic("invalid cluster location: " + gcp.ClusterLocation)
	}
	return strings.Join(split[:2], "-")
}
