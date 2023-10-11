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

func runCommand() *cobra.Command {
	var flags struct {
		namespace  string
		filename   string
		kubeconfig string
	}

	run := func(cmd *cobra.Command, args []string) error {
		defer tui.LogFile.Close()

		kubeconfigNamespace, restConfig, err := utils.BuildConfigFromFlags("", flags.kubeconfig)
		if err != nil {
			return fmt.Errorf("rest config: %w", err)
		}

		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return fmt.Errorf("clientset: %w", err)
		}

		client, err := NewClient(clientset, restConfig)
		if err != nil {
			return fmt.Errorf("client: %w", err)
		}

		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		tui.P = tea.NewProgram((&tui.RunModel{
			Ctx:      cmd.Context(),
			Path:     path,
			Filename: flags.filename,
			Namespace: tui.Namespace{
				Contextual: kubeconfigNamespace,
				Specified:  flags.namespace,
			},
			Client: client,
			K8s:    clientset,
		}).New())
		if _, err := tui.P.Run(); err != nil {
			return err
		}

		return nil
	}

	cmd := &cobra.Command{
		Use:   "run [dir]",
		Short: "Run a local directory. Supported kinds: Dataset, Model.",
		Example: `  # Upload code from the current directory,
  # scan *.yaml files looking for Substratus manifests to use.
  sub run

  # Upload modelling code and create a Model.
  sub run -f model.yaml .

  # Upoad dataset importing code and create a Dataset.
  sub run -f dataset.yaml .`,
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
