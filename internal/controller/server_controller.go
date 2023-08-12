package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
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

	"github.com/go-logr/logr"
	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/resources"
	"github.com/substratusai/substratus/internal/sci"
)

// ServerReconciler reconciles a Server object.
type ServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Cloud cloud.Cloud
	SCI   sci.ControllerClient

	// log should be used outside the context of Reconcile()
	log logr.Logger
}

//+kubebuilder:rbac:groups=substratus.ai,resources=servers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=servers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=servers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch

func (r *ServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling Server")
	defer log.Info("Done reconciling Server")

	var server apiv1.Server
	if err := r.Get(ctx, req.NamespacedName, &server); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if server.GetImage() == "" {
		// Image must be building.
		return ctrl.Result{}, nil
	}

	if result, err := r.reconcileServer(ctx, &server); !result.success {
		return result.Result, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.log = mgr.GetLogger()

	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Server{}).
		Watches(&apiv1.Model{}, handler.EnqueueRequestsFromMapFunc(handler.MapFunc(r.findServersForModel))).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *ServerReconciler) findServersForModel(ctx context.Context, obj client.Object) []reconcile.Request {
	model := obj.(*apiv1.Model)

	var servers apiv1.ServerList
	if err := r.List(ctx, &servers,
		client.MatchingFields{modelServerModelIndex: model.Name},
		client.InNamespace(obj.GetNamespace()),
	); err != nil {
		log.Log.Error(err, "unable to list servers for model")
		return nil
	}

	reqs := []reconcile.Request{}
	for _, svr := range servers.Items {

		reqs = append(reqs, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      svr.Name,
				Namespace: svr.Namespace,
			},
		})
	}
	return reqs
}

func (r *ServerReconciler) serverDeployment(server *apiv1.Server, model *apiv1.Model) (*appsv1.Deployment, error) {
	replicas := int32(1)

	const containerName = "serve"
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
					"server": server.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: withServerSelector(server, map[string]string{}),
					Annotations: map[string]string{
						"kubectl.kubernetes.io/default-container": containerName,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: modelServerServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:            containerName,
							Image:           server.GetImage(),
							ImagePullPolicy: "Always",
							Command:         server.Spec.Command,
							Ports: []corev1.ContainerPort{
								{
									Name:          modelServerHTTPServePortName,
									ContainerPort: 8080,
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										// TBD: Not sure if this should ever be something we configure via
										// the Server API. For now, we'll just hardcode it and add it to the container
										// contract.
										Path: "/",
										Port: intstr.FromString(modelServerHTTPServePortName),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := r.Cloud.MountBucket(&deploy.Spec.Template.ObjectMeta, &deploy.Spec.Template.Spec, model, cloud.MountBucketConfig{
		Name: "model",
		Mounts: []cloud.BucketMount{
			{BucketSubdir: "model", ContentSubdir: "saved-model"},
		},
		Container: containerName,
		ReadOnly:  true,
	}); err != nil {
		return nil, fmt.Errorf("mounting model: %w", err)
	}

	if err := ctrl.SetControllerReference(server, deploy, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	if err := resources.Apply(&deploy.Spec.Template.ObjectMeta, &deploy.Spec.Template.Spec, containerName,
		r.Cloud.Name(), server.Spec.Resources); err != nil {
		return nil, fmt.Errorf("applying resources: %w", err)
	}

	return deploy, nil
}

func (r *ServerReconciler) reconcileServer(ctx context.Context, server *apiv1.Server) (result, error) {
	log := log.FromContext(ctx)

	var model apiv1.Model
	if err := r.Get(ctx, client.ObjectKey{Name: server.Spec.Model.Name, Namespace: server.Namespace}, &model); err != nil {
		if apierrors.IsNotFound(err) {
			// Update this Model's status.
			server.Status.Ready = false
			meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
				Type:               apiv1.ConditionDeployed,
				Status:             metav1.ConditionFalse,
				Reason:             apiv1.ReasonModelNotFound,
				ObservedGeneration: server.Generation,
			})
			if err := r.Status().Update(ctx, server); err != nil {
				return result{}, fmt.Errorf("failed to update server status: %w", err)
			}

			return result{}, nil
		}

		return result{}, fmt.Errorf("getting model: %w", err)
	}

	if !model.Status.Ready {
		log.Info("Model not ready", "model", model.Name)

		server.Status.Ready = false
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               apiv1.ConditionDeployed,
			Status:             metav1.ConditionFalse,
			Reason:             apiv1.ReasonModelNotReady,
			ObservedGeneration: server.Generation,
		})
		if err := r.Status().Update(ctx, server); err != nil {
			return result{}, fmt.Errorf("failed to update server status: %w", err)
		}

		return result{}, nil
	}

	// ServiceAccount for loading the Model.
	// Within the context of GCP, this ServiceAccount will need IAM permissions
	// to read the GCS bucket containing the model.
	if result, err := reconcileServiceAccount(ctx, r.Cloud, r.SCI, r.Client, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      modelServerServiceAccountName,
			Namespace: model.Namespace,
		},
	}); !result.success {
		return result, err
	}

	service, err := r.serverService(server, &model)
	if err != nil {
		return result{}, fmt.Errorf("failed to construct service: %w", err)
	}
	if err := r.Patch(ctx, service, client.Apply, client.FieldOwner("server-controller")); err != nil {
		return result{}, fmt.Errorf("failed to apply service: %w", err)
	}

	deploy, err := r.serverDeployment(server, &model)
	if err != nil {
		return result{}, fmt.Errorf("failed to construct deployment: %w", err)
	}
	if err := r.Patch(ctx, deploy, client.Apply, client.FieldOwner("server-controller")); err != nil {
		return result{}, fmt.Errorf("failed to apply deployment: %w", err)
	}

	if err := r.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, deploy); err != nil {
		return result{}, fmt.Errorf("failed to get deployment: %w", err)
	}

	if deploy.Status.ReadyReplicas == 0 {
		server.Status.Ready = false
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               apiv1.ConditionDeployed,
			Status:             metav1.ConditionFalse,
			Reason:             apiv1.ReasonDeploymentNotReady,
			ObservedGeneration: server.Generation,
		})
	} else {
		server.Status.Ready = true
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               apiv1.ConditionDeployed,
			Status:             metav1.ConditionTrue,
			Reason:             apiv1.ReasonDeploymentReady,
			ObservedGeneration: server.Generation,
		})
	}

	if err := r.Status().Update(ctx, server); err != nil {
		return result{}, fmt.Errorf("failed to update model status: %w", err)
	}

	return result{success: true}, nil
}

const modelServerHTTPServePortName = "http-serve"

func (r *ServerReconciler) serverService(server *apiv1.Server, model *apiv1.Model) (*corev1.Service, error) {
	s := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name + "-server",
			Namespace: server.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: withServerSelector(server, map[string]string{}),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.FromString(modelServerHTTPServePortName),
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(server, s, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	return s, nil
}

func withServerSelector(server *apiv1.Server, labels map[string]string) map[string]string {
	labels["component"] = "server"
	labels["server"] = server.Name
	return labels
}
