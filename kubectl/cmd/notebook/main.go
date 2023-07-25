package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/substratusai/substratus/kubectl/internal/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

/*
# Declarative notebooks:

```sh
# Applies notebook to cluster. Opens notebook.
kubectl notebook -f notebook.yaml
```

# Notebooks from other sources:

```
# Creates notebook from Dataset. Opens notebook.
kubectl notebook -f dataset.yaml
kubectl notebook dataset/<name-of-dataset>

# Creates notebook from Model. Opens notebook.
kubectl notebook -f model.yaml
kubectl notebook -f model/<name-of-model>

# Creates notebook from Server. Opens notebook.
kubectl notebook -f server.yaml
kubectl notebook -f server/<name-of-server>
```

# Notebooks that are built from local directory:

New build flag: -b --build

Note: .spec.container is overridden with .spec.container.upload

```
kubectl notebook -b -f notebook.yaml
```

If notebook does NOT exist:

* Creates notebook with .container.upload set
* Remote build flow.
* Opens notebook.

If notebook does exist:

* Finds notebook.
* Prompts user to ask if they want to recreate the notebook (warning: will wipe contents - applicable when we support notebook snapshots).
* Updates .container.upload.md5checksum
* Remote build flow.
* Unsuspends notebook.
* Opens notebook.

# Existing (named) notebooks:

kubectl notebook -n default my-nb-name

* Finds notebook.
* Unsuspends notebook.
* Opens notebook.

# Existing (named) notebooks with build:

kubectl notebook -b -n default my-nb-name

* Finds notebook.
* Prompts user to ask if they want to recreate the notebook (warning: will wipe contents - applicable when we support notebook snapshots).
* Builds notebook.
* Unsuspends notebook.
* Opens notebook.
*/

func main() {
	var cfg struct {
		filename string
	}

	var cmd = &cobra.Command{
		Use:   "notebook [flags] <name>",
		Short: "Start a Jupyter Notebook development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
			if err != nil {
				return err
			}
			clientset, err := kubernetes.NewForConfig(config)
			if err != nil {
				return err

			}

			manifest, err := ioutil.ReadFile(cfg.filename)
			if err != nil {
				return err
			}

			obj, err := client.Decode(manifest)
			if err != nil {
				return err
			}

			_ = obj
			_ = clientset
			return nil
		},
	}

	cmd.Flags().StringVarP(&cfg.filename, "filename", "f", "", "Filename identifying the resource to develop against.")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
