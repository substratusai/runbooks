export IMAGE_REGISTRY=us-central1-docker.pkg.dev/cnp-demo-dev/substratus

# These variables are usually determined by the controller when it runs in the cluster.
export CLOUD=gcp
export GPU_TYPE=none
export GCP_PROJECT_ID=cnp-demo-dev
export GCP_CLUSTER_NAME=substratus
export GCP_CLUSTER_LOCATION=us-central1

gcloud container clusters get-credentials --region us-central1 substratus
