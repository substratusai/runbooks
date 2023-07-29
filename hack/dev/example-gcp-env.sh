# These variables are usually determined by the controller when it runs in the cluster.
export CLOUD=gcp
export GPU_TYPE=nvidia-l4
export PROJECT_ID=eminent-will-390401
export CLUSTER_NAME=substratus
export CLUSTER_LOCATION=us-central1

mkdir -p secrets
gcloud iam service-accounts keys create --iam-account=substratus-gcp-manager@${PROJECT_ID}.iam.gserviceaccount.com ./secrets/gcp-manager-key.json