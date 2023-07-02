# API Reference


<!-- GENERATED FROM https://github.com/substratusai/substratus USING make docs WHICH WROTE TO PATH /docs/api/ -->

**API Version: substratus.ai/v1**

Package v1 contains API Schema definitions for Substratus.

## Resources
- [Dataset](#dataset)
- [Model](#model)
- [ModelServer](#modelserver)
- [Notebook](#notebook)


## Types

### ComputeType

_Underlying type:_ `string`



_Appears in:_
- [ModelCompute](#modelcompute)



### Dataset



The Dataset API is used to describe data that can be referenced for training Models. 
 - Datasets pull in remote data sources using containerized data loaders. 
 - Users can specify their own ETL logic by referencing a repository from a Dataset. 
 - Users can leverage pre-built data loader integrations with various sources. 
 - Training typically requires a large dataset. The Dataset API pulls a dataset once and stores it in a bucket, which is mounted directly into training Jobs. 
 - The Dataset API allows users to query ready-to-use datasets (`kubectl get datasets`). 
 - The Dataset API allows Kubernetes RBAC to be applied as a mechanism for controlling access to data.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `substratus.ai/v1`
| `kind` _string_ | `Dataset`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[DatasetSpec](#datasetspec)_ | Spec is the desired state of the Dataset. |
| `status` _[DatasetStatus](#datasetstatus)_ | Status is the observed state of the Dataset. |


### DatasetSource



DatasetSource if a reference to the code that is doing the data sourcing.

_Appears in:_
- [DatasetSpec](#datasetspec)

| Field | Description |
| --- | --- |
| `git` _[GitSource](#gitsource)_ | Git is a reference to the git repository that contains the data loading code. |


### DatasetSpec



DatasetSpec defines the desired state of Dataset.

_Appears in:_
- [Dataset](#dataset)

| Field | Description |
| --- | --- |
| `filename` _string_ | Filename is the name of the file when it is downloaded. |
| `source` _[DatasetSource](#datasetsource)_ | Source if a reference to the code that is doing the data sourcing. |


### DatasetStatus



DatasetStatus defines the observed state of Dataset.

_Appears in:_
- [Dataset](#dataset)

| Field | Description |
| --- | --- |
| `url` _string_ | URL points to the underlying data storage (bucket URL). |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the Dataset. |


### GitSource





_Appears in:_
- [DatasetSource](#datasetsource)
- [ModelSource](#modelsource)

| Field | Description |
| --- | --- |
| `url` _string_ | URL to the git repository. Example: https://github.com/substratusai/model-falcon-40b |
| `path` _string_ | Path within the git repository referenced by url. |
| `branch` _string_ | Branch is the git branch to use. |


### Model



The Model API is used to build and train machine learning models. 
 - Base models can be built from a Git repository. 
 - Models can be trained by combining a base Model with a Dataset.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `substratus.ai/v1`
| `kind` _string_ | `Model`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[ModelSpec](#modelspec)_ | Spec is the desired state of the Model. |
| `status` _[ModelStatus](#modelstatus)_ | Status is the observed state of the Model. |


### ModelCompute





_Appears in:_
- [ModelSpec](#modelspec)

| Field | Description |
| --- | --- |
| `types` _[ComputeType](#computetype) array_ | Types is a list of supported compute types for this Model. This list should be ordered by preference, with the most preferred type first. |


### ModelServer



The ModelServer API is used to deploy a server that exposes the capabilities of a Model via a HTTP interface.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `substratus.ai/v1`
| `kind` _string_ | `ModelServer`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[ModelServerSpec](#modelserverspec)_ | Spec is the desired state of the ModelServer. |
| `status` _[ModelServerStatus](#modelserverstatus)_ | Status is the observed state of the ModelServer. |


### ModelServerSpec



ModelServerSpec defines the desired state of ModelServer

_Appears in:_
- [ModelServer](#modelserver)

| Field | Description |
| --- | --- |
| `modelName` _string_ | ModelName is the .metadata.name of the Model to be served. |


### ModelServerStatus



ModelServerStatus defines the observed state of ModelServer

_Appears in:_
- [ModelServer](#modelserver)

| Field | Description |
| --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the ModelServer. |


### ModelSize





_Appears in:_
- [ModelSpec](#modelspec)

| Field | Description |
| --- | --- |
| `parameterCount` _integer_ | ParameterCount is the number of parameters in the underlying model. |
| `parameterBits` _integer_ | ParameterBits is the number of bits per parameter in the underlying model. Common values would be 8, 16, 32. |


### ModelSource





_Appears in:_
- [ModelSpec](#modelspec)

| Field | Description |
| --- | --- |
| `git` _[GitSource](#gitsource)_ | Git is a reference to a git repository containing model code. |
| `modelName` _string_ | ModelName is the .metadata.name of another Model that this Model should be based on. |


### ModelSpec



ModelSpec defines the desired state of Model

_Appears in:_
- [Model](#model)

| Field | Description |
| --- | --- |
| `source` _[ModelSource](#modelsource)_ | Source is a reference to the source of the model. |
| `training` _[ModelTraining](#modeltraining)_ | Training should be set to run a training job. |
| `size` _[ModelSize](#modelsize)_ | Size describes different size dimensions of the underlying model. |
| `compute` _[ModelCompute](#modelcompute)_ | Compute describes the compute requirements and preferences of the model. |


### ModelStatus



ModelStatus defines the observed state of Model

_Appears in:_
- [Model](#model)

| Field | Description |
| --- | --- |
| `containerImage` _string_ | ContainerImage is reference to the container image that was built for this Model. |
| `servers` _string array_ | Servers is the list of servers that are currently running this Model. Soon to be deprecated. |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the Model. |


### ModelTraining





_Appears in:_
- [ModelSpec](#modelspec)

| Field | Description |
| --- | --- |
| `datasetName` _string_ | DatasetName is the .metadata.name of the Dataset to use for training. |
| `params` _[ModelTrainingParams](#modeltrainingparams)_ | Params is a list of hyperparameters to use for training. |


### ModelTrainingParams





_Appears in:_
- [ModelTraining](#modeltraining)

| Field | Description |
| --- | --- |
| `epochs` _integer_ | Epochs is the total number of iterations that should be run through the training data. Increasing this number will increase training time. |
| `dataLimit` _integer_ | DataLimit is the maximum number of training records to use. In the case of JSONL, this would be the total number of lines to train with. Increasing this number will increase training time. |
| `batchSize` _integer_ | BatchSize is the number of training records to use per (forward and backward) pass through the model. Increasing this number will increase the memory requirements of the training process. |


### Notebook



The Notebook API can be used to quickly spin up a development environment backed by high performance compute. 
 - Notebooks integrate with the Model and Dataset APIs allow for quick iteration. 
 - Notebooks can be synced to local directories to streamline developer experiences using Substratus kubectl plugins.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `substratus.ai/v1`
| `kind` _string_ | `Notebook`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[NotebookSpec](#notebookspec)_ | Spec is the observed state of the Notebook. |
| `status` _[NotebookStatus](#notebookstatus)_ | Status is the observed state of the Notebook. |


### NotebookSpec



NotebookSpec defines the desired state of Notebook

_Appears in:_
- [Notebook](#notebook)

| Field | Description |
| --- | --- |
| `modelName` _string_ | ModelName is the .metadata.name of the Model that this Notebook should be sourced from. |
| `suspend` _boolean_ | Suspend should be set to true to stop the notebook (Pod) from running. |


### NotebookStatus



NotebookStatus defines the observed state of Notebook

_Appears in:_
- [Notebook](#notebook)

| Field | Description |
| --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the Notebook. |


