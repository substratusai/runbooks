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
	"github.com/spf13/cobra"
	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/kubectl/internal/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
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
		build         string
		kubeconfig    string
		filename      string
		namespace     string
		noOpenBrowser bool
		sync          bool
		timeout       time.Duration
	}

	var cmd = &cobra.Command{
		Use:   "notebook [flags] <name>",
		Short: "Start a Jupyter Notebook development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())

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
					return err
				}
				defer os.Remove(tarball.TempDir)

				spin.Stop()
			}

			restConfig, err := clientcmd.BuildConfigFromFlags("", cfg.kubeconfig)
			if err != nil {
				return err
			}

			clientset, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				return err
			}

			c := &client.Client{Interface: clientset, Config: restConfig}
			notebooks, err := c.Resource(&apiv1.Notebook{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "substratus.ai/v1",
					Kind:       "Notebook",
				},
			})
			if err != nil {
				return err
			}

			var obj client.Object
			if cfg.filename != "" {
				manifest, err := os.ReadFile(cfg.filename)
				if err != nil {
					return err
				}
				obj, err = client.Decode(manifest)
				if err != nil {
					return err
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

			if cfg.build != "" {
				if err := client.SetUploadContainerSpec(nb, tarball); err != nil {
					return err
				}
			}

			if err := notebooks.Apply(nb); err != nil {
				return err
			}

			if cfg.build != "" {
				spin.Suffix = " Building: Uploading tarball..."
				spin.Start()

				if err := notebooks.Upload(nb, tarball); err != nil {
					return err
				}

				spin.Stop()
			}

			spin.Suffix = " Waiting for Notebook to be ready..."
			spin.Start()

			waitReadyCtx, cancelWaitReady := context.WithTimeout(ctx, cfg.timeout)
			if err := notebooks.WaitReady(waitReadyCtx, nb); err != nil {
				//cleanup()
				return err
			}
			cancelWaitReady() // Avoid context leak.

			spin.Stop()
			fmt.Println("Notebook: Ready")

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

					if err := c.PortForwardNotebook(portFwdCtx, false, nb, ready); err != nil {
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
			<-serveReady
			spin.Stop()
			fmt.Println("Connection: Ready")

			wg.Wait()

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
	cmd.Flags().BoolVar(&cfg.sync, "sync", false, "Sync local directory with Notebook")
	cmd.Flags().BoolVar(&cfg.noOpenBrowser, "no-open-browser", false, "Do not open the Notebook in a browser")
	cmd.Flags().DurationVarP(&cfg.timeout, "timeout", "t", 20*time.Minute, "Timeout for Notebook to become ready")

	// Add standard kubectl logging flags (for example: -v=2).
	goflags := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(goflags)
	cmd.Flags().AddGoFlagSet(goflags)

	return cmd
}

func defaultNamespace(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}
