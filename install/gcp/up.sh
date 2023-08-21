#!/bin/bash

set -e
set -u
set -x

PROJECT_ID=${PROJECT_ID:=$(gcloud config get project)}
REGION=${REGION:-us-central1}
ZONE=${ZONE:=${REGION}-a}
INSTALL_OPERATOR=no # set to yes if you want to install operator

# Enable required services.
gcloud services enable --project ${PROJECT_ID} container.googleapis.com
gcloud services enable --project ${PROJECT_ID} artifactregistry.googleapis.com

export CLUSTER_NAME=substratus
gcloud container clusters create ${CLUSTER_NAME} --location ${REGION} \
  --machine-type n2d-standard-8 --num-nodes 1 --min-nodes 1 --max-nodes 5 \
  --node-locations ${ZONE} --workload-pool ${PROJECT_ID}.svc.id.goog \
  --enable-image-streaming --enable-shielded-nodes --shielded-secure-boot \
  --shielded-integrity-monitoring --enable-autoprovisioning \
  --max-cpu 960 --max-memory 9600 --ephemeral-storage-local-ssd=count=2 \
  --autoprovisioning-scopes=logging.write,monitoring,devstorage.read_only,compute \
  --addons GcsFuseCsiDriver
  

## Configure a maintenance exclusion to prevent automatic upgrades for 160 days
START=$(date -I --date="-1 day")
END=$(date -I --date="+160 days")
gcloud container clusters update ${CLUSTER_NAME} --region ${REGION} \
    --add-maintenance-exclusion-name notouchy \
    --add-maintenance-exclusion-start ${START} \
    --add-maintenance-exclusion-end ${END} \
    --add-maintenance-exclusion-scope no_minor_or_node_upgrades
  

## Create L4 GPU nodepools
nodepool_args=(--spot --enable-autoscaling --enable-image-streaming
  --num-nodes=0 --min-nodes=0 --max-nodes=3 --cluster ${CLUSTER_NAME}
  --node-locations ${REGION}-a,${REGION}-b --region ${REGION} --async)

gcloud container node-pools create g2-standard-8 \
  --accelerator type=nvidia-l4,count=1,gpu-driver-version=latest \
  --machine-type g2-standard-8 --ephemeral-storage-local-ssd=count=1 \
  "${nodepool_args[@]}"

gcloud container node-pools create g2-standard-24 \
  --accelerator type=nvidia-l4,count=2,gpu-driver-version=latest \
  --machine-type g2-standard-24 --ephemeral-storage-local-ssd=count=2 \
  "${nodepool_args[@]}"

gcloud container node-pools create g2-standard-48 \
  --accelerator type=nvidia-l4,count=4,gpu-driver-version=latest \
  --machine-type g2-standard-48 --ephemeral-storage-local-ssd=count=4 \
  "${nodepool_args[@]}"


export ARTIFACTS_BUCKET="gs://${PROJECT_ID}-substratus-artifacts"
if ! gcloud storage buckets describe "${ARTIFACTS_BUCKET}" -q >/dev/null; then
## Create a bucket for substratus models and datasets
gcloud storage buckets create --project ${PROJECT_ID} "${ARTIFACTS_BUCKET}" \
  --location ${REGION}
## END
fi

# Create Artifact Registry to host container images
if ! gcloud artifacts repositories describe substratus --location us-central1 --project ${PROJECT_ID} -q > /dev/null; then
  gcloud artifacts repositories create substratus \
    --repository-format=docker --location=${REGION} \
    --project ${PROJECT_ID}
fi

# Create Google Service Account used by all of Substratus to access GCS and GAR
export SERVICE_ACCOUNT="substratus@${PROJECT_ID}.iam.gserviceaccount.com"
if ! gcloud iam service-accounts describe ${SERVICE_ACCOUNT} --project ${PROJECT_ID}; then
  gcloud iam service-accounts create substratus --project ${PROJECT_ID}
fi

# Give required permissions to Service Account
gcloud storage buckets add-iam-policy-binding ${ARTIFACTS_BUCKET} \
  --member="serviceAccount:${SERVICE_ACCOUNT}" --role=roles/storage.admin \
  --project ${PROJECT_ID}

gcloud artifacts repositories add-iam-policy-binding substratus \
  --location us-central1 --member="serviceAccount:${SERVICE_ACCOUNT}" \
  --role=roles/artifactregistry.admin --project ${PROJECT_ID}

# Allow the Service Account to bind K8s Service Account to this Service Account
gcloud iam service-accounts add-iam-policy-binding ${SERVICE_ACCOUNT} \
   --role roles/iam.serviceAccountAdmin --project ${PROJECT_ID} \
   --member "serviceAccount:${SERVICE_ACCOUNT}"

# Allow to create signed URLs
gcloud iam service-accounts add-iam-policy-binding ${SERVICE_ACCOUNT} \
   --role roles/iam.serviceAccountTokenCreator --project ${PROJECT_ID} \
   --member "serviceAccount:${SERVICE_ACCOUNT}"

gcloud iam service-accounts add-iam-policy-binding ${SERVICE_ACCOUNT} \
   --role roles/iam.workloadIdentityUser --project ${PROJECT_ID} \
   --member "serviceAccount:${PROJECT_ID}.svc.id.goog[substratus/sci]"

# Configure kubectl.
gcloud container clusters get-credentials --project ${PROJECT_ID} --region ${REGION} ${CLUSTER_NAME}
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
