package commands

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/substratusai/substratus/kubectl/internal/client"
)

func ApplyBuild() *cobra.Command {
	var flags struct {
		namespace      string
		filename       string
		build          string
		kubeconfig     string
		forceConflicts bool
	}

	cmd := &cobra.Command{
		Use:     "applybuild [flags] BUILD_CONTEXT",
		Args:    cobra.ExactArgs(1),
		Short:   "Apply a Substratus object, upload and build container in-cluster from a local directory",
		Version: Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			client.Version = Version

			ctx := cmd.Context()

			if flags.filename == "" {
				return fmt.Errorf("-f (--filename) is required")
			}
			flags.build = args[0]

			tarball, err := client.PrepareImageTarball(ctx, flags.build)
			if err != nil {
				return fmt.Errorf("preparing tarball: %w", err)
			}
			defer os.Remove(tarball.TempDir)

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

			manifest, err := os.ReadFile(flags.filename)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}

			obj, err := client.Decode(manifest)
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

			c := NewClient(clientset, restConfig)
			r, err := c.Resource(obj)
			if err != nil {
				return fmt.Errorf("resource client: %w", err)
			}

			if err := client.SetUploadContainerSpec(obj, tarball, NewUUID()); err != nil {
				return fmt.Errorf("setting upload in spec: %w", err)
			}
			if err := client.ClearImage(obj); err != nil {
				return fmt.Errorf("clearing image: %w", err)
			}

			if err := r.Apply(obj, flags.forceConflicts); err != nil {
				return fmt.Errorf("applying: %w", err)
			}

			if err := r.Upload(ctx, obj, tarball); err != nil {
				return fmt.Errorf("uploading: %w", err)
			}

			return nil
		},
	}

	defaultKubeconfig := os.Getenv("KUBECONFIG")
	if defaultKubeconfig == "" {
		defaultKubeconfig = clientcmd.RecommendedHomeFile
	}
	cmd.Flags().StringVarP(&flags.kubeconfig, "kubeconfig", "", defaultKubeconfig, "")
	cmd.Flags().StringVarP(&flags.filename, "filename", "f", "", "Filename identifying the resource to apply and build.")
	cmd.Flags().BoolVar(&flags.forceConflicts, "force-conflicts", false, "If true, server-side apply will force the changes against conflicts.")

	cmd.Flags().StringVarP(&flags.namespace, "namespace", "n", "", "Namespace of Notebook")

	// Add standard kubectl logging flags (for example: -v=2).
	goflags := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(goflags)
	cmd.Flags().AddGoFlagSet(goflags)

	return cmd
}
