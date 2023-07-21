package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/clientcmd"
)

type mockKubernetesClient struct {
	CreateResourceFunc func(gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured, opts metav1.CreateOptions) (*unstructured.Unstructured, error)
	WatchResourceFunc  func(gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions) (watch.Interface, error)
}

func (m *mockKubernetesClient) CreateResource(gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured, opts metav1.CreateOptions) (*unstructured.Unstructured, error) {
	return m.CreateResourceFunc(gvr, namespace, obj, opts)
}

func (m *mockKubernetesClient) WatchResource(gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return m.WatchResourceFunc(gvr, namespace, opts)
}

func TestRun(t *testing.T) {
	cfg := Config{
		Verbose:    true,
		Path:       "./",
		Kubeconfig: clientcmd.RecommendedHomeFile,
		Resource: Resource{
			Kind:        "Notebook",
			Name:        "test",
			Namespace:   "default",
			EncodedMd5:  "",
			Md5Checksum: "",
		},
	}

	// Create a minimal Dockerfile for testing.
	dockerfileContent := `
		FROM ubuntu:18.04

		RUN apt-get update -y && \
			apt-get install -y python3 && \
			mkdir api && echo foo> api/bar.txt

		CMD [ "python3", "-mhttp.server", "--bind=0.0.0.0", "8888" ]
	`
	err := os.WriteFile(filepath.Join(cfg.Path, "Dockerfile"), []byte(dockerfileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}
	defer os.Remove(filepath.Join(cfg.Path, "Dockerfile")) // Clean up after the test.

	client := &mockKubernetesClient{
		CreateResourceFunc: func(gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured, opts metav1.CreateOptions) (*unstructured.Unstructured, error) {
			if obj.GetName() != cfg.Resource.Name {
				t.Fatalf("CreateResource was called with the wrong resource name: got %v, want %v", obj.GetName(), cfg.Resource.Name)
			}
			return obj, nil
		},
		WatchResourceFunc: func(gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
			if opts.FieldSelector != "metadata.name="+cfg.Resource.Name {
				t.Fatalf("WatchResource was called with the wrong field selector: got %v, want %v", opts.FieldSelector, "metadata.name="+cfg.Resource.Name)
			}
			return nil, errors.New("an error")
		},
	}

	err = run(cfg, client)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	expectedError := "failed to watch resource \"test\": an error"
	if err.Error() != expectedError {
		t.Fatalf("Unexpected error: got %v, want %v", err, expectedError)
	}
}
