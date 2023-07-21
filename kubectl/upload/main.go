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

func main() {
	var cfg Config

	var uploadCmd = &cobra.Command{
		Use:   "upload [kind] [name]",
		Short: "Upload a resource of a given Kind and Name to be built as a substratus container image",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			cfg.Resource.Kind = args[0]
			cfg.Resource.Name = args[1]
			err := run(cfg)
			if err != nil {
				log.Fatal(err)
			}
		},
	}
	uploadCmd.Flags().StringVarP(&cfg.Path, "path", "p", ".", "Path to the directory to be uploaded")
	uploadCmd.Flags().StringVarP(&cfg.Resource.Namespace, "namespace", "n", "default", "Namespace of the resource created")
	if home := homeDir(); home != "" {
		uploadCmd.Flags().StringVarP(&cfg.Kubeconfig, "kubeconfig", "", filepath.Join(home, ".kube", "config"), "")
	} else {
		uploadCmd.Flags().StringVarP(&cfg.Kubeconfig, "kubeconfig", "", "", "")
	}
	uploadCmd.Flags().BoolVarP(&cfg.Verbose, "verbose", "v", false, "Verbose output")

	var rootCmd = &cobra.Command{Use: "upload"}
	rootCmd.AddCommand(uploadCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cfg Config) error {
	// spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	if !fileExists(filepath.Join(cfg.Path, "Dockerfile")) {
		// Q: in this case do we want to dynamically create or use a pre-existing,
		// minimal dockerfile that works for the given kind? e.g. for a notebook
		// it would likely install python and run jupyter lab --no-browser
		return errors.New("a Dockerfile does not exist at the given path")
	}

	if cfg.Verbose {
		fmt.Println("packaging the directory into a tarball")
	}

	tarPath := "/tmp/archive.tar.gz"
	err := tarGz(cfg.Path, tarPath)
	if err != nil {
		return err
	}
	defer os.Remove(tarPath)

	checksum, err := calculateMD5(tarPath)
	if err != nil {
		return err
	}

	cfg.Resource.Md5Checksum = checksum
	data, err := hex.DecodeString(checksum)
	if err != nil {
		return err
	}
	cfg.Resource.EncodedMd5 = base64.StdEncoding.EncodeToString(data)

	config, _ := clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	customResource := &unstructured.Unstructured{Object: map[string]interface{}{
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

	gvr := schema.GroupVersionResource{
		Group:    "substratus.ai",
		Version:  "v1",
		Resource: strings.ToLower(cfg.Resource.Kind) + "s", // plural is needed here
	}

	result, err := dynamicClient.Resource(gvr).Namespace(cfg.Resource.Namespace).Create(context.Background(), customResource, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Failed to create custom resource: %v", err)
		return err
	}

	watcher, err := dynamicClient.Resource(gvr).Namespace(cfg.Resource.Namespace).Watch(context.Background(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", customResource.GetName()),
	})
	if err != nil {
		log.Fatalf("Failed to watch resource %q: %v", customResource.GetName(), err)
		return err
	}
	if cfg.Verbose {
		fmt.Printf("Created custom resource %q.\n", result.GetName())
	}

	// Monitor the watcher channel
	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Added, watch.Modified:
			updatedResource := event.Object.(*unstructured.Unstructured)

			// Retrieve the value of .status.upload.md5checksum and .status.upload.uploadURL
			status, ok, err := unstructured.NestedMap(updatedResource.Object, "status", "upload")
			if err != nil || !ok {
				continue
			}

			uploadURL, ok := status["uploadURL"].(string)
			if ok && uploadURL != "" {
				watcher.Stop()

				err = uploadTarball(tarPath, uploadURL, cfg.Resource.EncodedMd5)
				if err != nil {
					return err
				}
			}
		case watch.Error:
			watcher.Stop()
			return errors.New("encountered a watch error")
		case watch.Deleted:
			watcher.Stop()
			return errors.New("the custom resource was deleted")
		default:
			watcher.Stop()
			return errors.New("unhandled event type")
		}
	}

	if cfg.Verbose {
		fmt.Printf("Upload is complete. Waiting for the build job to complete.\n")
	}

	// TODO(bjb): should we put a watcher also on name-kind-container-builder or does the utility stop here?
	// spin.Start()
	// spin.Suffix = " Waiting for the build job to complete..."
	// spin.Stop()
	// TODO(bjb): if it's a notebook, wait for it to be ready, then open it in a browser
	return nil
}
