# Substratus.AI - Kubernetes ML Platform

<p align="center">
   <a href="https://github.com/substratusai/substratus/actions/workflows/system-tests.yml">
     <img src="https://img.shields.io/github/actions/workflow/status/substratusai/substratus/system-tests.yml?branch=main&label=pipeline&style=flat" alt="continuous integration">
   </a>
    <a href="https://discord.gg/JeXhcmjZVm">
        <img alt="discord-invite" src="https://dcbadge.vercel.app/api/server/JeXhcmjZVm?style=flat">
    </a>
</p>

🚀 Serve popular OSS LLM models in minutes on CPUs or GPUs  
🎵 Fine-tune LLM models with no/low code  
📔 Provide a Colab style seamless Notebook experience  
☁️ Provide a unified ML platform across clouds  
⬆️ Easy to install with minimal dependencies

Support the project by adding a star on GitHub! ❤️



## Quickstart

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

*PS: Because of the small size of this particular LLM, expect comically bad answers to your prompts.*

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

If you want to try out a more capable LLM, running on substantial hardware, try [Kind with
GPU support](https://www.substratus.ai/docs/quickstart/local-kind?kind-mode=gpu),
or try [deploying Substratus in GKE](https://www.substratus.ai/docs/quickstart/gcp).

## Docs
* [Overview](https://www.substratus.ai/docs/overview)
* [Architecture](https://www.substratus.ai/docs/architecture)

## Creators
Feel free to contact any of us:
* [Nick Stogner](https://www.linkedin.com/in/nstogner/)
* [Sam Stoelinga](https://www.linkedin.com/in/samstoelinga/)
