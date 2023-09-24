package cli

import (
	"fmt"
	"os"
	"path/filepath"

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

		if flags.filename == "" {
			defaultFilename := "notebook.yaml"

			if _, err := os.Stat(filepath.Join(args[0], "notebook.yaml")); err == nil {
				flags.filename = defaultFilename
			} else {
				return fmt.Errorf("Flag -f (--filename) required when default notebook.yaml file does not exist")
			}
		}

		kubeconfigNamespace, restConfig, err := utils.BuildConfigFromFlags("", flags.kubeconfig)
		if err != nil {
			return fmt.Errorf("rest config: %w", err)
		}

		//namespace := "default"
		//if flags.namespace != "" {
		//	namespace = flags.namespace
		//} else if kubeconfigNamespace != "" {
		//	namespace = kubeconfigNamespace
		//}

		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return fmt.Errorf("clientset: %w", err)
		}

		c := NewClient(clientset, restConfig)
		//notebooks, err := c.Resource(&apiv1.Notebook{
		//	TypeMeta: metav1.TypeMeta{
		//		APIVersion: "substratus.ai/v1",
		//		Kind:       "Notebook",
		//	},
		//})
		//if err != nil {
		//	return fmt.Errorf("resource client: %w", err)
		//}

		//var obj client.Object
		//if flags.resume != "" {
		//	fetched, err := notebooks.Get(namespace, flags.resume)
		//	if err != nil {
		//		return fmt.Errorf("getting notebook: %w", err)
		//	}
		//	obj = fetched.(client.Object)
		//} else {
		//	manifest, err := os.ReadFile(flags.filename)
		//	if err != nil {
		//		return fmt.Errorf("reading file: %w", err)
		//	}
		//	obj, err = client.Decode(manifest)
		//	if err != nil {
		//		return fmt.Errorf("decoding: %w", err)
		//	}
		//	if obj.GetNamespace() == "" {
		//		// When there is no .metadata.namespace set in the manifest...
		//		obj.SetNamespace(namespace)
		//	} else {
		//		// TODO: Closer match kubectl behavior here by differentiaing between
		//		// the short -n and long --namespace flags.
		//		// See example kubectl error:
		//		// error: the namespace from the provided object "a" does not match the namespace "b". You must pass '--namespace=a' to perform this operation.
		//		if flags.namespace != "" && flags.namespace != obj.GetNamespace() {
		//			// When there is .metadata.namespace set in the manifest and
		//			// a conflicting -n or --namespace flag...
		//			return fmt.Errorf("the namespace from the provided object %q does not match the namespace %q from flag", obj.GetNamespace(), flags.namespace)
		//		}
		//	}
		//}

		//nb, err := client.NotebookForObject(obj)
		//if err != nil {
		//	return fmt.Errorf("notebook for object: %w", err)
		//}
		//nb.Spec.Suspend = ptr.To(false)

		var pOpts []tea.ProgramOption
		if flags.fullscreen {
			pOpts = append(pOpts, tea.WithAltScreen())
		}

		// Initialize our program
		tui.P = tea.NewProgram((&tui.NotebookModel{
			Ctx:      cmd.Context(),
			Path:     args[0],
			Filename: flags.filename,
			Namespace: tui.Namespace{
				Contextual: kubeconfigNamespace,
				Specified:  flags.namespace,
			},
			Client: c,
			K8s:    clientset,
		}).New(), pOpts...)
		if _, err := tui.P.Run(); err != nil {
			return err
		}

		return nil
	}

	cmd := &cobra.Command{
		Use:     "notebook",
		Aliases: []string{"nb"},
		Short:   "Start a Jupyter Notebook development environment",
		Args:    cobra.ExactArgs(1),
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
