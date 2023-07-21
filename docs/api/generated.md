# API Reference


<!-- GENERATED FROM https://github.com/substratusai/substratus USING make docs WHICH WROTE TO PATH /docs/api/ -->

**API Version: substratus.ai/v1**

Package v1 contains API Schema definitions for Substratus.

## Resources
- [Dataset](#dataset)
- [Model](#model)
- [Notebook](#notebook)
- [Server](#server)


## Types

### ArtifactsStatus





_Appears in:_
- [DatasetStatus](#datasetstatus)
- [ModelStatus](#modelstatus)

| Field | Description |
| --- | --- |
| `url` _string_ |  |


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


### DatasetSpec



DatasetSpec defines the desired state of Dataset.

_Appears in:_
- [Dataset](#dataset)

| Field | Description |
| --- | --- |
| `command` _string array_ | Command to run in the container. |
| `image` _[Image](#image)_ | Image that contains dataset loading code and dependencies. |
| `resources` _[Resources](#resources)_ | Resources are the compute resources required by the container. |
| `params` _object (keys:string, values:IntOrString)_ | Params will be passed into the loading process as environment variables. |


### DatasetStatus



DatasetStatus defines the observed state of Dataset.

_Appears in:_
- [Dataset](#dataset)

| Field | Description |
| --- | --- |
| `ready` _boolean_ | Ready indicates that the Dataset is ready to use. See Conditions for more details. |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the Dataset. |
| `artifacts` _[ArtifactsStatus](#artifactsstatus)_ | Artifacts status. |
| `image` _[ImageStatus](#imagestatus)_ | Image contains the status of the image. Upload URL is reported here. |


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



### Image





_Appears in:_
- [DatasetSpec](#datasetspec)
- [ModelSpec](#modelspec)
- [NotebookSpec](#notebookspec)
- [ServerSpec](#serverspec)

| Field | Description |
| --- | --- |
| `git` _[ImageGit](#imagegit)_ | Git is a reference to a git repository that will be built within the cluster. Built image will be set in the Image field. |
| `name` _string_ | Name of container image (example: "docker.io/your-username/your-image"). |
| `upload` _[ImageUpload](#imageupload)_ | Upload is a signal that a local tar of the directory should be uploaded to be built as an image. |


### ImageGit





_Appears in:_
- [Image](#image)

| Field | Description |
| --- | --- |
| `url` _string_ | URL to the git repository. Example: https://github.com/my-username/my-repo |
| `path` _string_ | Path within the git repository referenced by url. |
| `branch` _string_ | Branch is the git branch to use. |


### ImageStatus





_Appears in:_
- [DatasetStatus](#datasetstatus)
- [ModelStatus](#modelstatus)
- [NotebookStatus](#notebookstatus)
- [ServerStatus](#serverstatus)

| Field | Description |
| --- | --- |
| `uploadURL` _string_ | the Signed upload URL |
| `md5checksum` _string_ | Md5Checksum is the last md5 checksum that resulted in the successful creation of an UploadURL. |


### ImageUpload





_Appears in:_
- [Image](#image)

| Field | Description |
| --- | --- |
| `md5checksum` _string_ | Md5Checksum is the md5 checksum of the tar'd repo root requested to be uploaded and built. |


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


### ModelSpec



ModelSpec defines the desired state of Model

_Appears in:_
- [Model](#model)

| Field | Description |
| --- | --- |
| `command` _string array_ | Command to run in the container. |
| `image` _[Image](#image)_ | Image that contains model code and dependencies. |
| `resources` _[Resources](#resources)_ | Resources are the compute resources required by the container. |
| `baseModel` _[ObjectRef](#objectref)_ | BaseModel should be set in order to mount another model to be used for transfer learning. |
| `trainingDataset` _[ObjectRef](#objectref)_ | Dataset to mount for training. |
| `params` _object (keys:string, values:IntOrString)_ | Parameters are passing into the model training/loading container as environment variables. Environment variable name will be `"PARAM_" + uppercase(key)`. |


### ModelStatus



ModelStatus defines the observed state of Model

_Appears in:_
- [Model](#model)

| Field | Description |
| --- | --- |
| `ready` _boolean_ | Ready indicates that the Model is ready to use. See Conditions for more details. |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the Model. |
| `artifacts` _[ArtifactsStatus](#artifactsstatus)_ | Artifacts status. |
| `image` _[ImageStatus](#imagestatus)_ | Image contains the status of the image. Upload URL is reported here. |


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
| `command` _string array_ | Command to run in the container. |
| `suspend` _boolean_ | Suspend should be set to true to stop the notebook (Pod) from running. |
| `image` _[Image](#image)_ | Image that contains notebook and dependencies. |
| `resources` _[Resources](#resources)_ | Resources are the compute resources required by the container. |
| `model` _[ObjectRef](#objectref)_ | Model to load into the notebook container. |
| `dataset` _[ObjectRef](#objectref)_ | Dataset to load into the notebook container. |
| `params` _object (keys:string, values:IntOrString)_ | Params will be passed into the notebook container as environment variables. |


### NotebookStatus



NotebookStatus defines the observed state of Notebook

_Appears in:_
- [Notebook](#notebook)

| Field | Description |
| --- | --- |
| `ready` _boolean_ | Ready indicates that the Notebook is ready to serve. See Conditions for more details. |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the Notebook. |
| `image` _[ImageStatus](#imagestatus)_ | Image contains the status of the image. Upload URL is reported here. |


### ObjectRef





_Appears in:_
- [ModelSpec](#modelspec)
- [NotebookSpec](#notebookspec)
- [ServerSpec](#serverspec)

| Field | Description |
| --- | --- |
| `name` _string_ | Name of Kubernetes object. |


### Resources





_Appears in:_
- [DatasetSpec](#datasetspec)
- [ModelSpec](#modelspec)
- [NotebookSpec](#notebookspec)
- [ServerSpec](#serverspec)

| Field | Description |
| --- | --- |
| `cpu` _integer_ | CPU resources. |
| `disk` _integer_ | Disk size in Gigabytes. |
| `memory` _integer_ | Memory is the amount of RAM in Gigabytes. |
| `gpu` _[GPUResources](#gpuresources)_ | GPU resources. |


### Server



The Server API is used to deploy a server that exposes the capabilities of a Model via a HTTP interface.



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `substratus.ai/v1`
| `kind` _string_ | `Server`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[ServerSpec](#serverspec)_ | Spec is the desired state of the Server. |
| `status` _[ServerStatus](#serverstatus)_ | Status is the observed state of the Server. |


### ServerSpec



ServerSpec defines the desired state of Server

_Appears in:_
- [Server](#server)

| Field | Description |
| --- | --- |
| `command` _string array_ | Command to run in the container. |
| `image` _[Image](#image)_ | Image that contains model serving application and dependencies. |
| `resources` _[Resources](#resources)_ | Resources are the compute resources required by the container. |
| `model` _[ObjectRef](#objectref)_ | Model references the Model object to be served. |


### ServerStatus



ServerStatus defines the observed state of Server

_Appears in:_
- [Server](#server)

| Field | Description |
| --- | --- |
| `ready` _boolean_ | Ready indicates whether the Server is ready to serve traffic. See Conditions for more details. |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.26/#condition-v1-meta) array_ | Conditions is the list of conditions that describe the current state of the Server. |
| `image` _[ImageStatus](#imagestatus)_ | Image contains the status of the image. Upload URL is reported here. |


