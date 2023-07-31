# Development

## Remote Development

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
