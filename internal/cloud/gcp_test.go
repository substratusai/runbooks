package cloud_test

import (
	"context"
	"os"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/sethvargo/go-envconfig"
	"github.com/stretchr/testify/require"
	"github.com/substratusai/substratus/internal/cloud"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGCP(t *testing.T) {
	var gcp cloud.GCP
	expectedPrincipal := "substratus@my-project.iam.gserviceaccount.com"
	os.Setenv("CLUSTER_NAME", "my-cluster")
	os.Setenv("ARTIFACT_BUCKET_URL", "gs://my-artifact-bucket")
	os.Setenv("REGISTRY_URL", "gcr.io/my-project")
	os.Setenv("PRINCIPAL", "substratus@my-project.iam.gserviceaccount.com")
	os.Setenv("PROJECT_ID", "my-project")
	os.Setenv("CLUSTER_LOCATION", "us-central1")

	require.Error(t, validator.New().Struct(&gcp))
	require.NoError(t, envconfig.Process(context.Background(), &gcp))
	require.NoError(t, validator.New().Struct(&gcp))

	sa := corev1.ServiceAccount{}
	actualPrincipal, bound := gcp.GetPrincipal(&sa)
	require.Equal(t, actualPrincipal, expectedPrincipal)
	require.Equal(t, bound, false)

	gcp.AssociatePrincipal(&sa)
	actualPrincipal, bound = gcp.GetPrincipal(&sa)
	require.Equal(t, actualPrincipal, expectedPrincipal)
	require.Equal(t, bound, true)

	sa = corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{cloud.GCPWorkloadIdentityLabel: expectedPrincipal},
		},
	}
	actualPrincipal, bound = gcp.GetPrincipal(&sa)
	require.Equal(t, actualPrincipal, expectedPrincipal)
	require.Equal(t, bound, true)
}
