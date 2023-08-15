package cloud

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/compute/metadata"
	"github.com/go-playground/validator/v10"
	"github.com/sethvargo/go-envconfig"
	apiv1 "github.com/substratusai/substratus/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const CloudEnvVar = "CLOUD"

type Cloud interface {
	// Name of cloud.
	Name() string

	// AutoConfigure attempts to determine cloud configuration from metadata servers, etc.
	AutoConfigure(context.Context) error

	// ObjectBuildImageURL returns the URL of the container image that should be used
	// when Substratus builds a given Object.
	ObjectBuiltImageURL(BuildableObject) string

	// ObjectArtifactURL returns the URL of the artifact that was stored for a given Object.
	ObjectArtifactURL(Object) *BucketURL

	// AssociatePrincipal associates the given K8s service account with a cloud
	// identity (i.e. updates cloud specific annotations on K8s SA)
	AssociatePrincipal(*corev1.ServiceAccount)

	// GetPrincipal returns the IAM Principal (GCP SA, AWS IAM Role) that should be used
	// for a specific K8s Service Account. Returns the principal and whether the principal
	// was already bound successfully to the service account.
	GetPrincipal(*corev1.ServiceAccount) (string, bool)

	// MountBucket mutates the given Pod metadata and Pod spec in order to append
	// volumes mounts for a bucket.
	MountBucket(*metav1.ObjectMeta, *corev1.PodSpec, ArtifactObject, MountBucketConfig) error
}

func New(ctx context.Context) (Cloud, error) {
	var c Cloud
	// If CLOUD is set, then pull configuration from environment variables.
	cloudName := os.Getenv(CloudEnvVar)

	if cloudName == "" {
		if metadata.OnGCE() {
			cloudName = "gcp"
		}
	}

	switch cloudName {
	case GCPName:
		c = &GCP{}
	case KindName:
		c = &Kind{}
	default:
		if cloudName == "" {
			return nil, fmt.Errorf("unable to determine cloud, if running remotely, specify %v environment variable", CloudEnvVar)
		} else {
			return nil, fmt.Errorf("unsupported cloud: %q", cloudName)
		}
	}

	if err := envconfig.Process(ctx, c); err != nil {
		return nil, fmt.Errorf("environment: %w", err)
	}

	if err := c.AutoConfigure(ctx); err != nil {
		return nil, fmt.Errorf("autoconfigure: %w", err)
	}

	if err := validator.New().Struct(c); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	return c, nil
}

type BucketMount struct {
	BucketSubdir  string
	ContentSubdir string
}

type MountBucketConfig struct {
	Container string        // Example: trainer, loader
	Name      string        // Example: model, model-saved, data
	Mounts    []BucketMount // Example: model, data, logs
	ReadOnly  bool
}

type Object = client.Object

type BuildableObject interface {
	client.Object
	GetBuild() *apiv1.Build
	SetImage(string)
	GetImage() string
}

type ArtifactObject interface {
	client.Object
	GetStatusArtifacts() apiv1.ArtifactsStatus
}
