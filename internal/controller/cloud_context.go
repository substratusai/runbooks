package controller

import (
	"fmt"
	"net/http"
	"os"

	"cloud.google.com/go/compute/metadata"
	"github.com/kelseyhightower/envconfig"
	corev1 "k8s.io/api/core/v1"
)

type CloudContext interface {
	// AuthNServiceAccount adds the annotations necessary to run the given runtime with the given service account.
	// For example: On GKE, this will add the "iam.gke.io/gcp-service-account" annotation.
	AuthNServiceAccount(Runtime, *corev1.ServiceAccount) error
}

func NewCloudContext() (CloudContext, error) {
	envCloud, ok := os.LookupEnv("CLOUD")
	if ok {
		switch CloudType(envCloud) {
		case CloudTypeGCP:
			var c GCPCloudContext
			if err := envconfig.Process("GCP", &c); err != nil {
				return nil, fmt.Errorf("failed to process GCP environment variables: %w", err)
			}
			return &c, nil
		default:
			return nil, fmt.Errorf("unsupported cloud: %s", envCloud)
		}
	}

	if metadata.OnGCE() {
		return lookupGCPCloudContext()
	}
	return nil, fmt.Errorf("unable to determine cloud")
}

func lookupGCPCloudContext() (*GCPCloudContext, error) {
	md := metadata.NewClient(&http.Client{})

	var c GCPCloudContext

	var err error
	c.ProjectID, err = md.ProjectID()
	if err != nil {
		return nil, fmt.Errorf("failed to get project ID from metadata server: %w", err)
	}
	c.ClusterName, err = md.InstanceAttributeValue("cluster-name")
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster name from metadata server: %w", err)
	}
	c.ClusterLocation, err = md.InstanceAttributeValue("cluster-location")
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster location from metadata server: %w", err)
	}

	return &c, nil
}

type GCPCloudContext struct {
	ProjectID       string `envconfig:"PROJECT_ID" required:"true"`
	ClusterName     string `envconfig:"CLUSTER_NAME" required:"true"`
	ClusterLocation string `envconfig:"CLUSTER_LOCATION" required:"true"`
}

func (c *GCPCloudContext) AuthNServiceAccount(runtime Runtime, sa *corev1.ServiceAccount) error {
	switch runtime {
	case RuntimeBuilder:
		if sa.Annotations == nil {
			sa.Annotations = map[string]string{}
		}
		sa.Annotations["iam.gke.io/gcp-service-account"] = fmt.Sprintf("substratus-image-builder@%s.iam.gserviceaccount.com", c.ProjectID)
	case RuntimeDataPuller:
		if sa.Annotations == nil {
			sa.Annotations = map[string]string{}
		}
		sa.Annotations["iam.gke.io/gcp-service-account"] = fmt.Sprintf("substratus-data-puller@%s.iam.gserviceaccount.com", c.ProjectID)
	default:
		return fmt.Errorf("unsupported runtime: %s", runtime)
	}

	return nil
}
