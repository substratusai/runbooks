# Architecture

## Models

### Base Models

Base models are built from a Git repository by specifying `.spec.source.git` in a `kind: Model` object.

<img src="base-models.excalidraw.png" width="70%"></img>

### Training New Models

Training is triggered by creating a `kind: Model` with `.spec.training` and `.spec.source.modelName` set. It can be thought of as combining an existing model with a new set of data, producing a new model.

<img src="training.excalidraw.png" width="70%"></img>

## Datasets

The Dataset API is used to describe data that can be referenced for training Models.

* Training models typically requires a large dataset. Pulling this dataset from a remote source every time you train a model is slow and unreliable. The Dataset API pulls a dataset once and stores it on fast Persistent Disks, mounted directly to training Jobs.
* The Dataset controller pulls in a remote dataset once, and stores it, guaranteeing every model that references that dataset uses the same exact data (reproducable training results).
* The Dataset API allows users to query datasets on the platform (`kubectl get datasets`).
* The Dataset API allows Kubernetes RBAC to be applied as a mechanism for controlling access to data.
* Similar to the Model API, the Dataset API contains metadata about datasets (size of dataset --> which can be used to inform training job resource requirements).
* Dataset API provides a central place to define the auth credentials for remote dataset sources.
* Dataset API could provide integrations with many data sources including the Huggingface Hub, materializing the output of SQL queries, scraping and downloading an entire confluence site, etc.
* If Models have consistent or at least declarative training data format expectations, then the Dataset API allows for a programtic way to orchestrate coupling those models to a large number of datasets and producing a matrix of trained models.

<img src="datasets.excalidraw.png" width="70%"></img>

