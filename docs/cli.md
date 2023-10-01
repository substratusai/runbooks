# CLI

Operates on directories as a unit of work:

```
my-model/
  substratus.yaml <---- Reads Dataset/Model/Server/Notebook from here.
  Dockerfile <--- Optional
  run.ipynb
```

## Alternative Names

### "ai" CLI

Pros

* Short and sweet

Cons

* Generic

```bash
# Looks for "ai.yaml"...

ai notebook .
ai nb .
ai run .
```

### "strat" CLI

Pros

* More specific than `sub`

Cons

* Longer than `sub`

```bash
strat notebook .
strat nb .
strat run .
```

## Notebook

```bash
sub notebook .
sub nb .
```

## Get

```bash
sub get
```

```
sub get

models/
  facebook-opt-125m
  falcon-7b

datasets/
  squad

servers/
  falcon-7b
```

```
sub get models

facebook-opt-125m
falcon-7b
```

```
sub get models/falcon-7b

v3
v2
v1
```

```
sub get models/falcon-7b.v3

metrics:
  loss: 0.9
  abc: 123
```

## Apply

```
# Alternative names???

# "run" --> currently prefer "apply" b/c the target is a noun/end-object (Model / Dataset / Server)
sub run .
```

### Apply (with `<dir>` arg)

* Tar & upload
* Remote build
* Wait for Ready

```bash
sub apply .
```

## View

* Grab `run.html` (converted notebook) and serve on localhost.
* Open browser.

```bash
sub view model/falcon-7b
sub view dataset/squad
```

Alternative names:

```bash
sub logs
sub show
sub inspect
```

## Delete

```bash
sub delete <resource>/<name>
sub del

# By name
sub delete models/facebook-opt-125m
sub delete datasets/squad
```

## Inference Client

```bash
sub infer
sub inf 

# OR:
sub cl (client)
sub ch (chat)     # LLM prompt/completion
sub cl (classify) # Image recognition
```

https://github.com/charmbracelet/bubbletea/tree/master/examples#chat


## Substratus.yaml

### Option 1: Workspace file

A "workspace" file could represent multiple different objects.

```yaml
apiVersion: substratus.ai/v1
metadata:
  name: snakes-opt-125m
model:
  dataset:
    name: snakes
  model:
    name: facebook-opt-125m
dataset:
server:
  # Generated if not specified
  # Only valid if model is specified.
notebook:
  # Generated if not specified
```

### Option 2: Multi-doc

**Pros:**

* No new objects

**Cons:**

* Duplication of fields like `.metadata`

```yaml
apiVersion: substratus.ai/v1
kind: Model
metadata:
  name: snakes-opt-125m
spec:
  dataset:
    name: snakes
  model:
    name: facebook-opt-125m
---
apiVersion: substratus.ai/v1
kind: Server
metadata:
  name: snakes-opt-125m
spec:
  # ...
---
apiVersion: substratus.ai/v1
kind: Notebook
metadata:
  name: snakes-opt-125m
spec:
  # ...
```

### Option 3: Directory

**Pros:**

* No new objects
* Options for more than 1 objects

**Cons:**

* Duplication of fields like `.metadata`
* Messy

```
.substratus/
  dataset.yaml
  model.yaml
  notebook.yaml
  server.yaml

my-code.ipynb
```

```yaml
apiVersion: substratus.ai/v1
kind: Model
metadata:
  name: snakes-opt-125m
spec:
  dataset:
    name: snakes
  model:
    name: facebook-opt-125m
---
apiVersion: substratus.ai/v1
kind: Server
metadata:
  name: snakes-opt-125m
spec:
  # ...
---
apiVersion: substratus.ai/v1
kind: Notebook
metadata:
  name: snakes-opt-125m
spec:
  # ...
```

### Option 4: Kustomize-like

**Pros:**

* No new objects
* Options for more than 1 objects
* Ability to express remote base-model/dataset dependencies

**Cons:**

* Could get messy

```
substratus.yaml

dataset.yaml
model.yaml
notebook.yaml
server.yaml

my-code.ipynb
```

```yaml
Model: model.yaml
dependencies:
- file: ./model.yaml
- https: //raw.githubusercontent.com/substratusai/substratus/main/install/kind/manifests.yaml
- gcs: /some-bucket/some/file.yaml
```

### Option 5: Multi-doc with remotes

```yaml
apiVersion: substratus.ai/v1
kind: Model
metadata:
  name: snakes-opt-125m
spec:
  dataset:
    name: snakes
  model:
    name: facebook-opt-125m
---
- ../base-model.yaml
- https://raw.githubusercontent.com/substratusai/substratus/main/examples/facebook-opt-125m.yaml
- https://raw.githubusercontent.com/substratusai/substratus/main/examples/squad-dataset.yaml
```

