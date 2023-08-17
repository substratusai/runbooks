package controller

import (
	"context"
	"fmt"

	"github.com/substratusai/substratus/internal/cloud"
	"github.com/substratusai/substratus/internal/sci"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

func AssociatePrincipalSCIServiceAccount(ctx context.Context, client *kubernetes.Clientset, cloud cloud.Cloud) error {
	namespace := "substratus"
	serviceAccountName := "sci"

	sa, err := client.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	cloud.AssociatePrincipal(sa)
	_, err = client.CoreV1().ServiceAccounts(namespace).Update(ctx, sa, metav1.UpdateOptions{})
	return err
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
