#!/bin/bash

set -e
set -u

# Required env variables:
: "$TOKEN $PROJECT"

# Used by gcloud:
export CLOUDSDK_AUTH_ACCESS_TOKEN=${TOKEN}
# Used by terraform:
export GOOGLE_OAUTH_ACCESS_TOKEN=${TOKEN}
INSTALL_OPERATOR="${INSTALL_OPERATOR:-yes}"
AUTO_APPROVE="${AUTO_APPROVE:-no}"

set -x
# Enable required services.
gcloud services enable --project ${PROJECT} container.googleapis.com
gcloud services enable --project ${PROJECT} artifactregistry.googleapis.com

# Create terraform state bucket if one does not exist.
TF_BUCKET=${PROJECT}-substratus-terraform
gcloud storage buckets describe gs://${TF_BUCKET} >/dev/null || gcloud storage buckets create --project ${PROJECT} gs://${TF_BUCKET}

# Apply infrastructure.
cd terraform/gcp

# Backend variables cannot be configured via env variables.
echo "bucket = \"${TF_BUCKET}\"" >>backend.tfvars
terraform init --backend-config=backend.tfvars

export TF_VAR_project_id=${PROJECT}
if [ "${AUTO_APPROVE}" == "yes" ]; then
  terraform apply -auto-approve
else
  terraform apply
fi
cluster_name=$(terraform output --raw cluster_name)
cluster_region=$(terraform output --raw cluster_region)

cd -

# Create a bucket for substratus models and datasets
export ARTIFACTS_BUCKET="gs://${PROJECT}-substratus-artifacts"
if ! gcloud storage buckets describe "${ARTIFACTS_BUCKET}" -q >/dev/null; then
  gcloud storage buckets create --project ${PROJECT} "${ARTIFACTS_BUCKET}" --location ${cluster_region}
fi

# Create Artifact Registry to host container images
if ! gcloud artifacts repositories describe substratus --location us-central1 --project ${PROJECT} -q > /dev/null; then
  gcloud artifacts repositories create substratus \
    --repository-format=docker --location=${cluster_region} \
    --project ${PROJECT}
fi

# Create Google Service Account used by all of Substratus to access GCS and GAR
export SERVICE_ACCOUNT="substratus@${PROJECT}.iam.gserviceaccount.com"
if ! gcloud iam service-accounts describe ${SERVICE_ACCOUNT} --project ${PROJECT}; then
  gcloud iam service-accounts create substratus --project ${PROJECT}
fi

# Give required permissions to Service Account
gcloud storage buckets add-iam-policy-binding ${ARTIFACTS_BUCKET} \
  --member="serviceAccount:${SERVICE_ACCOUNT}" --role=roles/storage.admin \
  --project ${PROJECT}

gcloud artifacts repositories add-iam-policy-binding substratus \
  --location us-central1 --member="serviceAccount:${SERVICE_ACCOUNT}" \
  --role=roles/artifactregistry.admin --project ${PROJECT}

# Allow the Service Account to bind K8s Service Account to this Service Account
gcloud iam service-accounts add-iam-policy-binding ${SERVICE_ACCOUNT} \
   --role roles/iam.serviceAccountAdmin --project ${PROJECT} \
   --member "serviceAccount:${SERVICE_ACCOUNT}"

# allow SCI to manage workload identity bindings
gcloud iam service-accounts add-iam-policy-binding ${SERVICE_ACCOUNT} \
  --role roles/iam.serviceAccountAdmin --project ${PROJECT} \
  --member "serviceAccount:${PROJECT}.svc.id.goog[substratus/sci]"

# Allow to create signed URLs
gcloud iam service-accounts add-iam-policy-binding ${SERVICE_ACCOUNT} \
   --role roles/iam.serviceAccountTokenCreator --project ${PROJECT} \
   --member "serviceAccount:${SERVICE_ACCOUNT}"

gcloud iam service-accounts add-iam-policy-binding ${SERVICE_ACCOUNT} \
   --role roles/iam.workloadIdentityUser --project ${PROJECT} \
   --member "serviceAccount:${PROJECT}.svc.id.goog[substratus/sci]"

# Configure kubectl.
gcloud container clusters get-credentials --project ${PROJECT} --region ${cluster_region} ${cluster_name}
# Install nvidia driver
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/container-engine-accelerators/master/nvidia-driver-installer/cos/daemonset-preloaded-latest.yaml

# Install cluster components.
if [ "${INSTALL_OPERATOR}" == "yes" ]; then
  kubectl create ns substratus
  kubectl apply -f - << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: system
  namespace: substratus
data:
  CLOUD: gcp
EOF
  kubectl apply -f kubernetes/gcp/system.yaml
fi
