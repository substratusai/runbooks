package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/substratusai/substratus/internal/cli/utils"
)

func getCommand() *cobra.Command {
	var flags struct {
		namespace  string
		kubeconfig string
	}

	run := func(cmd *cobra.Command, args []string) error {
		// Log to a file. Useful in debugging since you can't really log to stdout.
		// Not required.
		logfilePath := os.Getenv("LOG")
		if logfilePath != "" {
			logFile, err := tea.LogToFile(logfilePath, "simple")
			if err != nil {
				return err
			}
			defer logFile.Close()
		}

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

		c := NewClient(clientset, restConfig)

		var scope string
		if len(args) > 0 {
			scope = args[0]
		}

		// Initialize our program
		p = tea.NewProgram(getModel{
			ctx:       cmd.Context(),
			scope:     scope,
			namespace: namespace,

			client: c,

			objects: newGetObjectMap(),
		})
		if _, err := p.Run(); err != nil {
			return err
		}

		return nil
	}

	cmd := &cobra.Command{
		Use:     "get",
		Aliases: []string{"ls"},
		Short:   "Get Substratus Datasets, Models, Notebooks, and Servers",
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
