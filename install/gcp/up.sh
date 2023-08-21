#!/bin/bash

set -u
set -x

PROJECT_ID=${PROJECT_ID:=$(gcloud config get project)}
REGION=${REGION:-us-central1}
ZONE=${ZONE:=${REGION}-a}
INSTALL_OPERATOR=no # set to yes if you want to install operator

# Enable required services.
gcloud services enable container.googleapis.com
gcloud services enable artifactregistry.googleapis.com

export CLUSTER_NAME=substratus
gcloud container clusters create ${CLUSTER_NAME} --location ${REGION} \
  --machine-type n2d-standard-8 --num-nodes 1 --min-nodes 1 --max-nodes 5 \
  --node-locations ${ZONE} --workload-pool ${PROJECT_ID}.svc.id.goog \
  --enable-image-streaming --enable-shielded-nodes --shielded-secure-boot \
  --shielded-integrity-monitoring --enable-autoprovisioning \
  --max-cpu 960 --max-memory 9600 --ephemeral-storage-local-ssd=count=2 \
  --autoprovisioning-scopes=logging.write,monitoring,devstorage.read_only,compute \
  --addons GcsFuseCsiDriver

# Configure a maintenance exclusion to prevent automatic upgrades for 160 days
START=$(date -I --date="-1 day")
END=$(date -I --date="+160 days")
gcloud container clusters update ${CLUSTER_NAME} --region ${REGION} \
    --add-maintenance-exclusion-name notouchy \
    --add-maintenance-exclusion-start ${START} \
    --add-maintenance-exclusion-end ${END} \
    --add-maintenance-exclusion-scope no_minor_or_node_upgrades

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
gcloud storage buckets create "${ARTIFACTS_BUCKET}" --location ${REGION}
fi

export GAR_REPO_NAME=substratus
export REGISTRY_URL=${REGION}-docker.pkg.dev/${PROJECT_ID}/${GAR_REPO_NAME}
if ! gcloud artifacts repositories describe ${GAR_REPO_NAME} --location ${REGION} -q > /dev/null; then
gcloud artifacts repositories create ${GAR_REPO_NAME} \
  --repository-format=docker --location=${REGION}
fi

# Create Google Service Account used by all of Substratus to access GCS and GAR
export SERVICE_ACCOUNT_NAME=substratus
export SERVICE_ACCOUNT="${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
if ! gcloud iam service-accounts describe ${SERVICE_ACCOUNT}; then
gcloud iam service-accounts create ${SERVICE_ACCOUNT_NAME}
fi

# Give required permissions to Service Account
gcloud storage buckets add-iam-policy-binding ${ARTIFACTS_BUCKET} \
  --member="serviceAccount:${SERVICE_ACCOUNT}" --role=roles/storage.admin

gcloud artifacts repositories add-iam-policy-binding substratus \
  --location us-central1 --member="serviceAccount:${SERVICE_ACCOUNT}" \
  --role=roles/artifactregistry.admin

# Allow the Service Account to bind K8s Service Account to this Service Account
gcloud iam service-accounts add-iam-policy-binding ${SERVICE_ACCOUNT} \
   --role roles/iam.serviceAccountAdmin --member "serviceAccount:${SERVICE_ACCOUNT}"

# Allow to create signed URLs
gcloud iam service-accounts add-iam-policy-binding ${SERVICE_ACCOUNT} \
   --role roles/iam.serviceAccountTokenCreator \
   --member "serviceAccount:${SERVICE_ACCOUNT}"

gcloud iam service-accounts add-iam-policy-binding ${SERVICE_ACCOUNT} \
   --role roles/iam.workloadIdentityUser \
   --member "serviceAccount:${PROJECT_ID}.svc.id.goog[substratus/sci]"

# Configure kubectl.
gcloud container clusters get-credentials --region ${REGION} ${CLUSTER_NAME}
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
  ARTIFACT_BUCKET_URL: ${ARTIFACTS_BUCKET}
  REGISTRY_URL: ${REGISTRY_URL}
  PRINCIPAL: ${SERVICE_ACCOUNT}
EOF
kubectl apply -f https://raw.githubusercontent.com/substratusai/substratus/main/install/gcp/manifests.yaml
fi
