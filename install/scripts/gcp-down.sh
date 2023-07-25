#!/bin/bash

set -e
set -u

# Required env variables:
: "$TOKEN $PROJECT"

# Used by gcloud:
export CLOUDSDK_AUTH_ACCESS_TOKEN=${TOKEN}
# Used by terraform:
export GOOGLE_OAUTH_ACCESS_TOKEN=${TOKEN}
AUTO_APPROVE="${AUTO_APPROVE:-no}"

tf_bucket=${PROJECT}-substratus-terraform

# Delete infrastructure.
cd terraform/gcp

# Backend variables cannot be configured via env variables.
echo "bucket = \"${tf_bucket}\"" >>backend.tfvars
terraform init --backend-config=backend.tfvars

export TF_VAR_project_id=${PROJECT}
if [ "$AUTO_APPROVE" == "yes" ]; then
  terraform destroy -auto-approve
else
  terraform destroy
fi
cluster=$(terraform output --raw cluster_name)

cd -
