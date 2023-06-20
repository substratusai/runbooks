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

Install with:

```sh
# TODO: Install from GitHub release.
go build ./kubectl/open-notebook && mv open-notebook /usr/local/bin/kubectl-open-notebook
```

