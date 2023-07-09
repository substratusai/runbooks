package cloud

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/kelseyhightower/envconfig"
)

// Context carries information about the cloud the controller is running in.
type Context struct {
	Name Name
	GCP  *GCPContext
}

type GCPContext struct {
	ProjectID       string `envconfig:"PROJECT_ID" required:"true"`
	ClusterName     string `envconfig:"CLUSTER_NAME" required:"true"`
	ClusterLocation string `envconfig:"CLUSTER_LOCATION" required:"true"`
}

func (gcp *GCPContext) Region() string {
	split := strings.Split(gcp.ClusterLocation, "-")
	if len(split) < 2 {
		panic("invalid cluster location: " + gcp.ClusterLocation)
	}
	return strings.Join(split[:2], "-")
}

func NewContext() (*Context, error) {
	// If CLOUD is set, then pull configuration from environment variables.
	envCloud, ok := os.LookupEnv("CLOUD")
	if ok {
		switch Name(envCloud) {
		case GCP:
			var gcp GCPContext
			if err := envconfig.Process("GCP", &gcp); err != nil {
				return nil, fmt.Errorf("failed to process GCP environment variables: %w", err)
			}
			return &Context{
				Name: GCP,
				GCP:  &gcp,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported cloud: %s", envCloud)
		}
	}

	if metadata.OnGCE() {
		gcp, err := lookupGCPContext()
		if err != nil {
			return nil, fmt.Errorf("looking up in cluster cloud context: %w", err)
		}
		return &Context{
			Name: GCP,
			GCP:  gcp,
		}, nil
	}

	return nil, fmt.Errorf("unable to determine cloud")
}

func lookupGCPContext() (*GCPContext, error) {
	md := metadata.NewClient(&http.Client{})

	var c GCPContext

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
