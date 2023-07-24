package main

import (
	"context"
	"fmt"
	"io"
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

	"github.com/substratusai/substratus/kubectl/open-notebook/internal/cp"
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

func (c *notebookClient) copyTo(ctx context.Context, nb *unstructured.Unstructured) error {
	return cp.ToPod(ctx, "./src", "/content/", podForNotebook(nb))
}

func (c *notebookClient) copyFrom(ctx context.Context, nb *unstructured.Unstructured) error {
	return cp.FromPod(ctx, "/content/src", "./src", podForNotebook(nb))
}

func (c *notebookClient) waitReady(ctx context.Context, nb *unstructured.Unstructured) error {
	if err := wait.PollImmediateInfiniteWithContext(ctx, time.Second,
		func(ctx context.Context) (bool, error) {
			nb = nb.DeepCopy()
			notebook, err := c.get(ctx, nb)
			if err != nil {
				return false, err
			}
			return isReady(notebook), nil
		},
	); err != nil {
		return fmt.Errorf("waiting for notebook to be ready: %w", err)
	}

	return nil
}

func podForNotebook(nb *unstructured.Unstructured) types.NamespacedName {
	// TODO: Pull Pod info from status of Notebook.
	return types.NamespacedName{
		Namespace: nb.GetNamespace(),
		Name:      nb.GetName() + "-notebook",
	}
}

func (c *notebookClient) portForward(ctx context.Context, verbose bool, nb *unstructured.Unstructured, ready chan struct{}) error {
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

	var stdout, stderr io.Writer
	if verbose {
		stdout, stderr = os.Stdout, os.Stderr
	} else {
		stdout, stderr = io.Discard, io.Discard
	}
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", localPort, podPort)}, ctx.Done(), ready, stdout, stderr)
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
