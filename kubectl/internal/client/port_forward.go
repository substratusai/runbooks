package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func podForNotebook(nb *apiv1.Notebook) types.NamespacedName {
	// TODO: Pull Pod info from status of Notebook.
	return types.NamespacedName{
		Namespace: nb.GetNamespace(),
		Name:      nb.GetName() + "-notebook",
	}
}

func (c *Client) PortForwardNotebook(ctx context.Context, verbose bool, nb *apiv1.Notebook, ready chan struct{}) error {
	// TODO: Pull Pod info from status of Notebook.
	podRef := podForNotebook(nb)
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		podRef.Namespace, podRef.Name)
	hostIP := strings.TrimLeft(c.Config.Host, "https://")

	transport, upgrader, err := spdy.RoundTripperFor(c.Config)
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
