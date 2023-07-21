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

func reconcileCloudServiceAccount(ctx context.Context, cloudConfig cloud.Cloud, c client.Client, sa *corev1.ServiceAccount) (result, error) {
	configureSA := func() error {
		cloudConfig.AssociateServiceAccount(sa)
		return nil
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, c, sa, configureSA); err != nil {
		return result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	return result{success: true}, nil
}
