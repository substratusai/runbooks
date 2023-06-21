package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apiv1 "github.com/substratusai/substratus/api/v1"
)

const (
	ReasonDeploymentReady    = "DeploymentReady"
	ReasonModelNotReady      = "ModelNotReady"
	ReasonDeploymentNotReady = "DeploymentNotReady"
)

// ModelServerReconciler reconciles a ModelServer object.
type ModelServerReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	GPUType GPUType
}

//+kubebuilder:rbac:groups=substratus.ai,resources=modelservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=modelservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=modelservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *ModelServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	lg.Info("Reconciling ModelServer")
	defer lg.Info("Done reconciling ModelServer")

	var server apiv1.ModelServer
	if err := r.Get(ctx, req.NamespacedName, &server); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var model apiv1.Model
	if err := r.Get(ctx, client.ObjectKey{Name: server.Spec.ModelName, Namespace: server.Namespace}, &model); err != nil {
		return ctrl.Result{}, fmt.Errorf("model not found: %w", err)
	}

	var isRegistered bool
	for _, svr := range model.Status.Servers {
		if svr == server.Name {
			isRegistered = true
			break
		}
	}
	if !isRegistered {
		// TODO: Remove from this list on deletion.
		model.Status.Servers = append(model.Status.Servers, server.Name)
		if err := r.Status().Update(ctx, &model); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update model status: %w", err)
		}
	}

	ready := meta.FindStatusCondition(model.Status.Conditions, ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue {
		lg.Info("Model not ready", "model", model.Name)

		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             ReasonModelNotReady,
			ObservedGeneration: server.Generation,
		})
		if err := r.Status().Update(ctx, &server); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update modelserver status: %w", err)
		}

		return ctrl.Result{}, nil
	}

	deploy, err := r.buildDeployment(&server, &model)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to build deployment: %w", err)
	}
	if err := r.Patch(ctx, deploy, client.Apply, client.FieldOwner("modelserver-controller")); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to apply deployment: %w", err)
	}

	if err := r.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, deploy); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get deployment: %w", err)
	}

	if deploy.Status.ReadyReplicas == 0 {
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             ReasonDeploymentNotReady,
			ObservedGeneration: server.Generation,
		})
	} else {
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             ReasonDeploymentReady,
			ObservedGeneration: server.Generation,
		})
	}

	if err := r.Status().Update(ctx, &server); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update model status: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.ModelServer{}).
		Watches(&source.Kind{Type: &apiv1.Model{}}, handler.EnqueueRequestsFromMapFunc(handler.MapFunc(modelServerForModel))).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

func modelServerForModel(obj client.Object) []reconcile.Request {
	model := obj.(*apiv1.Model)
	reqs := []reconcile.Request{}
	for _, svr := range model.Status.Servers {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: svr, Namespace: model.Namespace}})
	}
	return reqs
}

func (r *ModelServerReconciler) buildDeployment(server *apiv1.ModelServer, model *apiv1.Model) (*appsv1.Deployment, error) {
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name + "-server",
			Namespace: server.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			// TODO: HPA?
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"modelserver": server.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"modelserver": server.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            RuntimeServer,
							Image:           model.Status.ContainerImage,
							ImagePullPolicy: "Always",
							Command:         []string{"serve.sh"},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}

	if err := setRuntimeResources(model, &deploy.Spec.Template.Spec, r.GPUType, RuntimeServer); err != nil {
		return nil, fmt.Errorf("setting pod resources: %w", err)
	}

	if err := ctrl.SetControllerReference(server, deploy, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}
	return deploy, nil
}
