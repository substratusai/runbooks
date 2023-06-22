# Development

## Remote Development

Create a GCP environment.

```sh
docker build ./infra -t substratus-infra && docker run -it \
    -e REGION=us-central1 \
    -e ZONE=us-central1-a \
    -e PROJECT=$(gcloud config get project) \
    -e TOKEN=$(gcloud auth print-access-token) \
    substratus-infra gcp-up
```

Setup controller for running locally.

```sh
# Example only: use your own script.
source ./hack/dev/nick-gcp.sh
```

Run controller locally.

```sh
make dev
```

Create an example server.

```sh
kubectl apply -f examples/facebook-opt-125m/server.yaml
kubectl get pods
# NOTE: Use port 8000 on localhost b/c the controller is likely running locally serving metrics on :8080 which will result in a 404 not found.
kubectl port-forward pod/<pod-name> 8000:8080
open localhost:8000
```

Create an example notebook.

```sh
go build ./kubectl/open-notebook && mv open-notebook /usr/local/bin/kubectl-open-notebook
```

```sh
kubectl open notebook -f examples/facebook-opt-125m/notebook.yaml
```

Finetune a new model.

```sh
kubectl apply -f examples/facebook-opt-125m/finetuned-model.yaml
```

Cleanup.

```sh
docker build ./infra -t substratus-infra && docker run -it \
    -e REGION=us-central1 \
    -e ZONE=us-central1-a \
    -e PROJECT=$(gcloud config get project) \
    -e TOKEN=$(gcloud auth print-access-token) \
    substratus-infra gcp-down
```

TODO: Automate the cleanup of PVs... Don't forget to manually clean them up for now.

## Remote Deployment

```sh
# Use your project's registry.
export IMAGE=$GCP_REGION-docker.pkg.dev/$GCP_PROJECT_ID/substratus/controller-manager

# Docker build and push image.
make docker-build docker-push IMG=$IMAGE

# Build manifests
make config/install.yaml IMG=$IMAGE

# Edit GPU type as needed.
# Search for "GPU_TYPE" in ./config/install.yaml

# Install on the cluster.
kubectl apply -f ./config/install.yaml
```

## Local Development

**This flow is probably broken right now**

Install a local cluster using kind:
```
kind create cluster
```

Deploy the self-contained registry:
```
kubectl apply -f config/extra/registry.yaml
kubectl patch svc docker-registry -p '{"spec": {"type": "NodePort"}}'
```

Get the node IP address of the kind node:
```
export NODE_IP=$(kubectl get node kind-control-plane -o json | \
  jq -r '.status.addresses[] | select(.type == "InternalIP").address')
```

Get the nodeport of the registry service:
```
export NODE_PORT=$(kubectl get svc docker-registry -o json | jq '.spec.ports[0].nodePort')
```

Verify that you get an HTTP 200 OK response:
```
export IMAGE_REGISTRY=$NODE_IP:$NODE_PORT
curl http://$IMAGE_REGISTRY -v
```

Deploy the DaemonSet that modifies containerd config to allow
bundled registry:
```
cat config/extra/allow-bundled-registry-ds.yaml | envsubst | kubectl apply -f -
```

You should now be able to follow the steps to test it out locally:
```
make install
make run
```

