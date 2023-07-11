# Design

## Models

A Model object represents a logical ML model. The object describes a location
where ML model files can be stored for a specific model. 

Optionally, a loader can be specified to load ML models weights and biases from specific sources.
For example, a commonly used model loader would be the HuggingFace model loader
where by providing the model name would load the model to the Model object
bucket location. Model loaders are defined as container images and can either
point to prebuilt container images or a Git repository with Dockerfile.

A model

The Model object can optionally describe the training parameters including a reference to the Dataset that should be used for training. In this case, the Model object represents the eventual output of a training/finetuning operation.

```yaml
kind: Model
spec:
  container: # used for either loader or trainer
  loader: {} # Optional, when empty an just a location will be created with no files in it
  trainer: {} # Optional, will run a training job
```

### Model Sources

A Model can be sourced from Git or a prebuilt image. Using Git will trigger a container build Job.

```yaml
kind: Model
name: falcon-7b
spec:
  container:
    git:
      url: https://github.com/substratusai/hf-model-loader
    image: substratusai/hf-model-loader
  loader:
    params: {name: tiuu/falcon-7b}
    # optional future
  status:
    url: gcs://my-bucket/my-model/
    files: ["pytorch-001.bin", "config.json"]
    size: 27Gi
```

`params` get converted to environment variables following a specific scheme

The controller will orchestrate the following flow in this case:

<img src="./diagrams/model-building.excalidraw.png" width="80%"></img>


### Model Training

Approach 1: Include in Model object

Models can be trained by specifying the `.spec.training` section.

This will orchestrate a training Job with multiple FUSE mounted buckets.

<img src="./diagrams/model-training.excalidraw.png" width="80%"></img>

```yaml
kind: Model
name: falcon-7b-k8s
spec:
  container:
  # can only specify git or image not both
    git: 
      url: https://github.com/substratusai/hf-llm-trainer
      ref: refs/branch/main
    image: substratusai/hf-llm-peft-trainer # eventually introduce native pytorch trainer

  trainer:
    # baseModel can be left out for creating new base model
    baseModel:
      name: falcon-7b
      namespace: other-namespace
    dataset:
      name: k8s-instructions
      namespace: other-namespace
    resources: {}
    params: {epochs: 1}
```



### Model Training with Notebook
Notebook container contract is to run notebook.sh and serve
notebook compatible HTTP endpoint on port 8888.

User flow: Random experiments

notebook-gpu image has transformers, pytorch, cuda, python 3, etc.. most everything you need to get started.

Example notebook worflow for a user starting with no specific model or dataset in mind. The spec here uses a stock substratus container image:

1. Create Notebook with no sourceModel
2. Load a model dynamically in my notebook session
3. Experiment with the model and training and only save my notebook somewhere for reference

```yaml
kind: Notebook
name: notebooks-for-anything
spec:
  container:
    # or git repo as a source
    image: substratusai/notebook-gpu
  resources: {gpuCount: 4, type: L4}
```


User flow: Iterate on model training code

1. `git clone https://substratusai/hf-llm-trainer && cd hf-llm-trainer`
2. Modify Dockerfile
3. Create a Notebook manifest and apply the Notebook CR mounting a source model
4. `kubectl open notebook -f notebook-training-experiment` - the `kubectl` plugin does each of the following additional steps when `spec.container.local = true`
   * tar up directory respecting dockerignore
   * Get GCS signed url from controller notebook status 
   * upload to workspace.tar to signed URL
5. Notebook automatically has the local directory contents in the notebook when using the `kubectl` plugin (via cp sync operations)

```yaml
kind: Notebook
name: notebook-training-experiment
spec:
  container:
    local: true
  sourceModel: falcon-7b
  resources: {}
status:
    uploadUrl: gs://substratusai-notebooks/workspace.tar
```


User flow: How to get from Notebook to trainer?

1. As `kubectl open notebook ...` is terminated by the user, files will be synced from the Notebook back to the local directory. We can hope but not guarantee this is a git repo.
2. Optional: During `kubectl open notebook ...` termination, a signal will be sent to the controller to build a container image which can serve as the trainer.
3. Potentially the user is prompted for this auto-generation: A `model-${epoch}.yaml` manifest having the values of that trainer will be generated into the repo root having either spec.container.image populated with the build from step 2 OR the repo/branch info taken from the local dir. 

  ```yaml
  kind: Model
  name: falcon-7b-k8s-${epoch_or_branch_name}
  spec:
    container:
      image: my-org/model-falcon-7b-k8s-trainer-${epoch_or_branch_name} # the just-built trainer image
  
    trainer:
      sourceModel:
        name: falcon-7b
        namespace: other-namespace
      dataset: # determine this based on the running notebook - was a single dataset attached to it? In any other case, leave blank `{}` with a comment requiring more
        name: k8s-instructions
        namespace: other-namespace
      resources: {}
      params:
        epochs: 1


### Model Serving
```yaml
kind: ModelServer
spec:
  container:
    image: substratusai/basaran # or git repo
  # model gets mounted to /model/saved
  model: falcon-7b-k8s
  # optional
  quantization:
    bits: 4 # default is 16
  resources: {}
```
