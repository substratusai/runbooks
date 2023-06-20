#!/bin/bash

set -e
set -u

# Required env variables:
: "$TOKEN $PROJECT $REGION $ZONE"

# Used by gcloud:
export CLOUDSDK_AUTH_ACCESS_TOKEN=${TOKEN}
# Used by terraform:
export GOOGLE_OAUTH_ACCESS_TOKEN=${TOKEN}

# Create terraform state bucket.
bucket=${PROJECT}-substratus
gcloud storage buckets describe gs://${bucket} >/dev/null || gcloud storage buckets create --project ${PROJECT} gs://${bucket}

# Apply infrastructure.
cd terraform/gcp
echo "bucket = \"${bucket}\"" >>backend.tfvars
echo "project_id = \"${PROJECT}\"" >>terraform.tfvars
echo "region = \"${REGION}\"" >>terraform.tfvars
echo "zone = \"${ZONE}\"" >>terraform.tfvars
terraform init --backend-config=backend.tfvars
terraform apply --auto-approve
cluster=$(terraform output --raw cluster_name)
# I did not see a configuration option for setting NAP locations in terraform:
gcloud container clusters update --project ${PROJECT} ${cluster} --region ${REGION} --enable-autoprovisioning --autoprovisioning-locations=$ZONE
cd -
