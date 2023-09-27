package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/substratusai/substratus/internal/cli/utils"
	"github.com/substratusai/substratus/internal/tui"
)

func applyCommand() *cobra.Command {
	var flags struct {
		namespace  string
		filename   string
		kubeconfig string
	}

	run := func(cmd *cobra.Command, args []string) error {
		defer tui.LogFile.Close()

		if flags.filename == "" {
			return fmt.Errorf("Flag -f (--filename) required")
		}

		kubeconfigNamespace, restConfig, err := utils.BuildConfigFromFlags("", flags.kubeconfig)
		if err != nil {
			return fmt.Errorf("rest config: %w", err)
		}

		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return fmt.Errorf("clientset: %w", err)
		}

		tui.P = tea.NewProgram((&tui.ApplyModel{
			Ctx:      cmd.Context(),
			Path:     args[0],
			Filename: flags.filename,
			Namespace: tui.Namespace{
				Contextual: kubeconfigNamespace,
				Specified:  flags.namespace,
			},
			Client: NewClient(clientset, restConfig),
			K8s:    clientset,
		}).New())
		if _, err := tui.P.Run(); err != nil {
			return err
		}

		return nil
	}

	cmd := &cobra.Command{
		Use:     "apply [dir]",
		Aliases: []string{"apl"},
		Short:   "Apply a manifest, optionally build a container image from a directory.",
		Example: `  # Upload modelling code and create a Model.
  sub apply -f model.yaml .

  # Upoad dataset importing code and create a Dataset.
  sub apply -f dataset.yaml .

  # NOT YET IMPLEMENTED: Upload code from a local directory,
  # scan *.yaml files looking for Substratus manifests to use.
  #sub apply .

  # NOT YET IMPLEMENTED: Apply a local manifest file.
  #sub apply -f manifest.yaml

  # NOT YET IMPLEMENTED: Apply a remote manifest file.
  #sub apply -f https://some-place/manifest.yaml`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(cmd, args); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}

	defaultKubeconfig := os.Getenv("KUBECONFIG")
	if defaultKubeconfig == "" {
		defaultKubeconfig = clientcmd.RecommendedHomeFile
	}
	cmd.Flags().StringVarP(&flags.kubeconfig, "kubeconfig", "", defaultKubeconfig, "path to Kubernetes Kubeconfig file")
	cmd.Flags().StringVarP(&flags.namespace, "namespace", "n", "", "namespace of Notebook")
	cmd.Flags().StringVarP(&flags.filename, "filename", "f", "", "manifest file")

	return cmd
}
