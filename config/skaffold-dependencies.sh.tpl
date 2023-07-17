#!/bin/bash

gcloud iam service-accounts create skaffold-container-builder --project=${PROJECT_ID}
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member "serviceAccount:skaffold-container-builder@${PROJECT_ID}.iam.gserviceaccount.com" --role "roles/artifactregistry.writer"
gcloud iam service-accounts add-iam-policy-binding substratus-container-builder@${PROJECT_ID}.iam.gserviceaccount.com --role roles/iam.workloadIdentityUser --member "serviceAccount:${PROJECT_ID}.svc.id.goog[default/container-builder]"

kubectl create serviceaccount container-builder --namespace default
kubectl annotate serviceaccount container-builder \
  --namespace default \
  iam.gke.io/gcp-service-account=substratus-container-builder@${PROJECT_ID}.iam.gserviceaccount.com
