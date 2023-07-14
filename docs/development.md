# Development

## Remote Development

Create a GCP environment.

```sh
make install
```

Setup controller for running locally.

```sh
# Example only: use your own script.
source ./hack/dev/example-gcp-env.sh
```

Turn off the controller in the cluster.

```sh
kubectl scale deployments -n substratus controller-manager --replicas 0
```

Run controller locally.

```sh
make dev
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
make uninstall
```

TODO: Automate the cleanup of PVs... Don't forget to manually clean them up for now.
