#!/bin/bash

set -u
set -x

PROJECT_ID=${PROJECT_ID:=$(gcloud config get project)}
REGION=${REGION:-us-central1}
ZONE=${ZONE:=${REGION}-a}

export CLUSTER_NAME=substratus
gcloud container clusters delete ${CLUSTER_NAME} --location ${REGION} --quiet --async

export SERVICE_ACCOUNT="substratus@${PROJECT_ID}.iam.gserviceaccount.com"
gcloud --quiet iam service-accounts delete ${SERVICE_ACCOUNT} --project ${PROJECT_ID}

export ARTIFACTS_BUCKET="gs://${PROJECT_ID}-substratus-artifacts"
gcloud --quiet storage rm -r --project ${PROJECT_ID} "${ARTIFACTS_BUCKET}" || true

gcloud --quiet artifacts repositories delete substratus \
  --project ${PROJECT_ID} --location us-central1 || true
