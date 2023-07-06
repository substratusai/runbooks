package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
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
	return cp.ToPod(ctx, "./src", "/model/", podForNotebook(nb))
}

func (c *notebookClient) copyFrom(ctx context.Context, nb *unstructured.Unstructured) error {
	return cp.FromPod(ctx, "/model/src", "./src", podForNotebook(nb))
}

func (c *notebookClient) waitReady(ctx context.Context, nb *unstructured.Unstructured) error {
	if err := wait.PollImmediateInfiniteWithContext(ctx, time.Second,
		func(ctx context.Context) (bool, error) {
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

	return nil
}

func podForNotebook(nb *unstructured.Unstructured) types.NamespacedName {
	// TODO: Pull Pod info from status of Notebook.
	return types.NamespacedName{
		Namespace: nb.GetNamespace(),
		Name:      nb.GetName() + "-notebook",
	}
}

type portPair struct {
	containerPort int
	localPort     int
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

	// TODO: Add retry on broken connections.
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})

	var stdout, stderr io.Writer
	if verbose {
		stdout, stderr = os.Stdout, os.Stderr
	} else {
		stdout, stderr = io.Discard, io.Discard
	}

	// TODO: Remove hardcoding.
	portPairs := []portPair{
		{containerPort: 8888, localPort: 8888},
		{containerPort: 8889, localPort: 8889},
	}

	var wg sync.WaitGroup

	for _, pp := range portPairs {
		wg.Add(1) // Increment the WaitGroup counter

		localReady := make(chan struct{}) // Create a separate channel for each goroutine

		go func(pp portPair, ready chan struct{}) {
			defer wg.Done() // Decrement the WaitGroup counter when finished

			fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", pp.localPort, pp.containerPort)}, ctx.Done(), ready, stdout, stderr)
			if err != nil {
				fmt.Printf("Failed to create port forward for port %d: %s\n", pp.containerPort, err.Error())
				close(ready) // Close the ready channel to signal completion
				return
			}

			if err := fw.ForwardPorts(); err != nil {
				fmt.Printf("Failed to forward port %d: %s\n", pp.containerPort, err.Error())
			}

			close(ready) // Close the ready channel to signal completion
		}(pp, localReady)

		go func(ready chan struct{}) {
			<-ready // Wait for the ready channel to be closed
		}(localReady)
	}

	wg.Wait() // Wait for all port forwarding goroutines to complete

	return nil
}

func gvr(u *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := u.GroupVersionKind()
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: "notebooks",
	}
}
