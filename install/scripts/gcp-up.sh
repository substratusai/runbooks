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

# Configure kubectl.
gcloud container clusters get-credentials --project ${PROJECT} --region ${cluster_region} ${cluster_name}
# Install nvidia driver
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/container-engine-accelerators/master/nvidia-driver-installer/cos/daemonset-preloaded-latest.yaml

# Install cluster components.
if [ "${INSTALL_OPERATOR}" == "yes" ]; then
  kubectl apply -f kubernetes/namespace.yaml
  kubectl apply -f kubernetes/config.yaml
  kubectl apply -f kubernetes/system.yaml
fi
