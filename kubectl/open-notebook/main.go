package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	// NOTE: The above auth import does not work for GKE 1.26 and up.
	// https://cloud.google.com/blog/products/containers-kubernetes/kubectl-auth-changes-in-gke
	//_ "github.com/substratusai/substratus/kubectl/open-notebook/internal/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/pkg/browser"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var flags struct {
		kubeconfig    string
		filename      string
		namespace     string
		noOpenBrowser bool
		timeout       time.Duration
	}
	if home := homeDir(); home != "" {
		pflag.StringVar(&flags.kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "Path to the kubeconfig file")
	} else {
		pflag.StringVar(&flags.kubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	}
	pflag.StringVarP(&flags.filename, "filename", "f", "", "Filename of Notebook manifest (i.e. notebook.yaml)")
	pflag.StringVarP(&flags.namespace, "namespace", "n", "default", "Namespace of Notebook")
	pflag.BoolVar(&flags.noOpenBrowser, "no-open-browser", false, "Do not open the Notebook in a browser")
	pflag.DurationVarP(&flags.timeout, "timeout", "t", 10*time.Minute, "Timeout for Notebook to become ready")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  kubectl open notebook [NAME | -f filename]\n")
		pflag.PrintDefaults()
	}
	pflag.Parse()

	notebookName := pflag.Arg(0)

	if flags.filename == "" && notebookName == "" {
		return fmt.Errorf("must provide filename flag (-f) or notebook name")
	}

	config, err := clientcmd.BuildConfigFromFlags("", flags.kubeconfig)
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err

	}
	dclientset, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), flags.timeout)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	client := &notebookClient{
		config:     config,
		clientset:  clientset,
		dclientset: dclientset,
	}

	var notebook *unstructured.Unstructured
	if flags.filename != "" {
		var err error
		notebook, err = readManifest(flags.filename)
		if err != nil {
			return err
		}

		if notebook.GetNamespace() == "" {
			notebook.SetNamespace(flags.namespace)
		} else if notebook.GetNamespace() != flags.namespace {
			return fmt.Errorf("namespace in manifest does not match namespace flag")
		}

		fetchedNotebook, err := client.get(ctx, notebook)
		if err != nil {
			if apierrors.IsNotFound(err) {
				unsuspend(notebook)
				notebook, err = client.create(ctx, notebook)
				if err != nil {
					return fmt.Errorf("creating notebook: %w", err)
				}
			} else {
				return fmt.Errorf("getting notebook in namespace %v: %w", flags.namespace, err)
			}
		} else {
			notebook = fetchedNotebook
			if isSuspended(notebook) {
				notebook, err = client.unsuspend(ctx, notebook)
				if err != nil {
					return fmt.Errorf("unsuspending notebook: %w", err)
				}
			}
		}
	} else {
		notebook = &unstructured.Unstructured{}
		notebook.SetAPIVersion("substratus.ai/v1")
		notebook.SetKind("Notebook")
		notebook.SetName(notebookName)
		notebook.SetNamespace(flags.namespace)

		notebook, err = client.get(ctx, notebook)
		if err != nil {
			return fmt.Errorf("getting notebook in namespace %v: %w", flags.namespace, err)
		}
		if isSuspended(notebook) {
			notebook, err = client.unsuspend(ctx, notebook)
			if err != nil {
				return fmt.Errorf("unsuspending notebook: %w", err)
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)

	ready := make(chan struct{})

	cleanup := func() {
		// Use a new context to avoid using the cancelled one.
		cleanupCtx := context.Background()

		// Suspend notebook.
		fmt.Println("Cleanup: Suspending notebook...")
		if _, err := client.suspend(cleanupCtx, notebook); err != nil {
			fmt.Println("Error suspending notebook: %w", err)
		}
	}

	go func() {
		defer wg.Done()
		if err := client.waitAndServe(ctx, notebook, ready); err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) {
				os.Exit(0)
			} else {
				log.Fatal(err)
			}
		}
	}()

	fmt.Print("Waiting for Notebook to be ready...\n")
	select {
	case <-ready:
		break
	}
	fmt.Println("\nNotebook: Ready")

	url := "http://localhost:8888"
	if !flags.noOpenBrowser {
		fmt.Printf("Opening browser: %s\n", url)
		browser.OpenURL(url)
	} else {
		fmt.Printf("Notebook serving on: %s\n", url)
	}

	fmt.Println("Waiting for clean shutdown...")
	wg.Wait()

	cleanup()

	return nil
}
