package commands

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	apiv1 "github.com/substratusai/substratus/api/v1"
	"github.com/substratusai/substratus/kubectl/internal/client"
)

func Notebook() *cobra.Command {
	var flags struct {
		dir            string
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

	cmd := &cobra.Command{
		Use:     "notebook [flags] NAME",
		Short:   "Start a Jupyter Notebook development environment",
		Args:    cobra.MaximumNArgs(1),
		Version: Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			client.Version = Version

			ctx, ctxCancel := context.WithCancel(cmd.Context())
			cancel := func() {
				klog.V(1).Info("Context cancelled")
				ctxCancel()
			}
			defer cancel()

			// The -v flag is managed by klog, so we need to check it manually.
			var verbose bool
			if cmd.Flag("v").Changed {
				verbose = true
			}

			if flags.dir != "" {
				if flags.build == "" {
					flags.build = flags.dir
				}
				// If the user specified a directory, we assume they want to sync
				// unless they explicitly set --sync themselves.
				if !cmd.Flag("sync").Changed {
					flags.sync = true
				}
				// If the user specified a directory, we assume they have a notebook.yaml
				// file in their directory unless they explicitly set --filename themselves.
				if !cmd.Flag("filename").Changed {
					flags.filename = filepath.Join(flags.dir, "notebook.yaml")
				}
			}

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigs
				cancel()
			}()

			spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)

			kubeconfigNamespace, restConfig, err := buildConfigFromFlags("", flags.kubeconfig)
			if err != nil {
				return fmt.Errorf("rest config: %w", err)
			}

			namespace := "default"
			if flags.namespace != "" {
				namespace = flags.namespace
			} else if kubeconfigNamespace != "" {
				namespace = kubeconfigNamespace
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
			if len(args) == 1 {
				fetched, err := notebooks.Get(namespace, args[0])
				if err != nil {
					return fmt.Errorf("getting notebook: %w", err)
				}
				obj = fetched.(client.Object)
			} else if flags.filename != "" {
				manifest, err := os.ReadFile(flags.filename)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				obj, err = client.Decode(manifest)
				if err != nil {
					return fmt.Errorf("decoding: %w", err)
				}
				if obj.GetNamespace() == "" {
					// When there is no .metadata.namespace set in the manifest...
					obj.SetNamespace(namespace)
				} else {
					// TODO: Closer match kubectl behavior here by differentiaing between
					// the short -n and long --namespace flags.
					// See example kubectl error:
					// error: the namespace from the provided object "a" does not match the namespace "b". You must pass '--namespace=a' to perform this operation.
					if flags.namespace != "" && flags.namespace != obj.GetNamespace() {
						// When there is .metadata.namespace set in the manifest and
						// a conflicting -n or --namespace flag...
						return fmt.Errorf("the namespace from the provided object %q does not match the namespace %q from flag", obj.GetNamespace(), flags.namespace)
					}
				}
			} else {
				return fmt.Errorf("must specify -f (--filename) flag or NAME argument")
			}

			nb, err := client.NotebookForObject(obj)
			if err != nil {
				return fmt.Errorf("notebook for object: %w", err)
			}
			nb.Spec.Suspend = ptr.To(false)

			var tarball *client.Tarball
			if flags.build != "" {
				spin.Suffix = " Preparing tarball..."
				spin.Start()

				var err error
				tarball, err = client.PrepareImageTarball(ctx, flags.build)
				if err != nil {
					return fmt.Errorf("preparing tarball: %w", err)
				}
				defer os.Remove(tarball.TempDir)

				spin.Stop()
				fmt.Fprintln(NotebookStdout, "Tarball prepared.")

				if err := client.ClearImage(nb); err != nil {
					return fmt.Errorf("clearing image in spec: %w", err)
				}
				if err := client.SetUploadContainerSpec(nb, tarball, NewUUID()); err != nil {
					return fmt.Errorf("setting upload in spec: %w", err)
				}
			}

			if err := notebooks.Apply(nb, flags.forceConflicts); err != nil {
				return fmt.Errorf("applying: %w", err)
			}

			cleanup := func() {
				// Use a new context to avoid using the cancelled one.
				// ctx := context.Background()

				if flags.noSuspend {
					fmt.Fprintln(NotebookStdout, "Skipping notebook suspension, it will keep running.")
				} else {
					// Suspend notebook.
					spin.Suffix = " Suspending notebook..."
					spin.Start()
					_, err := notebooks.Patch(nb.Namespace, nb.Name, types.MergePatchType, []byte(`{"spec": {"suspend": true} }`), &metav1.PatchOptions{})
					spin.Stop()
					if err != nil {
						klog.Errorf("Error suspending notebook: %v", err)
					} else {
						fmt.Fprintln(NotebookStdout, "Notebook suspended.")
					}
				}
			}
			defer cleanup()

			if flags.build != "" {
				spin.Suffix = " Uploading tarball..."
				spin.Start()

				if err := notebooks.Upload(ctx, nb, tarball); err != nil {
					return fmt.Errorf("uploading: %w", err)
				}

				spin.Stop()
				fmt.Fprintln(NotebookStdout, "Tarball uploaded.")
			}

			spin.Suffix = " Waiting for Notebook to be ready..."
			spin.Start()

			waitReadyCtx, cancelWaitReady := context.WithTimeout(ctx, flags.timeout)
			defer cancelWaitReady() // Avoid context leak.
			if err := notebooks.WaitReady(waitReadyCtx, nb); err != nil {
				return fmt.Errorf("waiting for notebook to be ready: %w", err)
			}

			spin.Stop()
			fmt.Fprintln(NotebookStdout, "Notebook ready.")

			var wg sync.WaitGroup

			if flags.sync {
				wg.Add(1)
				go func() {
					defer func() {
						wg.Done()
						klog.V(2).Info("Syncing files from notebook: Done.")
						// Stop other goroutines.
						cancel()
					}()
					if err := c.SyncFilesFromNotebook(ctx, nb, flags.build); err != nil {
						if !errors.Is(err, context.Canceled) {
							klog.Errorf("Error syncing files from notebook: %v", err)
						}
					}
				}()
			}

			serveReady := make(chan struct{})
			wg.Add(1)
			go func() {
				defer func() {
					wg.Done()
					klog.V(2).Info("Port-forwarding: Done.")
					// Stop other goroutines.
					cancel()
				}()

				const maxRetries = 3
				for i := 0; i < maxRetries; i++ {
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
					if i == 0 {
						ready = serveReady
					} else {
						ready = make(chan struct{})
					}

					if err := c.PortForwardNotebook(portFwdCtx, verbose, nb, ready); err != nil {
						klog.Errorf("Port-forward returned an error: %v", err)
					}

					// Check if the command's context is cancelled, if so,
					// avoid restarting the port forward.
					if err := ctx.Err(); err != nil {
						klog.V(1).Infof("Context done, not attempting to restart port-forward: %v", err.Error())
						return
					}

					cancelPortFwd() // Avoid a build up of contexts before returning.
					backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
					klog.V(1).Infof("Restarting port forward (index = %v), after backoff: %s", i, backoff)
					time.Sleep(backoff)
				}
				klog.V(1).Info("Done trying to port-forward")
			}()

			spin.Suffix = " Waiting for connection to be ready to serve..."
			spin.Start()
			select {
			case <-serveReady:
			case <-ctx.Done():
				return fmt.Errorf("context done while waiting on connection to be ready: %w", ctx.Err())
			}
			spin.Stop()
			fmt.Fprintln(NotebookStdout, "Connection ready.")

			// TODO(nstogner): Grab token from Notebook status.
			url := "http://localhost:8888?token=default"
			if !flags.noOpenBrowser {
				fmt.Fprintf(NotebookStdout, "Opening browser to %s\n", url)
				browser.OpenURL(url)
			} else {
				fmt.Fprintf(NotebookStdout, "Open browser to: %s\n", url)
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
	cmd.Flags().StringVarP(&flags.kubeconfig, "kubeconfig", "", defaultKubeconfig, "")

	cmd.Flags().StringVarP(&flags.dir, "dir", "d", "", "Directory to launch the Notebook for. Equivalent to -f <dir>/notebook.yaml -b <dir> -s")
	cmd.Flags().StringVarP(&flags.build, "build", "b", "", "Build the Notebook from this local directory")
	cmd.Flags().StringVarP(&flags.filename, "filename", "f", "", "Filename identifying the resource to develop against.")
	cmd.Flags().BoolVarP(&flags.sync, "sync", "s", false, "Sync local directory with Notebook")

	cmd.Flags().StringVarP(&flags.namespace, "namespace", "n", "", "Namespace of Notebook")

	cmd.Flags().BoolVar(&flags.noSuspend, "no-suspend", false, "Do not suspend the Notebook when exiting")
	cmd.Flags().BoolVar(&flags.forceConflicts, "force-conflicts", true, "If true, server-side apply will force the changes against conflicts.")
	cmd.Flags().BoolVar(&flags.noOpenBrowser, "no-open-browser", false, "Do not open the Notebook in a browser")
	cmd.Flags().DurationVarP(&flags.timeout, "timeout", "t", 20*time.Minute, "Timeout for Notebook to become ready")

	// Add standard kubectl logging flags (for example: -v=2).
	goflags := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(goflags)
	cmd.Flags().AddGoFlagSet(goflags)

	return cmd
}
