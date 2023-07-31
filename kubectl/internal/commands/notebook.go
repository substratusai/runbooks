package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/kubectl/internal/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

/*
# Declarative notebooks:

```sh
# Applies notebook to cluster. Opens notebook.
kubectl notebook -f notebook.yaml
```

# Notebooks from other sources:

```
# Creates notebook from Dataset. Opens notebook.
kubectl notebook -f dataset.yaml
kubectl notebook dataset/<name-of-dataset>

# Creates notebook from Model. Opens notebook.
kubectl notebook -f model.yaml
kubectl notebook -f model/<name-of-model>

# Creates notebook from Server. Opens notebook.
kubectl notebook -f server.yaml
kubectl notebook -f server/<name-of-server>
```

# Notebooks that are built from local directory:

New build flag: -b --build

Note: .spec.container is overridden with .spec.container.upload

```
kubectl notebook -b -f notebook.yaml
```

If notebook does NOT exist:

* Creates notebook with .container.upload set
* Remote build flow.
* Opens notebook.

If notebook does exist:

* Finds notebook.
* Prompts user to ask if they want to recreate the notebook (warning: will wipe contents - applicable when we support notebook snapshots).
* Updates .container.upload.md5checksum
* Remote build flow.
* Unsuspends notebook.
* Opens notebook.

# Existing (named) notebooks:

kubectl notebook -n default my-nb-name

* Finds notebook.
* Unsuspends notebook.
* Opens notebook.

# Existing (named) notebooks with build:

kubectl notebook -b -n default my-nb-name

* Finds notebook.
* Prompts user to ask if they want to recreate the notebook (warning: will wipe contents - applicable when we support notebook snapshots).
* Builds notebook.
* Unsuspends notebook.
* Opens notebook.
*/

func Notebook() *cobra.Command {
	var cfg struct {
		build          string
		kubeconfig     string
		filename       string
		namespace      string
		noOpenBrowser  bool
		sync           bool
		forceConflicts bool
		noSuspend      bool
		timeout        time.Duration
	}

	var cmd = &cobra.Command{
		Use:   "notebook [flags] <name>",
		Short: "Start a Jupyter Notebook development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigs
				cancel()
			}()

			spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)

			var tarball *client.Tarball
			if cfg.build != "" {
				spin.Suffix = " Building: Preparing tarball..."
				spin.Start()

				var err error
				tarball, err = client.PrepareImageTarball(cfg.build)
				if err != nil {
					return fmt.Errorf("preparing tarball: %w", err)
				}
				defer os.Remove(tarball.TempDir)

				spin.Stop()
				fmt.Fprintln(NotebookStdout, "Building: Prepared")
			}

			restConfig, err := clientcmd.BuildConfigFromFlags("", cfg.kubeconfig)
			if err != nil {
				return fmt.Errorf("rest config: %w", err)
			}

			clientset, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				return fmt.Errorf("clientset: %w", err)
			}

			c := NewClient(clientset, restConfig)
			notebooks, err := c.Resource(&apiv1.Notebook{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "substratus.ai/v1",
					Kind:       "Notebook",
				},
			})
			if err != nil {
				return fmt.Errorf("resource client: %w", err)
			}

			var obj client.Object
			if cfg.filename != "" {
				manifest, err := os.ReadFile(cfg.filename)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				obj, err = client.Decode(manifest)
				if err != nil {
					return fmt.Errorf("decoding: %w", err)
				}
				if obj.GetNamespace() == "" {
					// TODO: Add -n flag to specify namespace.
					obj.SetNamespace("default")
				}
			} else if len(args) == 1 {
				fetched, err := notebooks.Get(defaultNamespace(cfg.namespace), args[0])
				if err != nil {
					return fmt.Errorf("getting notebook: %w", err)
				}
				obj = fetched.(client.Object)
			} else {
				return fmt.Errorf("must specify -f (--filename) or <name>")
			}

			nb, err := client.NotebookForObject(obj)
			if err != nil {
				return fmt.Errorf("notebook for object: %w", err)
			}
			nb.Spec.Suspend = ptr.To(false)

			if cfg.build != "" {
				if err := client.ClearImage(obj); err != nil {
					return fmt.Errorf("clearing image in spec: %w", err)
				}
				if err := client.SetUploadContainerSpec(nb, tarball, NewUUID()); err != nil {
					return fmt.Errorf("setting upload in spec: %w", err)
				}
			}

			if err := notebooks.Apply(nb, cfg.forceConflicts); err != nil {
				return fmt.Errorf("applying: %w", err)
			}

			cleanup := func() {
				// Use a new context to avoid using the cancelled one.
				//ctx := context.Background()

				if cfg.noSuspend {
					fmt.Fprintln(NotebookStdout, "Cleanup: Skipping notebook suspension, it will keep running.")
				} else {
					// Suspend notebook.
					spin.Suffix = " Cleanup: Suspending notebook..."
					spin.Start()
					if _, err := notebooks.Patch(nb.Namespace, nb.Name, types.MergePatchType, []byte(`{"spec": {"suspend": true} }`), &metav1.PatchOptions{}); err != nil {
						fmt.Fprintf(NotebookStdout, "Cleanup: Error suspending notebook: %v\n", err)
					}
					spin.Stop()
					fmt.Fprintln(NotebookStdout, "Cleanup: Suspended")
				}
			}
			defer cleanup()

			if cfg.build != "" {
				spin.Suffix = " Building: Uploading tarball..."
				spin.Start()

				if err := notebooks.Upload(ctx, nb, tarball); err != nil {
					return fmt.Errorf("uploading: %w", err)
				}

				spin.Stop()
				fmt.Fprintln(NotebookStdout, "Building: Uploaded")
			}

			spin.Suffix = " Waiting for Notebook to be ready..."
			spin.Start()

			waitReadyCtx, cancelWaitReady := context.WithTimeout(ctx, cfg.timeout)
			defer cancelWaitReady() // Avoid context leak.
			if err := notebooks.WaitReady(waitReadyCtx, nb); err != nil {
				return fmt.Errorf("waiting for notebook to be ready: %w", err)
			}

			spin.Stop()
			fmt.Fprintln(NotebookStdout, "Notebook: Ready")

			var wg sync.WaitGroup

			serveReady := make(chan struct{})
			wg.Add(1)
			go func() {
				defer wg.Done()

				first := true
				for {
					portFwdCtx, cancelPortFwd := context.WithCancel(ctx)
					defer cancelPortFwd() // Avoid a context leak
					runtime.ErrorHandlers = []func(err error){
						func(err error) {
							// Cancel a broken port forward to attempt to restart the port-forward.
							klog.Errorf("Port-forward error: %v", err)
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

					if err := c.PortForwardNotebook(portFwdCtx, true, nb, ready); err != nil {
						klog.Errorf("Port-forward returned an error: %v", err)
						return
					}

					// Check if the command's context is cancelled, if so,
					// avoid restarting the port forward.
					if err := ctx.Err(); err != nil {
						klog.V(1).Infof("Context done, not attempting to restart port-forward: %v", err.Error())
						return
					}

					cancelPortFwd() // Avoid a build up of contexts before returning.
					klog.V(1).Info("Restarting port forward")
					first = false
				}
			}()

			spin.Suffix = " Waiting for connection to be ready to serve..."
			spin.Start()
			select {
			case <-serveReady:
			case <-ctx.Done():
				return fmt.Errorf("context done while waiting on connection to be ready: %w", ctx.Err())
			}
			spin.Stop()
			fmt.Fprintln(NotebookStdout, "Connection: Ready")

			// TODO(nstogner): Grab token from Notebook status.
			url := "http://localhost:8888?token=default"
			if !cfg.noOpenBrowser {
				fmt.Fprintf(NotebookStdout, "Browser: opening: %s\n", url)
				browser.OpenURL(url)
			} else {
				fmt.Fprintf(NotebookStdout, "Browser: open to: %s\n", url)
			}

			klog.V(2).Info("Waiting for routines to complete before exiting")
			wg.Wait()
			klog.V(2).Info("Routines completed, exiting")

			return nil
		},
	}

	defaultKubeconfig := os.Getenv("KUBECONFIG")
	if defaultKubeconfig == "" {
		defaultKubeconfig = clientcmd.RecommendedHomeFile
	}
	cmd.Flags().StringVarP(&cfg.kubeconfig, "kubeconfig", "", defaultKubeconfig, "")
	cmd.Flags().StringVarP(&cfg.build, "build", "b", "", "Build the Notebook from this local directory")
	cmd.Flags().StringVarP(&cfg.filename, "filename", "f", "", "Filename identifying the resource to develop against.")
	cmd.Flags().StringVarP(&cfg.namespace, "namespace", "n", "default", "Namespace of Notebook")
	cmd.Flags().BoolVar(&cfg.noSuspend, "no-suspend", false, "Do not suspend the Notebook when exiting")
	cmd.Flags().BoolVar(&cfg.sync, "sync", false, "Sync local directory with Notebook")
	cmd.Flags().BoolVar(&cfg.forceConflicts, "force-conflicts", true, "If true, server-side apply will force the changes against conflicts.")
	cmd.Flags().BoolVar(&cfg.noOpenBrowser, "no-open-browser", false, "Do not open the Notebook in a browser")
	cmd.Flags().DurationVarP(&cfg.timeout, "timeout", "t", 20*time.Minute, "Timeout for Notebook to become ready")

	// Add standard kubectl logging flags (for example: -v=2).
	goflags := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(goflags)
	cmd.Flags().AddGoFlagSet(goflags)

	return cmd
}
