#!/bin/bash

gcloud iam service-accounts create skaffold-container-builder \
  --project=${PROJECT_ID}
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member "serviceAccount:skaffold-container-builder@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role "roles/artifactregistry.admin"
gcloud iam service-accounts add-iam-policy-binding \
  skaffold-container-builder@${PROJECT_ID}.iam.gserviceaccount.com \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:${PROJECT_ID}.svc.id.goog[default/skaffold-container-builder]"

kubectl create ns substratus
kubectl create serviceaccount skaffold-container-builder \
  --namespace default
kubectl annotate serviceaccount skaffold-container-builder \
  --namespace default \
  iam.gke.io/gcp-service-account=skaffold-container-builder@${PROJECT_ID}.iam.gserviceaccount.com
