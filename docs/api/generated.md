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

### CPUResources





_Appears in:_
- [Resources](#resources)

| Field | Description |
| --- | --- |
| `count` _integer_ | Count is the number of CPU cores. |
| `memory` _integer_ | Memory is the amount of RAM in Gi. |


### Container





_Appears in:_
- [DatasetSpec](#datasetspec)
- [ModelServerSpec](#modelserverspec)
- [ModelSpec](#modelspec)
- [NotebookSpec](#notebookspec)

| Field | Description |
| --- | --- |
| `git` _[GitSource](#gitsource)_ | Git is a reference to a git repository that will be built within the cluster. Built image will be set in the Image field. |
| `image` _string_ | Image of a container. |


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


### DatasetLoader





_Appears in:_
- [DatasetSpec](#datasetspec)

| Field | Description |
| --- | --- |
| `params` _object (keys:string, values:string)_ | Params will be passed into the loading process as environment variables. Environment variable name will be `"PARAM_" + uppercase(key)`. |


### DatasetSpec



DatasetSpec defines the desired state of Dataset.

_Appears in:_
- [Dataset](#dataset)

| Field | Description |
| --- | --- |
| `filename` _string_ | Filename is the name of the file when it is downloaded. |
| `container` _[Container](#container)_ | Container that contains dataset loading code and dependencies. |
| `resources` _[Resources](#resources)_ | Resources are the compute resources required by the container. |
| `loader` _[DatasetLoader](#datasetloader)_ | Loader configures the loading process. |


### DatasetStatus



DatasetStatus defines the observed state of Dataset.

_Appears in:_
- [Dataset](#dataset)

| Field | Description |
| --- | --- |
| `ready` _boolean_ | Ready indicates that the Dataset is ready to use. See Conditions for more details. |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the Dataset. |
| `url` _string_ | URL of the loaded data. |


### GPUResources





_Appears in:_
- [Resources](#resources)

| Field | Description |
| --- | --- |
| `type` _[GPUType](#gputype)_ | Type of GPU. |
| `count` _integer_ | Count is the number of GPUs. |


### GPUType

_Underlying type:_ `string`



_Appears in:_
- [GPUResources](#gpuresources)



### GitSource





_Appears in:_
- [Container](#container)

| Field | Description |
| --- | --- |
| `url` _string_ | URL to the git repository. Example: https://github.com/my-username/my-repo |
| `path` _string_ | Path within the git repository referenced by url. |
| `branch` _string_ | Branch is the git branch to use. |


### Model



The Model API is used to build and train machine learning models. 
 - Base models can be built from a Git repository. 
 - Models can be trained by combining a base Model with a Dataset. 
 - Model artifacts are persisted in cloud buckets.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `substratus.ai/v1`
| `kind` _string_ | `Model`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[ModelSpec](#modelspec)_ | Spec is the desired state of the Model. |
| `status` _[ModelStatus](#modelstatus)_ | Status is the observed state of the Model. |


### ModelLoader





_Appears in:_
- [ModelSpec](#modelspec)

| Field | Description |
| --- | --- |
| `params` _object (keys:string, values:string)_ | Params will be passed into the loading process as environment variables. Environment variable name will be `"PARAM_" + uppercase(key)`. |


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
| `container` _[Container](#container)_ | Container that contains model serving application and dependencies. |
| `resources` _[Resources](#resources)_ | Resources are the compute resources required by the container. |
| `model` _[ObjectRef](#objectref)_ | Model references the Model object to be served. |


### ModelServerStatus



ModelServerStatus defines the observed state of ModelServer

_Appears in:_
- [ModelServer](#modelserver)

| Field | Description |
| --- | --- |
| `ready` _boolean_ | Ready indicates whether the ModelServer is ready to serve traffic. See Conditions for more details. |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the ModelServer. |


### ModelSpec



ModelSpec defines the desired state of Model

_Appears in:_
- [Model](#model)

| Field | Description |
| --- | --- |
| `container` _[Container](#container)_ | Container that contains model code and dependencies. |
| `resources` _[Resources](#resources)_ | Resources are the compute resources required by the container. |
| `loader` _[ModelLoader](#modelloader)_ | Loader should be set to run a loading job. Cannot also be set with Trainer. |
| `trainer` _[ModelTrainer](#modeltrainer)_ | Trainer should be set to run a training job. Cannot also be set with Loader. |


### ModelStatus



ModelStatus defines the observed state of Model

_Appears in:_
- [Model](#model)

| Field | Description |
| --- | --- |
| `ready` _boolean_ | Ready indicates that the Model is ready to use. See Conditions for more details. |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the Model. |
| `url` _string_ | URL of model artifacts. |


### ModelTrainer





_Appears in:_
- [ModelSpec](#modelspec)

| Field | Description |
| --- | --- |
| `baseModel` _[ObjectRef](#objectref)_ | BaseModel should be set in order to mount another model to be used for transfer learning. |
| `datasetName` _[ObjectRef](#objectref)_ | Dataset to mount for training. |
| `epochs` _integer_ | Epochs is the total number of iterations that should be run through the training data. Increasing this number will increase training time. The EPOCHS environment variable will be set during training. |
| `dataLimit` _integer_ | DataLimit is the maximum number of training records to use. In the case of JSONL, this would be the total number of lines to train with. Increasing this number will increase training time. The DATA_LIMIT environment variable will be set during training. |
| `batchSize` _integer_ | BatchSize is the number of training records to use per (forward and backward) pass through the model. Increasing this number will increase the memory requirements of the training process. The BATCH_SIZE environment variable will be set during training. |
| `params` _object (keys:string, values:string)_ | Params will be passed into the loading process as environment variables. Environment variable name will be `"PARAM_" + uppercase(key)`. For standard parameters like Epochs, use the well-defined Trainer fields. |


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
| `suspend` _boolean_ | Suspend should be set to true to stop the notebook (Pod) from running. |
| `container` _[Container](#container)_ |  |
| `resources` _[Resources](#resources)_ | Resources are the compute resources required by the container. |
| `model` _[ObjectRef](#objectref)_ | Model to load into the notebook container. |
| `dataset` _[ObjectRef](#objectref)_ | Dataset to load into the notebook container. |


### NotebookStatus



NotebookStatus defines the observed state of Notebook

_Appears in:_
- [Notebook](#notebook)

| Field | Description |
| --- | --- |
| `ready` _boolean_ | Ready indicates that the Notebook is ready to serve. See Conditions for more details. |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the Notebook. |


### ObjectRef





_Appears in:_
- [ModelServerSpec](#modelserverspec)
- [ModelTrainer](#modeltrainer)
- [NotebookSpec](#notebookspec)

| Field | Description |
| --- | --- |
| `name` _string_ | Name of Kubernetes object. |


### Resources





_Appears in:_
- [DatasetSpec](#datasetspec)
- [ModelServerSpec](#modelserverspec)
- [ModelSpec](#modelspec)
- [NotebookSpec](#notebookspec)

| Field | Description |
| --- | --- |
| `cpu` _[CPUResources](#cpuresources)_ |  |
| `gpu` _[GPUResources](#gpuresources)_ |  |


