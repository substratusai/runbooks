package client

import (
	"context"
	"fmt"
	"time"

	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/substratusai/substratus/api/v1"
)

var Version = "development"

type Object = client.Object

var FieldManager = "kubectl"

func init() {
	apiv1.AddToScheme(scheme.Scheme)
}

var _ Interface = &Client{}

type Interface interface {
	PortForwardNotebook(ctx context.Context, verbose bool, nb *apiv1.Notebook, ready chan struct{}) error
	Resource(obj Object) (*Resource, error)
	SyncFilesFromNotebook(context.Context, *apiv1.Notebook, string,
		func(file string, complete bool, err error),
	) error
}

func NewClient(inf kubernetes.Interface, cfg *rest.Config) Interface {
	return &Client{Interface: inf, Config: cfg}
}

type Client struct {
	kubernetes.Interface
	Config *rest.Config
}

type Resource struct {
	*resource.Helper
}

func (c *Client) Resource(obj Object) (*Resource, error) {
	// Create a REST mapper that tracks information about the available resources in the cluster.
	groupResources, err := restmapper.GetAPIGroupResources(c.Interface.Discovery())
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

	// Create a client specifically for working with the object.
	restClient, err := newRestClient(c.Config, mapping.GroupVersionKind.GroupVersion())
	if err != nil {
		return nil, err
	}

	helper := resource.NewHelper(restClient, mapping)
	helper.FieldManager = FieldManager
	// helper.FieldValidation = "Strict"

	// Use the REST helper to create the object in the "default" namespace.
	return &Resource{Helper: helper}, nil
}

func newRestClient(restConfig *rest.Config, gv schema.GroupVersion) (rest.Interface, error) {
	// restConfig.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	restConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	restConfig.GroupVersion = &gv
	if len(gv.Group) == 0 {
		restConfig.APIPath = "/api"
	} else {
		restConfig.APIPath = "/apis"
	}

	return rest.RESTClientFor(restConfig)
}

func (r *Resource) WaitReady(ctx context.Context, obj Object) error {
	if err := wait.PollImmediateInfiniteWithContext(ctx, time.Second,
		func(ctx context.Context) (bool, error) {
			fetched, err := r.Get(obj.GetNamespace(), obj.GetName())
			if err != nil {
				return false, err
			}
			readyable, ok := fetched.(interface{ GetStatusReady() bool })
			if !ok {
				return false, fmt.Errorf("object is not readyable: %T", fetched)
			}

			return readyable.GetStatusReady(), nil
		},
	); err != nil {
		return fmt.Errorf("waiting for object to be ready: %w", err)
	}

	return nil
}

func (r *Resource) Watch(ctx context.Context, namespace string, obj Object) (watch.Interface, error) {
	// NOTE: The r.Helper.Watch() method does not support passing a context, calling the code
	// below instead (it was pulled from the Helper implementation).
	w := r.RESTClient.Get().
		NamespaceIfScoped(namespace, r.NamespaceScoped).
		Resource(r.Resource)

	if obj != nil && obj.GetName() != "" {
		w = w.VersionedParams(&metav1.ListOptions{
			ResourceVersion: obj.GetResourceVersion(),
			Watch:           true,
			FieldSelector:   fields.OneTermEqualSelector("metadata.name", obj.GetName()).String(),
		}, metav1.ParameterCodec)
	} else {
		w = w.VersionedParams(&metav1.ListOptions{Watch: true}, metav1.ParameterCodec)
	}

	return w.Watch(ctx)
}
