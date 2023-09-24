package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func (c *Client) PortForward(ctx context.Context, logger io.Writer, podRef types.NamespacedName, ready chan struct{}) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		podRef.Namespace, podRef.Name)
	hostIP := strings.TrimLeft(c.Config.Host, "https://")

	transport, upgrader, err := spdy.RoundTripperFor(c.Config)
	if err != nil {
		return err
	}

	// TODO: Use an available local port, or allow it to be overridden.
	localPort, podPort := 8888, 8888

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})

	var stdout, stderr io.Writer
	if logger != nil {
		stdout, stderr = logger, logger
	} else {
		stdout, stderr = io.Discard, io.Discard
	}
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", localPort, podPort)}, ctx.Done(), ready, stdout, stderr)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}
