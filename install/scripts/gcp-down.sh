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

TF_BUCKET=${PROJECT}-substratus-terraform

# Delete infrastructure.
cd terraform/gcp

# Backend variables cannot be configured via env variables.
echo "bucket = \"${TF_BUCKET}\"" >>backend.tfvars
terraform init --backend-config=backend.tfvars

export TF_VAR_project_id=${PROJECT}
cluster_name=$(terraform output --raw cluster_name)
cluster_region=$(terraform output --raw cluster_region)
if [ "${AUTO_APPROVE}" == "yes" ]; then
  terraform destroy -auto-approve
else
  terraform destroy
fi

cd -

set -x
export SERVICE_ACCOUNT="substratus@${PROJECT}.iam.gserviceaccount.com"
# can't delete service account, getting permission denied
# gcloud --quiet iam service-accounts delete ${SERVICE_ACCOUNT} --project ${PROJECT}


export ARTIFACTS_BUCKET="gs://${PROJECT}-substratus-artifacts"
gcloud --quiet storage rm -r --project ${PROJECT} "${ARTIFACTS_BUCKET}" || true

gcloud --quiet artifacts repositories delete substratus \
  --project ${PROJECT} --location us-central1 || true
