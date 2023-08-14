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
	os.Setenv("PRINCIPAL", "dummy-value")

	require.Error(t, validator.New().Struct(&common))
	require.NoError(t, envconfig.Process(context.Background(), &common))
	require.NoError(t, validator.New().Struct(&common))

	require.EqualValues(t, cloud.Common{
		ClusterName:       "my-cluster",
		ArtifactBucketURL: &cloud.BucketURL{Scheme: "gs", Bucket: "my-artifact-bucket", Path: ""},
		RegistryURL:       "gcr.io/my-project",
		Principal:         "dummy-value",
	}, common)

	require.Equal(t, "gcr.io/my-project/my-cluster-model-my-ns-my-model:latest", common.ObjectBuiltImageURL(&apiv1.Model{
		TypeMeta:   metav1.TypeMeta{Kind: "Model"},
		ObjectMeta: metav1.ObjectMeta{Name: "my-model", Namespace: "my-ns"},
		Spec: apiv1.ModelSpec{
			Build: &apiv1.Build{},
		},
	}))
	require.Equal(t, "gcr.io/my-project/my-cluster-model-my-ns-my-model:v1.2.3", common.ObjectBuiltImageURL(&apiv1.Model{
		TypeMeta:   metav1.TypeMeta{Kind: "Model"},
		ObjectMeta: metav1.ObjectMeta{Name: "my-model", Namespace: "my-ns"},
		Spec: apiv1.ModelSpec{
			Build: &apiv1.Build{
				Git: &apiv1.BuildGit{
					Tag: "v1.2.3",
				},
			},
		},
	}))
	require.Equal(t, "gcr.io/my-project/my-cluster-model-my-ns-my-model:feature-x", common.ObjectBuiltImageURL(&apiv1.Model{
		TypeMeta:   metav1.TypeMeta{Kind: "Model"},
		ObjectMeta: metav1.ObjectMeta{Name: "my-model", Namespace: "my-ns"},
		Spec: apiv1.ModelSpec{
			Build: &apiv1.Build{
				Git: &apiv1.BuildGit{
					Branch: "feature-x",
				},
			},
		},
	}))
	require.Equal(t, "gcr.io/my-project/my-cluster-model-my-ns-my-model:80355073480594a99470dcacccd8cf2c", common.ObjectBuiltImageURL(&apiv1.Model{
		TypeMeta:   metav1.TypeMeta{Kind: "Model"},
		ObjectMeta: metav1.ObjectMeta{Name: "my-model", Namespace: "my-ns"},
		Spec: apiv1.ModelSpec{
			Build: &apiv1.Build{
				Upload: &apiv1.BuildUpload{
					MD5Checksum: "80355073480594a99470dcacccd8cf2c",
				},
			},
		},
	}))
	require.Equal(t, "gs://my-artifact-bucket/93ea94b18012ca14d84e1468d65e8709", common.ObjectArtifactURL(&apiv1.Model{TypeMeta: metav1.TypeMeta{Kind: "Model"}, ObjectMeta: metav1.ObjectMeta{Name: "my-model", Namespace: "my-ns"}}).String())
}
