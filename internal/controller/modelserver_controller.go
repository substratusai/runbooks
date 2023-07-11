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
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/internal/resources"
)

// ModelServerReconciler reconciles a ModelServer object.
type ModelServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	*ContainerImageReconciler

	// log should be used outside the context of Reconcile()
	log logr.Logger
}

//+kubebuilder:rbac:groups=substratus.ai,resources=modelservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=substratus.ai,resources=modelservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=substratus.ai,resources=modelservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch

func (r *ModelServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling ModelServer")
	defer log.Info("Done reconciling ModelServer")

	var server apiv1.ModelServer
	if err := r.Get(ctx, req.NamespacedName, &server); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if result, err := r.ReconcileContainerImage(ctx, &server); !result.success {
		return result.Result, err
	}

	if result, err := r.reconcileServer(ctx, &server); !result.success {
		return result.Result, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.log = mgr.GetLogger()

	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.ModelServer{}).
		Watches(&source.Kind{Type: &apiv1.Model{}}, handler.EnqueueRequestsFromMapFunc(handler.MapFunc(r.findServersForModel))).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *ModelServerReconciler) findServersForModel(obj client.Object) []reconcile.Request {
	model := obj.(*apiv1.Model)

	var servers apiv1.ModelServerList
	if err := r.List(context.Background(), &servers,
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

func (r *ModelServerReconciler) serverDeployment(server *apiv1.ModelServer, model *apiv1.Model) (*appsv1.Deployment, error) {
	replicas := int32(1)

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	annotations := map[string]string{}

	if model != nil {
		if err := mountModel(annotations, &volumes, &volumeMounts, model.Status.URL, "", true); err != nil {
			return nil, fmt.Errorf("appending model volume: %w", err)
		}
	}

	const containerName = "serve"
	annotations["kubectl.kubernetes.io/default-container"] = containerName
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
					ServiceAccountName: modelServerServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:            containerName,
							Image:           model.Spec.Image.Name,
							ImagePullPolicy: "Always",
							// NOTE: tini should be installed as the ENTRYPOINT the image and will be used
							// to execute this script.
							Command: server.Spec.Command,
							Ports: []corev1.ContainerPort{
								{
									Name:          modelServerHTTPServePortName,
									ContainerPort: 8080,
								},
							},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(server, deploy, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference: %w", err)
	}

	if err := resources.Apply(&deploy.Spec.Template.ObjectMeta, &deploy.Spec.Template.Spec, containerName,
		r.CloudContext.Name, server.Spec.Resources); err != nil {
		return nil, fmt.Errorf("applying resources: %w", err)
	}

	return deploy, nil
}

func (r *ModelServerReconciler) reconcileServer(ctx context.Context, server *apiv1.ModelServer) (result, error) {
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
			return result{}, fmt.Errorf("failed to update modelserver status: %w", err)
		}

		return result{}, nil
	}

	// ServiceAccount for loading the Model.
	// Within the context of GCP, this ServiceAccount will need IAM permissions
	// to read the GCS bucket containing the model.
	if result, err := reconcileCloudServiceAccount(ctx, r.CloudContext, r.Client, &corev1.ServiceAccount{
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
	if err := r.Patch(ctx, service, client.Apply, client.FieldOwner("modelserver-controller")); err != nil {
		return result{}, fmt.Errorf("failed to apply service: %w", err)
	}

	deploy, err := r.serverDeployment(server, &model)
	if err != nil {
		return result{}, fmt.Errorf("failed to construct deployment: %w", err)
	}
	if err := r.Patch(ctx, deploy, client.Apply, client.FieldOwner("modelserver-controller")); err != nil {
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

func withModelServerSelector(server *apiv1.ModelServer, labels map[string]string) map[string]string {
	labels["component"] = "modelserver"
	labels["modelserver"] = server.Name
	return labels
}
