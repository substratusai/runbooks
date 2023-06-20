package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type notebookClient struct {
	config     *rest.Config
	dclientset dynamic.Interface
	clientset  kubernetes.Interface
}

func (c *notebookClient) get(ctx context.Context, nb *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.dclientset.Resource(gvr(nb)).Namespace(nb.GetNamespace()).Get(ctx, nb.GetName(), metav1.GetOptions{})
}

func (c *notebookClient) suspend(ctx context.Context, nb *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.patch(ctx, nb, `{"spec":{"suspend":true}}`)
}

func (c *notebookClient) unsuspend(ctx context.Context, nb *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.patch(ctx, nb, `{"spec":{"suspend":false}}`)
}

func (c *notebookClient) patch(ctx context.Context, nb *unstructured.Unstructured, patch string) (*unstructured.Unstructured, error) {
	return c.dclientset.Resource(gvr(nb)).Namespace(nb.GetNamespace()).Patch(ctx, nb.GetName(), types.MergePatchType, []byte(patch), metav1.PatchOptions{})
}

func (c *notebookClient) create(ctx context.Context, nb *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.dclientset.Resource(gvr(nb)).Namespace(nb.GetNamespace()).Create(ctx, nb, metav1.CreateOptions{})
}

func (c *notebookClient) waitAndServe(ctx context.Context, nb *unstructured.Unstructured, ready chan struct{}) error {
	if err := wait.PollImmediateInfiniteWithContext(ctx, time.Second,
		func(ctx context.Context) (bool, error) {
			fmt.Printf(".") // progress bar!

			nb = nb.DeepCopy()
			notebook, err := c.get(ctx, nb)
			if err != nil {
				return false, err
			}
			return hasCondition(notebook, "Ready", "True"), nil
		},
	); err != nil {
		return fmt.Errorf("waiting for notebook to be ready: %w", err)
	}

	// TODO: Pull Pod info from status of Notebook.
	podName, podNamespace := nb.GetName()+"-notebook", nb.GetNamespace()
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		podNamespace, podName)
	hostIP := strings.TrimLeft(c.config.Host, "https://")

	transport, upgrader, err := spdy.RoundTripperFor(c.config)
	if err != nil {
		return err
	}

	// TODO: Remove hardcoding.
	localPort, podPort := 8888, 8888

	// TODO: Add retry on broken connections.
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", localPort, podPort)}, ctx.Done(), ready, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

func gvr(u *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := u.GroupVersionKind()
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: "notebooks",
	}
}
