package cloud

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_artifactHashInput(t *testing.T) {
	obj := &testArtifactObject{}
	obj.SetKind("Model")
	obj.SetName("my-model")
	obj.SetNamespace("my-ns")

	require.Equal(t, "clusters/my-cluster/namespaces/my-ns/models/my-model",
		artifactHashInput("my-cluster", obj),
	)
}

type testArtifactObject struct {
	unstructured.Unstructured
}

func (o *testArtifactObject) GetStatusURL() string {
	return ""
}
