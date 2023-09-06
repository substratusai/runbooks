package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/substratusai/substratus/internal/cli/client"
	"github.com/substratusai/substratus/internal/cli/utils"
)

func runCommand() *cobra.Command {
	var flags struct {
		namespace  string
		filename   string
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

		if flags.filename == "" {
			flags.filename = filepath.Join(args[0], defaultFilename)
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

		var obj client.Object
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

		c := NewClient(clientset, restConfig)
		res, err := c.Resource(obj)
		if err != nil {
			return fmt.Errorf("resource client: %w", err)
		}

		// Initialize our program
		p = tea.NewProgram(runModel{
			ctx:       cmd.Context(),
			path:      args[0],
			namespace: namespace,

			object: obj,

			client:   c,
			resource: res,

			uploadProgress: progress.New(progress.WithDefaultGradient()),
			operations:     map[operation]status{},
		})
		if _, err := p.Run(); err != nil {
			return err
		}

		return nil
	}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a Substratus Dataset, Model, or Server",
		Args:  cobra.ExactArgs(1),
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
