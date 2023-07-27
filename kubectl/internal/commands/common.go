package commands

import (
	"io"
	"os"

	"github.com/substratusai/substratus/kubectl/internal/client"
)

// NewClient is a dirty hack to allow the client to be mocked out in tests. Eww.
var NewClient = client.NewClient

// Stdout is a dirty hack to allow stdout to be inspected in tests. Eww.
var Stdout io.Writer = os.Stdout
