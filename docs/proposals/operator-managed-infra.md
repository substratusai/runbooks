# Operator Managed Infra and multi namespace
Design doc: Not a feature yet

Instead of managing all the infrastructure through terraform, the
operator itself should be responsible for managing the Workload Identity
bindings between the K8s Service Account and the Cloud Service Account/IAM role.

Doing this, will make it possible to use Substratus resources in multiple
namespaces without having to use another tool to configure Cloud identity/role mappings.

Out of scope:
* Managing the bucket, image registry or other any other resources


## Why?
* Simplified install: Allow install to use `gcloud/aws` or `eksctl` to create initial K8s cluster + nodepools
* Make install onto existing K8s cluster straight forward and remove
  complexity of using Terraform
* Improved UX, all an end-user needs to do is install our manifests and ensure GCP service account has enough permissions
* Enterprises generally have very specific requirements about how they create their K8s Clusters and nodepools.
* Allow Substratus to work in a multi namespace environment. Currently namespace is hardcoded in workload identity settings in the terraform

## How?

<img src="./diagrams/operator-managed-infra.md" width="80%"></img>

### Installation
**Installation Flow** will be responsible for the following:
* Create a K8s cluster with nodes/GPUs, this step is optional if end-user already has a K8s cluster (eksctl or gcloud)
* Create Bucket, this step is optional if end-user already has a bucket
* Create Image Registry, this step is optional if end-user already has an image registry
* REQUIRED: Create Google Service Account or AWS IAM Role that has resource level permissions to:
  * read and write a specific bucket
  * pull and push to a specific Image Registry (GAR/ECR)
  * (GCP) set IAM policy on the service Account itself to be able to manage workload identity bindings to KSAs. This can be done by assigning the service account to be a ServiceAccountAdmin to itself
  * (AWS) UpdateAssumeRolePolicy on the AWS Role used by Substratus (required for multi namespace support)
* REQUIRED: Apply Subsubstratus manifests and configure them to use correct bucket and image registry

### Controller
Controller will be responsible for the following:
* Creating the K8s ServiceAccount and related binding by calling `enforceServiceAccount` whenever a Substratus resource gets reconciled
* (GCP only) example set annotation for Google Service Account AND update IAM policy on the Service Account so it can use the Google Service Account. For example:
  ```
  gcloud iam service-accounts add-iam-policy-binding substratus@my-project.iam.gserviceaccount.com \
   --role roles/iam.workloadIdentityUser \
   --member "serviceAccount:myproject.svc.id.goog[new-namespace/substratus]"
  ```
* (AWS Only) Annotate the service account with ARN of IAM role AND (awsmanager) call UpdateAssumeRolePolicy
* The annotations to service accounts happen inside the controller itself
* Any API calls made to clouds should go through a cloud manager e.g. `gcpmanager` or `awsmanager`


## User Impact / Docs
TODO: Update this with actual proposed install steps for GCP

1. (Optional) Create your GKE cluster `gcloud container clusters create` and create nodepools

2. Create service account and assign permissions needed

   ```
   gcloud iam service-accounts create substratus
   gcloud iam service-accounts add-iam-policy-binding substratus@my-project.iam.gserviceaccount.com \
    --role roles/iam.serviceAccountAdmin \
    --member "serviceAccount:myproject.svc.id.goog[substratus/substratus]"
   gcloud projects add-iam-policy-binding my-project \
    --member "serviceAccount:substratus@my-project.iam.gserviceaccount.com" \
    --role "roles/storage.admin" --role "roles/artifactregistry.repoAdmin"
   ```

The role `iam.serviceAccountAdmin` is needed to be able to use workload identity across multiple
   namespaces. The controller can now add IAM policy bindings to the substratus SA for other namespaces.
