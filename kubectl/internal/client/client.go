package client

import (
	meta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/substratusai/substratus/api/v1"
)

var FieldManager = "kubectl"

func init() {
	apiv1.AddToScheme(scheme.Scheme)
}

type Client struct {
	resource *resource.Helper
	//kubeClientset kubernetes.Interface
	//restConfig    *rest.Config
}

func NewClientFor(kubeClientset kubernetes.Interface, restConfig *rest.Config, obj client.Object) (*Client, error) {
	res, err := resourceFor(kubeClientset, restConfig, obj)
	if err != nil {
		return nil, err
	}

	return &Client{
		resource: res,
		//kubeClientset: kubeClientset,
		//restConfig:    restConfig,
	}, nil
}

func resourceFor(kubeClientset kubernetes.Interface, restConfig *rest.Config, obj runtime.Object) (*resource.Helper, error) {
	// Create a REST mapper that tracks information about the available resources in the cluster.
	groupResources, err := restmapper.GetAPIGroupResources(kubeClientset.Discovery())
	if err != nil {
		return nil, err
	}
	rm := restmapper.NewDiscoveryRESTMapper(groupResources)

	// Get some metadata needed to make the REST request.
	gvk := obj.GetObjectKind().GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := rm.RESTMapping(gk, gvk.Version)
	if err != nil {
		return nil, err
	}

	name, err := meta.NewAccessor().Name(obj)
	if err != nil {
		return nil, err
	}
	_ = name

	// Create a client specifically for creating the object.
	restClient, err := newRestClient(restConfig, mapping.GroupVersionKind.GroupVersion())
	if err != nil {
		return nil, err
	}

	// Use the REST helper to create the object in the "default" namespace.
	return resource.NewHelper(restClient, mapping), nil
}

func newRestClient(restConfig *rest.Config, gv schema.GroupVersion) (rest.Interface, error) {
	restConfig.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	restConfig.GroupVersion = &gv
	if len(gv.Group) == 0 {
		restConfig.APIPath = "/api"
	} else {
		restConfig.APIPath = "/apis"
	}

	return rest.RESTClientFor(restConfig)
}
