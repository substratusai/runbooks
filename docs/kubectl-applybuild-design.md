# Kubectl ApplyBuild Design

```bash
kubectl applybuild -f MANIFEST_FILE  BUILD_CONTEXT
```

## Basic Usage

```bash
kubectl applybuild -f ./notebook.yaml .
kubectl applybuild -f ./dataset.yaml .
kubectl applybuild -f ./model.yaml .
kubectl applybuild -f ./server.yaml .
```
