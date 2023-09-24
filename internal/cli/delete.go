package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/substratusai/substratus/internal/client"
	"github.com/substratusai/substratus/internal/cli/utils"
	"github.com/substratusai/substratus/internal/tui"
)

func deleteCommand() *cobra.Command {
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

		objs := map[string]client.Object{}
		var res *client.Resource

		if flags.filename != "" {
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

			objs[obj.GetName()] = obj
			res, err = c.Resource(obj)
			if err != nil {
				return fmt.Errorf("resource client: %w", err)
			}
		}

		// Initialize our program
		tui.P = tea.NewProgram((&tui.DeleteModel{
			Ctx:       cmd.Context(),
			Scope:     args[0],
			Namespace: namespace,

			Objects: objs,

			Client:   c,
			Resource: res,
		}).New())
		if _, err := tui.P.Run(); err != nil {
			return err
		}

		return nil
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del"},
		Short:   "Delete a Substratus Dataset, Model, Server, or Notebook",
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

	return cmd
}
