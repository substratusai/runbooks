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
  source: {} # Optional, when empty an just a location will be created with no files in it
  training: {} # Optional, will run a training job
```

### Model Sources

A Model can be sourced from Git or a prebuilt image. Using Git will trigger a container build Job.

```yaml
kind: Model
spec:
  source:
    image: substratusai/hf-model-loader
    git:
      url: https://github.com/substratusai/model-falcon-7b
      path: /optional
      branch: optional
```

The controller will orchestrate the following flow in this case:

<img src="./diagrams/model-building.excalidraw.png" width="80%"></img>

A Model can be sourced from another Model. Because the other Model was already built, the build process can be skipped and the container image can be pulled immediately. This increases the efficiency of use-cases such as applying N-number of new Model objects with different training configurations.

```yaml
kind: Model
spec:
  source:
    model:
      name: base-model-falcon-7b
      namespace: optional
      cluster: possible-future-feature
```

### Model Training

Approach 1: Include in Model object

Models can be trained by specifying the `.spec.training` section.

```yaml
kind: Model
spec:
  training:
    mode: Finetune # Or "Scratch", or ...?
    resources: {}
    params: {}
    dataset:
      name: my-dataset
      namespace: optional
      cluster: possible-future-feature
```

This will orchestrate a training Job with multiple FUSE mounted buckets.

<img src="./diagrams/model-training.excalidraw.png" width="80%"></img>


Approach 2: ModelJob to allow model transformations
```yaml
kind: ModelJob
spec:
  sourceModel: falcon-7b-instruct
  destinationModel: falcon-7b-instruct-k8s # destinationModel gets auto created if not existent yet
  image: substratusai/hf-llm-peft-trainer # eventually introduce native pytorch trainer
  resources: {}
  params: {epochs: 1}
  dataset: k8s-instructions
```
In this case the contaimer image will have falcon-7b-instruct model mounted under `/model/saved`
in read-only model. The destinationModel would be mounted under `/model/trained` using read-write
mode.


### Model Training with Notebook
```yaml
kind: Notebook
spec:
  image: substratus/notebook-gpu
  sourceModel: falcon-7b-instruct
  destinationModel: falcon-7b-instruct-k8s # destinationModel gets auto created if not existent yet
  resources: {}
```

