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

		client, err := NewClient(clientset, restConfig)
		if err != nil {
			return fmt.Errorf("client: %w", err)
		}

		// Initialize our program
		tui.P = tea.NewProgram((&tui.ApplyModel{
			Ctx:      cmd.Context(),
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
		Use:     "apply",
		Aliases: []string{"ap"},
		Short:   "Apply Substratus (or any Kubernetes) objects",
		Example: `  # Scan *.yaml files looking for manifests to apply.
  sub apply ./dir/

  # Apply a single manifest file.
  sub apply -f manifests.yaml

  # Apply a remote manifest.
  sub apply -f https://some/manifest.yaml`,
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

	cmd.Flags().StringVarP(&flags.kubeconfig, "kubeconfig", "", defaultKubeconfig, "")
	cmd.Flags().StringVarP(&flags.namespace, "namespace", "n", "", "Namespace of Notebook")
	cmd.Flags().StringVarP(&flags.filename, "filename", "f", "", "Manifest file")

	return cmd
}
