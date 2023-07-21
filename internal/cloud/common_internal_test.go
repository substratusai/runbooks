package cloud

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv1 "github.com/substratusai/substratus/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_artifactHashInput(t *testing.T) {
	model := &apiv1.Model{TypeMeta: metav1.TypeMeta{Kind: "Model"}, ObjectMeta: metav1.ObjectMeta{Name: "my-model", Namespace: "my-ns"}}

	require.Equal(t, "clusters/my-cluster/namespaces/my-ns/models/my-model",
		objectHashInput("my-cluster", model),
	)
}
