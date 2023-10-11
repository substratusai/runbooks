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

func inferCommand() *cobra.Command {
	var flags struct {
		namespace  string
		kubeconfig string
	}

	run := func(cmd *cobra.Command, args []string) error {
		defer tui.LogFile.Close()

		kubeconfigNamespace, restConfig, err := utils.BuildConfigFromFlags("", flags.kubeconfig)
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

		_ = namespace

		client, err := NewClient(clientset, restConfig)
		if err != nil {
			return fmt.Errorf("client: %w", err)
		}
		_ = client

		// Initialize our program
		// TODO: Use a differnt tui-model for different types of Model objects:
		// ex: Vector-search TUI, Image recongnition TUI
		m := tui.ChatModel{}

		tui.P = tea.NewProgram(m)
		if _, err := tui.P.Run(); err != nil {
			return err
		}

		return nil
	}

	cmd := &cobra.Command{
		Use:     "infer",
		Aliases: []string{"in"},
		Short:   "Run inference against a Substratus Server",
		Args:    cobra.MaximumNArgs(1),
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

	return cmd
}
