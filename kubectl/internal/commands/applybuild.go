package commands

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/klog/v2"

	"github.com/spf13/cobra"
	"github.com/substratusai/substratus/kubectl/internal/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func ApplyBuild() *cobra.Command {
	var cfg struct {
		filename       string
		build          string
		kubeconfig     string
		forceConflicts bool
	}

	var cmd = &cobra.Command{
		Use:     "applybuild [flags] BUILD_CONTEXT",
		Args:    cobra.ExactArgs(1),
		Short:   "Apply a Substratus object, upload and build container in-cluster from a local directory",
		Version: Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			client.Version = Version

			ctx := cmd.Context()

			if cfg.filename == "" {
				return fmt.Errorf("-f (--filename) is required")
			}
			cfg.build = args[0]

			tarball, err := client.PrepareImageTarball(ctx, cfg.build)
			if err != nil {
				return fmt.Errorf("preparing tarball: %w", err)
			}
			defer os.Remove(tarball.TempDir)

			restConfig, err := clientcmd.BuildConfigFromFlags("", cfg.kubeconfig)
			if err != nil {
				return fmt.Errorf("rest config: %w", err)
			}

			clientset, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				return fmt.Errorf("clientset: %w", err)
			}

			manifest, err := os.ReadFile(cfg.filename)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}

			obj, err := client.Decode(manifest)
			if err != nil {
				return fmt.Errorf("decoding: %w", err)
			}
			if obj.GetNamespace() == "" {
				// TODO: Add -n flag to specify namespace.
				obj.SetNamespace("default")
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

			if err := r.Apply(obj, cfg.forceConflicts); err != nil {
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
	cmd.Flags().StringVarP(&cfg.kubeconfig, "kubeconfig", "", defaultKubeconfig, "")
	cmd.Flags().StringVarP(&cfg.filename, "filename", "f", "", "Filename identifying the resource to apply and build.")
	cmd.Flags().BoolVar(&cfg.forceConflicts, "force-conflicts", false, "If true, server-side apply will force the changes against conflicts.")

	// Add standard kubectl logging flags (for example: -v=2).
	goflags := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(goflags)
	cmd.Flags().AddGoFlagSet(goflags)

	return cmd
}
