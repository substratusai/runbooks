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
		Use:   "applybuild [flags]",
		Short: "Apply a Substratus object, upload and build container in-cluster from a local directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if cfg.filename == "" {
				return fmt.Errorf("-f (--filename) is required")
			}

			tarball, err := client.PrepareImageTarball(cfg.build)
			if err != nil {
				return err
			}
			defer os.Remove(tarball.TempDir)

			restConfig, err := clientcmd.BuildConfigFromFlags("", cfg.kubeconfig)
			if err != nil {
				return err
			}

			clientset, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				return err
			}

			manifest, err := os.ReadFile(cfg.filename)
			if err != nil {
				return err
			}

			obj, err := client.Decode(manifest)
			if err != nil {
				return err
			}
			if obj.GetNamespace() == "" {
				// TODO: Add -n flag to specify namespace.
				obj.SetNamespace("default")
			}

			c := NewClient(clientset, restConfig)
			r, err := c.Resource(obj)
			if err != nil {
				return err
			}

			if err := client.SetUploadContainerSpec(obj, tarball); err != nil {
				return err
			}

			if err := r.Apply(obj, cfg.forceConflicts); err != nil {
				return err
			}

			if err := r.Upload(ctx, obj, tarball); err != nil {
				return err
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
	cmd.Flags().StringVarP(&cfg.build, "build", "b", ".", "Directory with Dockerfile.")
	cmd.Flags().BoolVar(&cfg.forceConflicts, "force-conflicts", false, "If true, server-side apply will force the changes against conflicts.")

	// Add standard kubectl logging flags (for example: -v=2).
	goflags := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(goflags)
	cmd.Flags().AddGoFlagSet(goflags)

	return cmd
}
