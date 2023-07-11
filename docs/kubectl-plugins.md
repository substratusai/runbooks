# Kubectl Plugins

## Notebooks

`kubectl open notebook`

Starts and opens a Jupyter Notebook in the user's browser. Will suspend the Notebook (delete the running Pod) upon cancelling the command.

Examples:

```sh
kubectl open notebook my-nb-name
kubectl open notebook -n my-namespace my-nb-name
kubectl open notebook -f notebook.yaml
```

### Installation

Clone the repo:

```sh
git clone https://github.com/substratusai/substratus && cd substratus
```

Install the binary:

```sh
go build ./kubectl/open-notebook
sudo mv open-notebook /usr/local/bin/kubectl-open-notebook
```

If the plugin installed correctly, you should see it listed as a `kubectl plugin`:

```sh
kubectl plugin list 2>/dev/null | grep kubectl-open-notebook
```
