#!/bin/bash

set -e
set -u

# Required env variables:
: "$TOKEN $PROJECT"

AUTO_APPROVE=""

while (("$#")); do
  case "$1" in
  --auto-approve)
    AUTO_APPROVE="-auto-approve"
    shift
    ;;
  *)
    echo "Error: Invalid argument"
    exit 1
    ;;
  esac
done

# Used by gcloud:
export CLOUDSDK_AUTH_ACCESS_TOKEN=${TOKEN}
# Used by terraform:
export GOOGLE_OAUTH_ACCESS_TOKEN=${TOKEN}

tf_bucket=${PROJECT}-substratus-terraform

# Delete infrastructure.
cd terraform/gcp

# Backend variables cannot be configured via env variables.
echo "bucket = \"${tf_bucket}\"" >>backend.tfvars
terraform init --backend-config=backend.tfvars

export TF_VAR_project_id=${PROJECT}
terraform destroy ${AUTO_APPROVE}
cluster=$(terraform output --raw cluster_name)

cd -
