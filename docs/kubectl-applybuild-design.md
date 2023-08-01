# Kubectl ApplyBuild Design

Apply-build is intended to combine `kubectl apply` with `docker build` (asynchronously executed remotely in the cluster).

```bash
kubectl applybuild -f MANIFEST_FILE  BUILD_CONTEXT
```

## Basic Usage

Build an image from a manifest and apply it to the current cluster. 

```bash
kubectl applybuild -f ./notebook.yaml .
kubectl applybuild -f ./dataset.yaml .
kubectl applybuild -f ./model.yaml .
kubectl applybuild -f ./server.yaml .
```