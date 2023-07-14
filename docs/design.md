# Design

## Common Functionality across all Kinds

### Container Images

All Substratus kinds correspond to actions that are taken by containers. These containers can be built
by Substratus (from git, or from an upload) or referenced from external registries.

```yaml
spec:
  image:
    # Only one of the following fields should be set by the user:

    # Optional git source.
    git:
      url: https://github.com/substratusai/hf-model-loader
    
    # Optional upload source.
    upload: {}

    # Optionally specify image name to externally published container image
    # This field will be set by the controller if a build-source (above) was used.
    name: substratusai/hf-model-loader
```

### Resources

```yaml
spec:
  resources:
    gpu:
      count: 8
      type: nvidia-l4
    cpu: 6
    disk: 30 # Gigabytes
    memory: 48 # Gigabytes
```

### Command

Optionally you can override the default command of a container by providing
`command` in the Substratus resource:
```yaml
spec:
  command: ["train.sh"]
```

### Storage

#### Bucket Paths

In Kubernetes the combination of the following fields make up a unique reference to a given object (i.e. database key): `split(.apiVersion, "/")[0] + .kind + .metadata.namespace + .metadata.name`. By storing related artifacts using a similar scheme, restore operations are made trivial: the administrator will not need to worry about backing up the `.status.url` field (as opposed to a scheme where storage location is `.metadata.uid` based - which would be unique for a given in-time instance of an object). When deleting and recreating clusters, if the storage bucket persisted, objects (i.e. Models, Datasets) can simply be re-applied into the new cluster (i.e. [velero](https://velero.io/)/similar is not needed). This plays nicely with GitOps.

Storage scheme:

```
# Models
gs://{project_id}-substratus-storage/namespaces/{namespace}/models/{name}/model
gs://{project_id}-substratus-storage/namespaces/{namespace}/models/{name}/logs

# Datasets
gs://{project_id}-substratus-storage/namespaces/{namespace}/datasets/{name}/data
gs://{project_id}-substratus-storage/namespaces/{namespace}/datasets/{name}/logs
```

NOTE: With this approach, a single bucket could be used to store all Models and Datasets for a given cluster.

Pseudo-reconciler logic for a Model:

```
if .status.url != "" {
  return
}

url := "{bucket}/namespaces/{namespace}/models/{name}"
if sci.bucketObjectExists(url) {
  # Use the bucket as the source of truth if .status.url did not exist.
  updateStatus(url)
  return
}

# ... Looks like the model should be imported/trained/etc.

runJob()
```

#### Mount Points

All mount points will are made under a standardized `/ml` directory which should correspond to the `WORKDIR` of a Dockerfile. This works well for Jupyter notebooks which can be run with `/ml` set as the serving directory: all relevant mounts will be populated on the file explorer sidebar.

The `logs/` directories below are used to store the notebook cell output, python logs, tensorboard stats, etc. These directories are mounted as read-only in Notebooks to explore the output of other background jobs.

##### Dataset (importing)

```
/ml/params.json  # Populated from .spec.params (also available as env vars).

/ml/data/        # Mounted RW: initially empty dir to write new files
/ml/logs/        # Mounted RW: initially empty dir to write new files
```

##### Model (importing)

```
/ml/params.json  # Populated from .spec.params (also available as env vars).

/ml/model/       # Mounted RW: initially empty dir to write new files
/ml/logs/        # Mounted RW: initially empty dir to write new files
```

##### Model (training)

```
/ml/params.json        # Populated from .spec.params (also available as env vars).

/ml/data/              # Mounted RO: from .spec.trainingDataset

/ml/saved-model/       # Mounted RO: from .spec.baseModel

/ml/model/             # Mounted RW: initially empty dir for writing new files
/ml/logs/              # Mounted RW: initially empty dir for writing logs
```

##### Notebook

NOTE: The `/saved-model/` directory is the same as the container for the Model object when `.baseModel` is specified. This allows for easy development of Model training code.

```
/ml/params.json        # Populated from .spec.params (also available as env vars).

/ml/data/              # Mounted RO: from .spec.dataset
/ml/data-logs/         # Mounted RO: from .spec.dataset

/ml/saved-model/       # Mounted RO: from .spec.model
/ml/saved-model-logs/  # Mounted RO: from .spec.model
```

##### Server:

```
/ml/params.json        # Populated from .spec.params (also available as env vars).

/ml/saved-model/       # Mounted RO: from .spec.model
```

## Kind: Model

A Model object represents a logical ML model.

The user specifies all the information needed to either A. Import a model, or B. Train/Finetune
a base model in the `.spec` block.

The controller reports the stored location of the model (bucket URL) in the `.status` block.

### Spec

#### .spec.params

```yaml
spec:
  params:
    abc: xyz # Environment variable will look like: PARAM_ABC=xyz
```

Parameters get converted to environment variables using the following scheme:

`PARAM_{upper(param_key)}={param_value}`

### Status

#### .status.url

This URL is used by the controller when other resources reference this Model by name. The controller can mount Model artifacts into other Model containers for training, into Notebooks for development purposes, or into a Server for loading and serving the Model over HTTP.

```yaml
status:
  url: gs://projectid-substratus-models/82c2706c-b941-4d8d-84a5-8037cf35df82/
```

### Use cases

#### Use case: Importing Huggingface Models

A Model object could specify a Huggingface importer container which would download model weights and biases. The reference to the
Huggingface model is passed in via `.spec.params`.

```yaml
kind: Model
name: falcon-7b
spec:
  image:
    git:
      url: https://github.com/substratusai/model-loader-huggingface
      branch: main
  params: {name: tiiuae/falcon-7b}
  # ...
status:
  url: gcs://my-bucket/my-model/
  files: ["pytorch-001.bin", "config.json"]
  diskSize: 27Gi
```

The controller will orchestrate the following flow in this case:

<img src="./diagrams/model-building.excalidraw.png" width="80%"></img>

#### Use case: Finetuning a Base Model

Models can be trained by specifying the `.spec.baseModel` section. 

```yaml
apiVersion: substratus.ai/v1
kind: Model
metadata:
  name: falcon-7b-k8s
spec:
  image:
    git:
      url: https://github.com/substratusai/model-trainer-huggingface
  baseModel:
    name: falcon-7b
    #namespace: base-models 
  trainingDataset:
    name: k8s-instructions
  params:
    epochs: 1
  resources:
    cpu: 2
    memory: 8
    gpu:
      count: 4
      type: nvidia-l4
```

This will orchestrate a training Job with the base model artifacts FUSE mounted.

<img src="./diagrams/model-training.excalidraw.png" width="80%"></img>

## Kind: Notebook

Notebooks are used for development and experimentation purposes.

### Use cases

#### Use case: Experimenting with a model using Juptyer Notebooks

Example notebook worflow for a user starting with no specific model or dataset in mind. The spec here could use a stock substratus container image.

In this case, the `notebook-gpu` image would have all the `transformers`, `pytorch`, `cuda`, `python 3`, etc. libraries pre-installed.

```yaml
kind: Notebook
metadata:
  name: notebooks-for-anything
spec:
  image:
    name: substratusai/notebook-gpu
  resources:
    gpu:
      count: 4
      type: nvidia-l4
```

#### Use case: Iterate on Model Training Code

In this case, a user might want to update the code used for training in a Model object. The goal here is to create a development environment for the user that exactly mimics the training environment.

Steps:

1. `git clone https://substratusai/hf-llm-trainer && cd hf-llm-trainer`
2. [Optionally] Modify Dockerfile.
3. `kubectl open notebook -f .`
4. The kubectl plugin does the following:
   * Creates a Notebook with `.image.upload` set.
   * Tars local directory respecting `.dockerignore`
5. A Substratus controller in the background:
   * Creates a signed URL for the upload.
   * Updates the Notebook status with the signed URL.
6. The kubectl plugin continues:
   * Uploads tar to signed URL.
7. The Substratus controller is now orchestrating the build of this image using kaniko.
8. After the Notebook is marked as Ready (`.status.ready: true`), the kubectl plugin:
   * Copies local `*.py` files into the running notebook Pod.
   * Fetches a token reported in the Notebook `.status.token` field.
   * Opens browser to `http://localhost:8888?with-token=...`.

```yaml
kind: Notebook
name: notebook-training-experiment
spec:
  image:
    upload: {} # This is how the plugin signals it wants to upload a directory for building.
  model: # Mounts the model. Plugin auto-populated this by finding the corresponding `model.yaml` file.
    name: falcon-7b  
  resources: {...} 
status:
  uploadUrl: https://some-signed-url... # Controller populated this.
  # FUTURE:
  # token: aklsdjfkasdljfs # Jupyter notebook token reported by the controller.
```

#### Future use case: Notebook to Model Trainer

1. As `kubectl open notebook ...` is terminated by the user, files will be synced from the Notebook back to the local directory. We can hope but not guarantee this is a git repo.
2. Optional: During `kubectl open notebook ...` termination, a signal will be sent to the controller to build a container image which can serve as the trainer.
3. Potentially the user is prompted for this auto-generation: A `model-${epoch}.yaml` manifest having the values of that trainer will be generated into the repo root having either spec.container.image populated with the build from step 2 OR the repo/branch info taken from the local dir. 

```yaml
kind: Model
name: falcon-7b-k8s-${epoch_or_branch_name}
spec:
  image:
    name: my-org/model-falcon-7b-k8s-trainer-${epoch_or_branch_name} # the just-built trainer image
  baseModel:
    name: falcon-7b
    #namespace: other-namespace
  trainingDataset: # determine this based on the running notebook - was a single dataset attached to it? In any other case, leave blank `{}` with a comment requiring more
    name: k8s-instructions
    #namespace: other-namespace
  resources: {}
  params:
    epochs: 1
```

## Kind: Server

Servers run Models to serve inference endpoints.

```yaml
kind: Server
spec:
  image:
    image: substratusai/basaran
  # model artifacts get mounted to /model/saved
  model:
    name: falcon-7b-k8s
  # FUTURE:
  resources: {}
```

### Possible Future Features

* Support embeddings?
* Support quantization?

```yaml
spec:
  quantization:
    bits: 4 # default is 16
```
