# Kubectl Notebook Design

## Manifests

* Applys Notebook with .container.upload set
* Remote build flow.
* Opens notebook.

```bash
# Open notebook.
kubectl notebook -f notebook.yaml

# Open notebook for other manifest type.
kubectl notebook -f dataset.yaml
kubectl notebook -f model.yaml
kubectl notebook -f server.yaml

# Build local directory and open "notebook.yaml" file in that directory, sync files to/from that notebook.
kubectl notebook -d .

# Build local directory and open notebook.
kubectl notebook -f notebook.yaml -d .

# Build local directory and open notebook, 
kubectl notebook -f notebook.yaml -d . --sync=false
```

## Notebooks from Objects

* Finds notebook.
* Unsuspends notebook.
* Opens notebook.

```bash
kubectl notebook NOTEBOOK_NAME
```

* Applys notebook for object.
* Unsuspends notebook (if it existed).
* Opens notebook.

```bash
kubectl notebook dataset/DATASET_NAME
kubectl notebook model/MODEL_NAME
kubectl notebook server/SERVER_NAME
```

## Local Configuration

Perhaps it makes sense to keep track of what notebooks map to what directories locally.

```
$HOME/.config/substratus/notebooks.yaml
```

```yaml
<path-to-dir>:
  notebook:
    name: abc
    namespace: xyz
  config:
    build: true
    sync: true
```
