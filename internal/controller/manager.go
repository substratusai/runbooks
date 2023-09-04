package controller

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apiv1 "github.com/substratusai/substratus/api/v1"
)

const (
	notebookModelIndex   = "spec.model.name"
	notebookDatasetIndex = "spec.dataset.name"

	modelModelIndex   = "spec.model.name"
	modelDatasetIndex = "spec.dataset.name"

	modelServerModelIndex = "spec.model.name"
)

func SetupIndexes(mgr manager.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Notebook{}, notebookModelIndex, func(rawObj client.Object) []string {
		notebook := rawObj.(*apiv1.Notebook)
		if notebook.Spec.Model == nil {
			return []string{}
		}
		return []string{notebook.Spec.Model.Name}
	}); err != nil {
		return fmt.Errorf("notebook: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Notebook{}, notebookDatasetIndex, func(rawObj client.Object) []string {
		notebook := rawObj.(*apiv1.Notebook)
		if notebook.Spec.Dataset == nil {
			return []string{}
		}
		return []string{notebook.Spec.Dataset.Name}
	}); err != nil {
		return fmt.Errorf("notebook: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Model{}, modelModelIndex, func(rawObj client.Object) []string {
		model := rawObj.(*apiv1.Model)
		if model.Spec.Model == nil {
			return []string{}
		}
		return []string{model.Spec.Model.Name}
	}); err != nil {
		return fmt.Errorf("model: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Model{}, modelDatasetIndex, func(rawObj client.Object) []string {
		model := rawObj.(*apiv1.Model)
		if model.Spec.Dataset == nil {
			return []string{}
		}
		return []string{model.Spec.Dataset.Name}
	}); err != nil {
		return fmt.Errorf("model: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Server{}, modelServerModelIndex, func(rawObj client.Object) []string {
		server := rawObj.(*apiv1.Server)
		return []string{server.Spec.Model.Name}
	}); err != nil {
		return fmt.Errorf("server: %w", err)
	}

	return nil
}
