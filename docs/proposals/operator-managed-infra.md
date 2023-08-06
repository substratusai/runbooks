# Operator Managed Infra and multi namespace
Design doc: Not a feature yet

Instead of managing all the infrastructure through terraform, the
operator itself should be responsible for managing the infra it requires.
This will make it easier for users to adopt Substratus into existing
environments without having to learn IaC tooling.


## Why?
* Make install onto existing K8s cluster straight forward and remove
  complexity of using Terraform
* Improved UX, all an end-user needs to do is install our manifests and ensure service account
  has enough permissions
* Enterprises generally have very specific way and their existing IaC tools to create
  clusters and manage nodepools
* Allow Substratus to work in a multi namespace environment because right now default
  namespace is hardcoded in workload identity settings
* Get closer to having a local Substratus with kind by making Object Storage optional

## How?
* Make the Image Registry and Object Storage Bucket configurable as a setting of the operator itself
* Object storage will be optional and if not provided then default PVC class will be used
* Utilize a single service account that has enough permissions to do everything needed within Substratus
* Simplify Substratus to use a single K8s SA. Simplicity is preferred here over the minor security benefits you get
  for using a different SA for each kind of Substratus resource.
* Provide Substratus controller the permissions to manage registry, bucket and set IAM policy on the GCP SA.
  The following permissions will be required by the substratus Service Account on the project itself:
  ```
  roles/storage.admin, roles/artifactregistry.repoAdmin, roles/iam.serviceAccountAdmin
  ```

### Implementation details
* Object storage: Create bucket if the configured bucket does not exist
* Image Registry: Create registry if the configured registry does not exist
* Create K8s Service Accounts automatically in each namespace where Substratus is used and
  annotate the K8s Service Account with the correct GCP Service Account. Ensure that following
  gets called when a new Service Account is provisioned in a new namespace:
  ```
  gcloud iam service-accounts add-iam-policy-binding substratus@my-project.iam.gserviceaccount.com \
   --role roles/iam.workloadIdentityUser \
   --member "serviceAccount:myproject.svc.id.goog[new-namespace/substratus]"
  ```

How should Substratus handle infra management?
* Terraform
* Native K8s controller that uses Golang SDK for AWS/GCP/..

The Native K8s controller would better fit within the existing Substratus code base.
It would also allow us to verify if a resource is already there and if not just create
it.

## User Impact / Docs

1. (Optional) Create your GKE cluster `gcloud container clusters create` and create nodepools

2. Create service account and assign permissions needed

   ```
   gcloud iam service-accounts create substratus
   gcloud iam service-accounts add-iam-policy-binding substratus@my-project.iam.gserviceaccount.com \
    --role roles/iam.workloadIdentityUser \
    --member "serviceAccount:myproject.svc.id.goog[substratus/substratus]"
   gcloud projects add-iam-policy-binding my-project \
    --member "serviceAccount:substratus@my-project.iam.gserviceaccount.com" \
    --role "roles/storage.admin" --role "roles/artifactregistry.repoAdmin" \
    --role "roles/iam.serviceAccountAdmin"
   ```

   The role roles/iam.serviceAccountAdmin is needed to be able to use workload identity across multiple
   namespaces. The controller can now add IAM policy bindings to the substratus SA for other namespaces.

3. Deploy Substratus operator using helm

The following values.yaml would be provided (or configmap):
```
image_registry:  us-central1-docker.pkg.dev/my-project/substratus-repo
bucket:          gs://my-project-substratus
# Ensures the K8s Service Accounts have the right annotations
service_account: substratus@my-project.iam.gserviceaccount.com
```

Install operator:
```
helm repo add substratusai https://substratusai.github.io/substratusai-helm
helm install substratusai/substratus
```

## TODO
* Understand how this would work in AWS
* Provide pseudocode of how Bucket controller might work in AWS and GCP
* Provide install steps for AWS
