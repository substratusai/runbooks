# Development

## Control Plane

Create a GCP environment.

```sh
make dev-up
```

Run Substratus control plane locally.

```sh
make dev-run
```

Create an example model and server.

```sh
kubectl apply -f examples/facebook-opt-125m/model.yaml
kubectl apply -f examples/facebook-opt-125m/server.yaml
kubectl get pods
# NOTE: Use port 8000 on localhost b/c the controller is likely running locally serving metrics on :8080 which will result in a 404 not found.
kubectl port-forward pod/<pod-name> 8000:8080
open localhost:8000
```

Create an example notebook.

```sh
go build ./kubectl/open-notebook
sudo mv open-notebook /usr/local/bin/kubectl-open-notebook
```

```sh
kubectl open notebook -f examples/facebook-opt-125m/notebook.yaml
```

Fine-tune a new model.

```sh
kubectl apply -f examples/facebook-opt-125m/finetuned-model.yaml
```

Cleanup.

```sh
make dev-down
```

TODO: Automate the cleanup of PVs... Don't forget to manually clean them up for now.

## Kubectl Plugins

### Install from source

You can test out the latest kubectl plugin by building from source directly:
```sh
go build ./kubectl/cmd/notebook && sudo mv notebook /usr/local/bin/kubectl-notebook
go build ./kubectl/cmd/applybuild && sudo mv applybuild /usr/local/bin/kubectl-applybuild
```

### Install using script

Release binaries are created for most architectures when the repo is tagged.
Be aware that moving the binary to your PATH might fail due to permissions
(observed on mac). If it fails, the script will retry the `mv` with `sudo` and
prompt you for your password:

```sh
bash -c "$(curl -fsSL https://raw.githubusercontent.com/substratusai/substratus/main/install/scripts/install_kubectl_plugin.sh)"
```

If the plugin installed correctly, you should see it listed as a `kubectl plugin`:

```sh
kubectl plugin list 2>/dev/null | grep kubectl-notebook
```
