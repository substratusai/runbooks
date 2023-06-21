package controller

import (
	"fmt"
	"net/http"
	"os"

	"cloud.google.com/go/compute/metadata"
	"github.com/kelseyhightower/envconfig"
)

// CloudContext carries information about the cloud the controller is running in.
type CloudContext struct {
	CloudType CloudType
	GCP       *GCPCloudContext
}

type GCPCloudContext struct {
	ProjectID       string `envconfig:"PROJECT_ID" required:"true"`
	ClusterName     string `envconfig:"CLUSTER_NAME" required:"true"`
	ClusterLocation string `envconfig:"CLUSTER_LOCATION" required:"true"`
}

func NewCloudContext() (*CloudContext, error) {
	envCloud, ok := os.LookupEnv("CLOUD")
	if ok {
		switch CloudType(envCloud) {
		case CloudTypeGCP:
			var gcp GCPCloudContext
			if err := envconfig.Process("GCP", &gcp); err != nil {
				return nil, fmt.Errorf("failed to process GCP environment variables: %w", err)
			}
			return &CloudContext{
				CloudType: CloudTypeGCP,
				GCP:       &gcp,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported cloud: %s", envCloud)
		}
	}

	if metadata.OnGCE() {
		gcp, err := lookupGCPCloudContext()
		if err != nil {
			return nil, fmt.Errorf("looking up in cluster cloud context: %w", err)
		}
		return &CloudContext{
			CloudType: CloudTypeGCP,
			GCP:       gcp,
		}, nil
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
