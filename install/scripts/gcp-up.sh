#!/bin/bash

set -e
set -u

# Required env variables:
: "$TOKEN $PROJECT"

# Used by gcloud:
export CLOUDSDK_AUTH_ACCESS_TOKEN=${TOKEN}
# Used by terraform:
export GOOGLE_OAUTH_ACCESS_TOKEN=${TOKEN}

# Enable required services.
gcloud services enable --project ${PROJECT} container.googleapis.com
gcloud services enable --project ${PROJECT} artifactregistry.googleapis.com

# Create terraform state bucket if one does not exist.
tf_bucket=${PROJECT}-substratus-terraform
gcloud storage buckets describe gs://${tf_bucket} >/dev/null || gcloud storage buckets create --project ${PROJECT} gs://${tf_bucket}

# Apply infrastructure.
cd terraform/gcp

# Backend variables cannot be configured via env variables.
echo "bucket = \"${tf_bucket}\"" >>backend.tfvars
terraform init --backend-config=backend.tfvars

export TF_VAR_project_id=${PROJECT}
terraform apply
cluster_name=$(terraform output --raw cluster_name)
cluster_region=$(terraform output --raw cluster_region)

cd -

# Configure kubectl.
gcloud container clusters get-credentials --project ${PROJECT} --region ${cluster_region} ${cluster_name}

# Install cluster components.
kubectl apply -f kubernetes/system.yaml
