# Substratus.AI

Substratus is a cross cloud substrate for training and serving AI models. Substratus extends the Kubernetes control plane to orchestrate ML operations through the addition of new API endpoints: Model, ModelServer, Dataset, and Notebook.

## Why Substratus?

* Train and serve models from within your cloud account. Your data stays private.
* Avoid library lock-in and dependency hell through leveraging containers.
* Let substratus calculate your resource requirements and automatically provision GPUs, CPUs, and Memory.
* Adopt best practice conventions by default.
* Train pre-packaged state of the art models on your own datasets.
* Leverage GitOps out of the box.

## Quickstart

Stand up a Kubernetes cluster with Substratus installed.

**TODO: Implement deployment of substratus... this doesnt work yet**

```sh
docker build ./infra -t substratus-infra && docker run -it \
    -e REGION=us-central1 \
    -e ZONE=us-central1-a \
    -e PROJECT=$(gcloud config get project) \
    -e TOKEN=$(gcloud auth print-access-token) \
    substratus-infra gcp-up
```

**TODO: How can we facilitate the user getting their Kubeconfig credentials without adding a step?**

**TODO: Direct the user to a more capable model**

Substratus comes with some popular models pre-installed. Getting an API up and running that serves a model can be done in a single command. NOTE: This will NOT automatically expose the API outside of your cluster.

```sh
kubectl apply -f ./examples/facebook-opt-125m/server.yaml
```

Test out your model.

```sh
kubectl port-forward service <TODO>

curl localhost:8080/generate -d '{"prompt": "Where should I eat for dinner in San Francisco?"}'
```

## Understanding the API

### Model API

The Model API is capable of building base Models from Git repositories, or finetuning existing Models from base Models.

[embedmd]:# (examples/facebook-opt-125m/finetuned-model.yaml)
```yaml
apiVersion: substratus.ai/v1
kind: Model
metadata:
  name: my-model
spec:
  version: v1.0.0
  source:
    modelName: facebook-opt-125m
  training:
    datasetName: favorite-colors
  size:
    runtime: 1.6Gi
    container: 9Gi
```

### ModelServer API

The ModelServer API runs a web server that serves the Model for inference (FUTURE: and tokenization).

[embedmd]:# (examples/facebook-opt-125m/finetuned-server.yaml)
```yaml
apiVersion: substratus.ai/v1
kind: ModelServer
metadata:
  name: my-model-server
spec:
  modelName: my-model
```

### Dataset API

The Dataset API snapshots and locally caches remote datasets to facilitate efficient and reproducable training results. Use Datasets to curate an internal catalog of data available to data scientists with fine-grained access control.

[embedmd]:# (config/base-datasets/favorite-colors.yaml)
```yaml
apiVersion: substratus.ai/v1
kind: Dataset
metadata:
  name: favorite-colors
spec:
  source:
    url: https://raw.githubusercontent.com/substratusai/models/main/facebook-opt-125m/sample-data/favorite-color-blue.jsonl
    filename: fav-colors.jsonl
  size: 1Gi
```

### Notebook API

The Notebook API allows data scientists to quickly spin up a Jupyter Notebook from an existing Model to allow for quick iteration.

Notebooks can be opened using the `kubectl open notebook` command (which is a substratus kubectl plugin).

[embedmd]:# (examples/facebook-opt-125m/notebook.yaml)
```yaml
apiVersion: substratus.ai/v1
kind: Notebook
metadata:
  name: my-notebook
spec:
  modelName: my-model
```
