package gcp_test

import (
	"net/http"
	"os"
	"testing"

	"cloud.google.com/go/compute/metadata"
	"github.com/substratusai/substratus/internal/sci/gcp"
)

func TestServer(t *testing.T) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Skipping test because GOOGLE_APPLICATION_CREDENTIALS is not set")
	}
	server, err := gcp.NewServer()
	if err != nil {
		t.Errorf("Error creating server")
	}

	hc := &http.Client{}
	mc := metadata.NewClient(hc)
	err = server.AutoConfigure(mc)
	if err != nil {
		t.Errorf("Error running AutoConfigure %v", err)
	}
}
