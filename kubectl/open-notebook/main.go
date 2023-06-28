package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
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

	"k8s.io/apimachinery/pkg/util/runtime"
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
		sync          bool
		verbose       bool
		timeout       time.Duration
	}
	if home := homeDir(); home != "" {
		pflag.StringVar(&flags.kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "Path to the kubeconfig file")
	} else {
		pflag.StringVar(&flags.kubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	}
	pflag.StringVarP(&flags.filename, "filename", "f", "", "Filename of Notebook manifest (i.e. notebook.yaml)")
	pflag.StringVarP(&flags.namespace, "namespace", "n", "default", "Namespace of Notebook")
	pflag.BoolVarP(&flags.verbose, "verbose", "v", false, "Verbose output")
	pflag.BoolVar(&flags.sync, "sync", false, "Sync local directory with Notebook")
	pflag.BoolVar(&flags.noOpenBrowser, "no-open-browser", false, "Do not open the Notebook in a browser")
	pflag.DurationVarP(&flags.timeout, "timeout", "t", 20*time.Minute, "Timeout for Notebook to become ready")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  kubectl open notebook [NAME | -f filename]\n")
		pflag.PrintDefaults()
	}
	pflag.Parse()

	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)

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

	ctx, cancel := context.WithCancel(context.Background())

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

	// Use a new context to avoid using the cancelled one.
	cleanupCtx := context.Background()
	cleanup := func() {
		// Suspend notebook.
		spin.Suffix = " Cleanup: Suspending notebook..."
		spin.Start()
		if _, err := client.suspend(cleanupCtx, notebook); err != nil {
			fmt.Println("Error suspending notebook:", err)
		}
		spin.Stop()
		fmt.Println("Notebook: Suspended")
	}

	spin.Suffix = " Waiting for Notebook to be ready..."
	spin.Start()
	waitReadyCtx, cancelWaitReady := context.WithTimeout(ctx, flags.timeout)
	if err := client.waitReady(waitReadyCtx, notebook); err != nil {
		cleanup()
		log.Fatal(err)
	}
	cancelWaitReady() // Avoid context leak.
	spin.Stop()
	fmt.Println("Notebook: Ready")

	if flags.sync {
		spin.Suffix = " Syncing local directory with Notebook..."
		spin.Start()
		if err := client.copyTo(ctx, notebook); err != nil {
			cleanup()
			log.Fatal(err)
		}
		spin.Stop()
		fmt.Println("Sync: Done")
	}

	serveReady := make(chan struct{})
	go func() {
		defer wg.Done()

		first := true

		for {
			portFwdCtx, cancelPortFwd := context.WithCancel(ctx)
			defer cancelPortFwd() // Avoid a context leak
			runtime.ErrorHandlers = []func(err error){
				func(err error) {
					fmt.Println("Port forward error:", err)
					cancelPortFwd()
				},
			}

			// portForward will close the ready channel when it returns.
			// so we only use the outer ready channel once. On restart of the portForward,
			// we use a new channel.
			var ready chan struct{}
			if first {
				ready = serveReady
			} else {
				ready = make(chan struct{})
			}

			if err := client.portForward(portFwdCtx, flags.verbose, notebook, ready); err != nil {
				//if errors.Is(err, context.Canceled) {
				//	fmt.Println("Serve: stopping: context was cancelled")
				//	return
				//} else {
				fmt.Println("Serve: returned an error: ", err)
				return
				//}
			}

			if err := ctx.Err(); err != nil {
				fmt.Println("Serve: stopping:", err.Error())
				return
			}

			fmt.Println("Restarting port forward")
			cancelPortFwd() // Avoid a context leak
			first = false
		}
	}()

	spin.Suffix = " Waiting for connection to be ready to serve..."
	spin.Start()
	select {
	case <-serveReady:
		break
	}
	spin.Stop()
	fmt.Println("Connection: Ready")

	url := "http://localhost:8888"
	if !flags.noOpenBrowser {
		fmt.Printf("Browser: opening: %s\n", url)
		browser.OpenURL(url)
	} else {
		fmt.Printf("Browser: open to: %s\n", url)
	}

	// Wait for clean shutdown...
	wg.Wait()

	if flags.sync {
		spin.Suffix = " Syncing Notebook to local directory..."
		spin.Start()
		if err := client.copyFrom(cleanupCtx, notebook); err != nil {
			cleanup()
			log.Fatal(err)
		}
		spin.Stop()
		fmt.Println("Sync: Done")
	}

	cleanup()

	return nil
}
