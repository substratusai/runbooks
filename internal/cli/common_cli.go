package cli

import (
	"k8s.io/client-go/kubernetes/scheme"

	apiv1 "github.com/substratusai/substratus/api/v1"
)

func init() {
	apiv1.AddToScheme(scheme.Scheme)
}

const defaultFilename = "substratus.yaml"
