# Design

## Models

A Model object represents a logical ML model.

The object describes where the source code for the model should come from. This code will be stored in a container image. That container image will initially be run in order to store the initial weights and biases in a bucket that is mounted into the container as a directory. This is most useful in cases where the Model is representing a pre-trained model.

The Model object can optionally describe the training parameters including a reference to the Dataset that should be used for training. In this case, the Model object represents the eventual output of a training/finetuning operation.

```yaml
kind: Model
spec:
  source: {} # Required
  training: {} # Optional, will run a training job
```

### Model Sources

A Model can be sourced from Git. This will trigger a build Job.

```yaml
kind: Model
spec:
  source:
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