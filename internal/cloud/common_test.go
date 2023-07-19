package cloud_test

import (
	"context"
	"os"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/sethvargo/go-envconfig"
	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCommon(t *testing.T) {
	var common cloud.Common
	os.Setenv("CLUSTER_NAME", "my-cluster")
	os.Setenv("ARTIFACT_BUCKET_URL", "gs://my-artifact-bucket")
	os.Setenv("REGISTRY_URL", "gcr.io/my-project")

	require.Error(t, validator.New().Struct(&common))
	require.NoError(t, envconfig.Process(context.Background(), &common))
	require.NoError(t, validator.New().Struct(&common))

	require.EqualValues(t, cloud.Common{
		ClusterName:       "my-cluster",
		ArtifactBucketURL: "gs://my-artifact-bucket",
		RegistryURL:       "gcr.io/my-project",
	}, common)

	require.Equal(t, "gcr.io/my-project/my-cluster-model-my-ns-my-model", common.ObjectBuiltImageURL(&apiv1.Model{TypeMeta: metav1.TypeMeta{Kind: "Model"}, ObjectMeta: metav1.ObjectMeta{Name: "my-model", Namespace: "my-ns"}}))
	require.Equal(t, "gs://my-artifact-bucket/93ea94b18012ca14d84e1468d65e8709", common.ObjectArtifactURL(&apiv1.Model{TypeMeta: metav1.TypeMeta{Kind: "Model"}, ObjectMeta: metav1.ObjectMeta{Name: "my-model", Namespace: "my-ns"}}))
}
