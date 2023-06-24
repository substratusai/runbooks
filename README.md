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

Stand up a Kubernetes cluster with Substratus installed (for more configuration options see [installation guide](./docs/installation.md).

```sh
docker build ./install -t substratus-installer && \
docker run -it \
  -v $HOME/.kube:/root/.kube \
  -e PROJECT=$(gcloud config get project) \
  -e TOKEN=$(gcloud auth print-access-token) \
  substratus-installer gcp-up.sh
```

Kubectl should now be pointing at your newly created cluster.

Substratus comes with some state of the art models ready to go. For now we will just install a small model to test things out.

```sh
kubectl apply -f ./examples/facebook-opt-125m/model.yaml
```

Run a model server.

```sh
kubectl apply -f ./examples/facebook-opt-125m/server.yaml
```

Test out your model.

```sh
kubectl port-forward service <TODO>
curl localhost:8080/generate -d '{"prompt": "Where should I eat for dinner in San Francisco?"}'
```

Read more about how to fine-tune your model **<TODO: link to longer quickstart>**.

To cleanup all created resources:

```
docker run -it \
  -e PROJECT=$(gcloud config get project) \
  -e TOKEN=$(gcloud auth print-access-token) \
  substratus-installer gcp-down.sh
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
  source:
    modelName: facebook-opt-125m
  training:
    datasetName: favorite-colors
  # TODO: This should be copied from the source Model.
  size:
    parameterBits: 32
    parameterCount: 125000000
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

[embedmd]:# (examples/facebook-opt-125m/dataset.yaml)
```yaml
apiVersion: substratus.ai/v1
kind: Dataset
metadata:
  name: favorite-colors
spec:
  source:
    url: https://raw.githubusercontent.com/substratusai/models/main/facebook-opt-125m/hack/sample-data.jsonl
    filename: fav-colors.jsonl
```

### Notebook API

The Notebook API allows data scientists to quickly spin up a Jupyter Notebook from an existing Model to allow for quick iteration.

Notebooks can be opened using the `kubectl open notebook` command (which is a substratus kubectl plugin). Local directories can be 2-way synced with remote Notebook environments using the `--sync` flag. This allows users to quickly iterate on model source code.

[embedmd]:# (examples/facebook-opt-125m/notebook.yaml)
```yaml
apiVersion: substratus.ai/v1
kind: Notebook
metadata:
  name: nick-fb-opt-125m
spec:
  modelName: facebook-opt-125m
```

