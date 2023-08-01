package commands

import (
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/substratusai/substratus/kubectl/internal/client"
)

// NewClient is a dirty hack to allow the client to be mocked out in tests.
var NewClient = client.NewClient

// NotebookStdout is a dirty hack to allow stdout to be inspected in tests.
var NotebookStdout io.Writer = os.Stdout

var NewUUID = func() string {
	return uuid.New().String()
}

func defaultNamespace(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}
