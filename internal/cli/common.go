package cli

import (
	"k8s.io/client-go/kubernetes/scheme"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/client"
)

func init() {
	apiv1.AddToScheme(scheme.Scheme)
}

// NewClient is a dirty hack to allow the client to be mocked out in tests.
var NewClient = client.NewClient
