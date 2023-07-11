package controller

import (
	"context"
	"fmt"

	"github.com/substratusai/substratus/internal/cloud"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	containerBuilderServiceAccountName = "container-builder"
	modellerServiceAccountName         = "modeller"
	modelServerServiceAccountName      = "model-server"
	notebookServiceAccountName         = "notebook"
	dataLoaderServiceAccountName       = "data-loader"
)

func reconcileCloudServiceAccount(ctx context.Context, cloudCtx *cloud.Context, c client.Client, sa *corev1.ServiceAccount) (result, error) {
	configureSA := func() error {
		if sa.Annotations == nil {
			sa.Annotations = make(map[string]string)
		}
		switch name := cloudCtx.Name; name {
		case cloud.GCP:
			sa.Annotations["iam.gke.io/gcp-service-account"] = fmt.Sprintf("substratus-%s@%s.iam.gserviceaccount.com", sa.Name, cloudCtx.GCP.ProjectID)
		default:
			return fmt.Errorf("unsupported cloud type: %q", name)
		}
		return nil
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, c, sa, configureSA); err != nil {
		return result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	return result{success: true}, nil
}
