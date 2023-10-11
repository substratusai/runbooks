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

func notebookCommand() *cobra.Command {
	var flags struct {
		resume     string
		namespace  string
		filename   string
		kubeconfig string
		fullscreen bool
	}

	run := func(cmd *cobra.Command, args []string) error {
		defer tui.LogFile.Close()

		//if flags.filename == "" {
		//	defaultFilename := "notebook.yaml"
		//	if _, err := os.Stat(filepath.Join(args[0], "notebook.yaml")); err == nil {
		//		flags.filename = defaultFilename
		//	} else {
		//		return fmt.Errorf("Flag -f (--filename) required when default notebook.yaml file does not exist")
		//	}
		//}

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

		var pOpts []tea.ProgramOption
		if flags.fullscreen {
			pOpts = append(pOpts, tea.WithAltScreen())
		}

		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		// Initialize our program
		tui.P = tea.NewProgram((&tui.NotebookModel{
			Ctx:      cmd.Context(),
			Path:     path,
			Filename: flags.filename,
			Namespace: tui.Namespace{
				Contextual: kubeconfigNamespace,
				Specified:  flags.namespace,
			},
			Client: client,
			K8s:    clientset,
		}).New(), pOpts...)
		if _, err := tui.P.Run(); err != nil {
			return err
		}

		return nil
	}

	cmd := &cobra.Command{
		Use:     "notebook [dir]",
		Aliases: []string{"nb"},
		Short:   "Start a Jupyter Notebook development environment",
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
	cmd.Flags().StringVarP(&flags.filename, "filename", "f", "", "Manifest file")
	cmd.Flags().StringVarP(&flags.resume, "resume", "r", "", "Name of notebook to resume")

	cmd.Flags().BoolVar(&flags.fullscreen, "fullscreen", false, "Fullscreen mode")

	return cmd
}
