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
    paramsFrom: configMap?
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
2. Create a Notebook that mounts source model
3. `kubectl open notebook notebook-training-experiment`
   * tar up directory respecting dockerignore
   * Get GCS signed url from controller notebook status
   * upload to workspace.tar to signed URL
3. Notebook automatically has all my local directory in the notebook when using the kubectl plugin

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


How to get from Notebook to trainer?

Sync back files from the notebook into local directory
and built into new image.


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
