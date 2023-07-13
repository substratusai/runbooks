package controller

import (
	"context"
	"fmt"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	notebookModelIndex   = "spec.model.name"
	notebookDatasetIndex = "spec.dataset.name"

	modelBaseModelIndex       = "spec.baseModel.name"
	modelTrainingDatasetIndex = "spec.trainingDataset.name"

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
		return fmt.Errorf("Notebook: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Notebook{}, notebookDatasetIndex, func(rawObj client.Object) []string {
		notebook := rawObj.(*apiv1.Notebook)
		if notebook.Spec.Dataset == nil {
			return []string{}
		}
		return []string{notebook.Spec.Dataset.Name}
	}); err != nil {
		return fmt.Errorf("Notebook: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Model{}, modelBaseModelIndex, func(rawObj client.Object) []string {
		model := rawObj.(*apiv1.Model)
		if model.Spec.BaseModel == nil {
			return []string{}
		}
		return []string{model.Spec.BaseModel.Name}
	}); err != nil {
		return fmt.Errorf("Model: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Model{}, modelTrainingDatasetIndex, func(rawObj client.Object) []string {
		model := rawObj.(*apiv1.Model)
		if model.Spec.TrainingDataset == nil {
			return []string{}
		}
		return []string{model.Spec.TrainingDataset.Name}
	}); err != nil {
		return fmt.Errorf("Model: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Server{}, modelServerModelIndex, func(rawObj client.Object) []string {
		server := rawObj.(*apiv1.Server)
		return []string{server.Spec.Model.Name}
	}); err != nil {
		return fmt.Errorf("Server: %w", err)
	}

	return nil
}
