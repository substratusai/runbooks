package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	Scheme *runtime.Scheme

	*ContainerReconciler
}

//+kubebuilder:rbac:groups=substratus.ai,resources=modelservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=modelservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=modelservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *ModelServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lg := log.FromContext(ctx)

	lg.Info("Reconciling ModelServer")
	defer lg.Info("Done reconciling ModelServer")

	var server apiv1.ModelServer
	if err := r.Get(ctx, req.NamespacedName, &server); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if result, err := r.ReconcileContainer(ctx, &server); !result.Complete {
		return result.Result, err
	}

	var model apiv1.Model
	if err := r.Get(ctx, client.ObjectKey{Name: server.Spec.Model.Name, Namespace: server.Namespace}, &model); err != nil {
		if apierrors.IsNotFound(err) {
			// Update this Model's status.
			meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
				Type:               ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonSourceModelNotFound,
				ObservedGeneration: server.Generation,
			})
			if err := r.Status().Update(ctx, &server); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to update server status: %w", err)
			}

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("getting model: %w", err)
	}

	var isRegistered bool
	for _, svr := range model.Status.Servers {
		if svr == server.Name {
			isRegistered = true
			break
		}
	}
	if !isRegistered {
		// TODO: Stop using this, switch to cache index for enqueueing.
		// NOTE: There is no cleanup of this list at the moment.
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

	service, err := r.serverService(&server, &model)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to construct service: %w", err)
	}
	if err := r.Patch(ctx, service, client.Apply, client.FieldOwner("modelserver-controller")); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to apply service: %w", err)
	}

	deploy, err := r.serverDeployment(&server, &model)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to construct deployment: %w", err)
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
		Owns(&corev1.Service{}).
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

func (r *ModelServerReconciler) serverDeployment(server *apiv1.ModelServer, model *apiv1.Model) (*appsv1.Deployment, error) {
	replicas := int32(1)

	const serverContainerName = "server"
	annotations := map[string]string{}
	annotations["kubectl.kubernetes.io/default-container"] = serverContainerName
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name + "-modelserver",
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
					Labels:      withModelServerSelector(server, map[string]string{}),
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            serverContainerName,
							Image:           model.Spec.Container.Image,
							ImagePullPolicy: "Always",
							// NOTE: tini should be installed as the ENTRYPOINT the image and will be used
							// to execute this script.
							Args: []string{"serve.sh"},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http-app",
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(server, deploy, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}
	return deploy, nil
}

func (r *ModelServerReconciler) serverService(server *apiv1.ModelServer, model *apiv1.Model) (*corev1.Service, error) {
	s := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name + "-modelserver",
			Namespace: server.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: withModelServerSelector(server, map[string]string{}),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.FromString("http-app"),
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(server, s, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return s, nil
}

func withModelServerSelector(server *apiv1.ModelServer, labels map[string]string) map[string]string {
	labels["component"] = "modelserver"
	labels["modelserver"] = server.Name
	return labels
}
