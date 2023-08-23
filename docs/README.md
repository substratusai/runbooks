# Substratus

Substratus is a cross-cloud substrate for training
and serving ML models. It extends the Kubernetes control plane to orchestrate ML
operations through the addition of new custom resources: Model, Server,
Dataset, and Notebook.

We created Substratus because we believe:

* Installing an ML platform should take minutes not weeks.
* Running state of the art LLMs should be single-command-simple.
* Finetuning on your own data should work out of the box.
* Simplicity should not sacrifice on flexibility - Jupyter Notebooks should seamlessly integrate into your workflow.

Learn more on the website:

* [Intro Post](https://www.substratus.ai/blog/introducing-substratus)
* [Overview](https://www.substratus.ai/docs/overview)
* [Architecture](https://www.substratus.ai/docs/architecture)

See what it is about in less than 2 minutes:

[![Watch the video](https://img.youtube.com/vi/CLyXKJHIQ6A/hq2.jpg)](https://youtu.be/CLyXKJHIQ6A)

## Try it out!

Create a local Kubernetes cluster using Kind.

[embedmd]:# (../install/kind/up.sh bash /kind.*/ $)
```bash
kind create cluster --name substratus --config - <<EOF
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 30080
EOF
```

Install Substratus.

```bash
kubectl apply -f https://raw.githubusercontent.com/substratusai/substratus/main/install/kind/manifests.yaml
```

Import a small Open Source LLM.

```bash
kubectl apply -f https://raw.githubusercontent.com/substratusai/substratus/main/examples/facebook-opt-125m/base-model.yaml
```

[embedmd]:# (../examples/facebook-opt-125m/base-model.yaml yaml)
```yaml
apiVersion: substratus.ai/v1
kind: Model
metadata:
  namespace: default
  name: facebook-opt-125m
spec:
  image: substratusai/model-loader-huggingface
  params:
    name: facebook/opt-125m
```

Serve the LLM.

```bash
kubectl apply -f https://raw.githubusercontent.com/substratusai/substratus/main/examples/facebook-opt-125m/base-server.yaml
```

[embedmd]:# (../examples/facebook-opt-125m/base-server.yaml yaml)
```yaml
apiVersion: substratus.ai/v1
kind: Server
metadata:
  name: facebook-opt-125m
spec:
  image: substratusai/model-server-basaran
  model:
    name: facebook-opt-125m
```

Checkout the progress of the Model and the Server.

```bash
kubectl get ai
```

When they report a `Ready` status, start a port-forward.

```bash
kubectl port-forward service/facebook-opt-125m-server 8080:8080
```

Open your browser to [http://localhost:8080/](http://localhost:8080/) or curl the LLM's API.

*PS: Because of the small size of this particular LLM, the answers are more comical than anything.*

```bash
curl http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{ \
    "model": "facebook-opt-125m", \
    "prompt": "Who was the first president of the United States? ", \
    "max_tokens": 10\
  }'
```

Delete the local cluster.

[embedmd]:# (../install/kind/down.sh bash /kind.*/ $)
```bash
kind delete cluster --name substratus
```
