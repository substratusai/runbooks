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

Release binaries are created for most architectures when the repo is tagged.
[Identify the release](https://github.com/substratusai/substratus/releases) you
want to use and replace the value of RELEASE in the command below:

```sh
RELEASE="v0.4.0-alpha" wget -qO- $(uname -sm | awk -v release="$RELEASE" '{print "https://github.com/substratusai/substratus/releases/download/" release "/kubectl-open-notebook_" $1 "_" $2 ".tar.gz"}') | tar zxv && \
sudo mv kubectl-open-notebook /usr/local/bin/
```

If the plugin installed correctly, you should see it listed as a `kubectl plugin`:

```sh
kubectl plugin list 2>/dev/null | grep kubectl-open-notebook
```
