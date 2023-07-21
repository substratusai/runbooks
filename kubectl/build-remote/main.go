package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	Verbose    bool
	Path       string
	Kubeconfig string
	TarPath    string
	Resource
}

type Resource struct {
	Kind       string
	Name       string
	Namespace  string
	EncodedMd5 string
	// TODO(bjb): clean this up when the CRD changes we should only pass around
	// encodedMd5s and not hex checksums
	Md5Checksum string
}

type KubernetesClient interface {
	CreateResource(gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured, opts metav1.CreateOptions) (*unstructured.Unstructured, error)
	WatchResource(gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions) (watch.Interface, error)
}

type realKubernetesClient struct {
	dynamicClient dynamic.Interface
}

func (c *realKubernetesClient) CreateResource(gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured, opts metav1.CreateOptions) (*unstructured.Unstructured, error) {
	return c.dynamicClient.Resource(gvr).Namespace(namespace).Create(context.Background(), obj, opts)
}

func (c *realKubernetesClient) WatchResource(gvr schema.GroupVersionResource, namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.dynamicClient.Resource(gvr).Namespace(namespace).Watch(context.Background(), opts)
}

func main() {
	var cfg Config

	var cmd = &cobra.Command{
		Use:   "build-remote [kind] [name]",
		Short: "build-remote packages and uploads a resource of a given Kind and Name to be remotely built by substratus as an image",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			cfg.Resource.Kind = strings.ToTitle(args[0])
			cfg.Resource.Name = args[1]

			config, _ := clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
			dynamicClient, err := dynamic.NewForConfig(config)
			if err != nil {
				log.Fatal(err)
			}

			client := &realKubernetesClient{
				dynamicClient: dynamicClient,
			}

			err = run(cfg, client)
			if err != nil {
				log.Fatal(err)
			}
		},
	}
	cmd.Flags().StringVarP(&cfg.Path, "path", "p", ".", "Path to the directory to be uploaded")
	cmd.Flags().StringVarP(&cfg.Resource.Namespace, "namespace", "n", "default", "Namespace of the resource created")
	if home := homeDir(); home != "" {
		cmd.Flags().StringVarP(&cfg.Kubeconfig, "kubeconfig", "", filepath.Join(home, ".kube", "config"), "")
	} else {
		cmd.Flags().StringVarP(&cfg.Kubeconfig, "kubeconfig", "", "", "")
	}
	cmd.Flags().BoolVarP(&cfg.Verbose, "verbose", "v", false, "Verbose output")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cfg Config, client KubernetesClient) error {
	if !fileExists(filepath.Join(cfg.Path, "Dockerfile")) {
		return errors.New("a Dockerfile does not exist at the given path")
	}

	if cfg.Verbose {
		fmt.Println("packaging the directory into a tarball")
	}
	tmpDir, err := os.MkdirTemp("/tmp", "substratus-kubctl-upload")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg.TarPath = tmpDir + "/archive.tar.gz"
	err = tarGz(cfg.Path, cfg.TarPath)
	if err != nil {
		return fmt.Errorf("failed to create a .tar.gz of the directory: %w", err)
	}
	defer os.Remove(cfg.TarPath)

	checksum, err := calculateMD5(cfg.TarPath)
	if err != nil {
		return fmt.Errorf("failed to calculate the checksum: %w", err)
	}

	cfg.Resource.Md5Checksum = checksum
	data, err := hex.DecodeString(checksum)
	if err != nil {
		return fmt.Errorf("failed to decode hex checksum: %w", err)
	}
	cfg.Resource.EncodedMd5 = base64.StdEncoding.EncodeToString(data)

	customResource := createCustomResource(cfg)
	gvr := schema.GroupVersionResource{
		Group:    "substratus.ai",
		Version:  "v1",
		Resource: strings.ToLower(cfg.Resource.Kind) + "s", // plural is needed here
	}
	result, err := client.CreateResource(gvr, cfg.Resource.Namespace, customResource, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create custom resource: %w", err)
	}

	watcher, err := client.WatchResource(gvr, cfg.Resource.Namespace, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", customResource.GetName()),
	})
	if err != nil {
		return fmt.Errorf("failed to watch resource %q: %w", customResource.GetName(), err)
	}
	if cfg.Verbose {
		fmt.Printf("Created custom resource %q.\n", result.GetName())
	}

	// Monitor the watcher channel
	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Added, watch.Modified:
			err = handleWatchEvent(event, cfg, "upload")
			if err != nil {
				return fmt.Errorf("failed watching the resource: %w", err)
			}
		case watch.Error:
			return errors.New("encountered a watch error")
		case watch.Deleted:
			return errors.New("the custom resource was deleted")
		default:
			return errors.New("unhandled event type")
		}
	}

	if cfg.Verbose {
		fmt.Printf("Upload was successful. Waiting for the build job to complete.\n")
	}
	// TODO(bjb): should we put a watcher also on name-kind-container-builder or does the utility stop here?

	return nil
}

func createCustomResource(cfg Config) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "substratus.ai/v1",
		"kind":       cfg.Resource.Kind,
		"metadata": map[string]interface{}{
			"name": cfg.Resource.Name,
		},
		"spec": map[string]interface{}{
			"image": map[string]interface{}{
				"upload": map[string]interface{}{
					"md5checksum": cfg.Resource.Md5Checksum,
				},
			},
		},
	}}
}

func handleWatchEvent(event watch.Event, cfg Config, phase string) error {
	updatedResource := event.Object.(*unstructured.Unstructured)

	status, ok, err := unstructured.NestedMap(updatedResource.Object, "status")
	if err != nil || !ok {
		return nil
	}

	switch phase {
	case "upload":
		uploadURL, ok := status["uploadURL"].(string)
		if ok && uploadURL != "" {
			err = uploadTarball(cfg.TarPath, uploadURL, cfg.Resource.EncodedMd5)
			if err != nil {
				return fmt.Errorf("tar upload: %w", err)
			}
		}
	case "build":
		ready, ok := status["ready"].(bool)
		if ok && ready {
			fmt.Printf("Build job is complete.\n")
			return nil
		}
	}

	return nil
}
