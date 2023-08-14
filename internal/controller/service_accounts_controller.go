package controller

import (
	"context"
	"fmt"

	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/sci"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	containerBuilderServiceAccountName = "container-builder"
	modellerServiceAccountName         = "modeller"
	modelServerServiceAccountName      = "model-server"
	notebookServiceAccountName         = "notebook"
	dataLoaderServiceAccountName       = "data-loader"
)

type ServiceAccountReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Cloud cloud.Cloud
	SCI   sci.ControllerClient
}

func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != "substratus" && req.Name != "sci" {
		return ctrl.Result{}, nil
	}
	log := log.FromContext(ctx)
	log.Info("Reconciling ServiceAccount substratus/sci")
	defer log.Info("Done reconciling Service Account")
	var sa corev1.ServiceAccount
	if err := r.Get(ctx, req.NamespacedName, &sa); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if _, exists := r.Cloud.GetPrincipal(&sa); !exists {
		if sa.Annotations == nil {
			sa.Annotations = map[string]string{}
		}

		configureSA := func() error {
			r.Cloud.AssociatePrincipal(&sa)
			return nil
		}

		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, &sa, configureSA); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create or update SCI service account: %w", err)
		}
	}
	return ctrl.Result{}, nil
}

func reconcileServiceAccount(ctx context.Context, cloudConfig cloud.Cloud, sciClient sci.ControllerClient, c client.Client, sa *corev1.ServiceAccount) (result, error) {
	if sa.Annotations == nil {
		sa.Annotations = map[string]string{}
	}

	configureSA := func() error {
		cloudConfig.AssociatePrincipal(sa)
		return nil
	}

	principal, bound := cloudConfig.GetPrincipal(sa)
	if !bound {
		bindIdentityRequest := sci.BindIdentityRequest{
			Principal:                principal,
			KubernetesServiceAccount: sa.Name,
			KubernetesNamespace:      sa.Namespace,
		}
		if _, err := sciClient.BindIdentity(ctx, &bindIdentityRequest); err != nil {
			return result{}, fmt.Errorf("failed bind identity principal %s to K8s SA %s/%s: %w",
				principal, sa.Namespace, sa.Name, err)
		}
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, c, sa, configureSA); err != nil {
		return result{}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	return result{success: true}, nil
}

//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ServiceAccount{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(&corev1.ServiceAccount{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
