package controller

import (
	"context"
	"fmt"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func SetupIndexes(mgr manager.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Model{}, "spec.trainer.baseModel.name", func(rawObj client.Object) []string {
		model := rawObj.(*apiv1.Model)
		if model.Spec.Trainer == nil || model.Spec.Trainer.BaseModel == nil {
			return []string{}
		}
		return []string{model.Spec.Trainer.BaseModel.Name}
	}); err != nil {
		return fmt.Errorf("Model: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.Model{}, "spec.trainer.dataset.name", func(rawObj client.Object) []string {
		model := rawObj.(*apiv1.Model)
		if model.Spec.Trainer == nil {
			return []string{}
		}
		return []string{model.Spec.Trainer.Dataset.Name}
	}); err != nil {
		return fmt.Errorf("Model: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apiv1.ModelServer{}, "spec.model.name", func(rawObj client.Object) []string {
		server := rawObj.(*apiv1.ModelServer)
		return []string{server.Spec.Model.Name}
	}); err != nil {
		return fmt.Errorf("Model: %w", err)
	}

	return nil
}
