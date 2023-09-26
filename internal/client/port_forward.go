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

type ForwardedPorts struct {
	Local int
	Pod   int
}

func (c *Client) PortForward(ctx context.Context, logger io.Writer, podRef types.NamespacedName, ports ForwardedPorts, ready chan struct{}) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		podRef.Namespace, podRef.Name)
	hostIP := strings.TrimLeft(c.Config.Host, "https://")

	transport, upgrader, err := spdy.RoundTripperFor(c.Config)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})

	var stdout, stderr io.Writer
	if logger != nil {
		stdout, stderr = logger, logger
	} else {
		stdout, stderr = io.Discard, io.Discard
	}
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", ports.Local, ports.Pod)}, ctx.Done(), ready, stdout, stderr)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}
