apiVersion: v1
kind: ServiceAccount
metadata:
  annotations:
    iam.gke.io/gcp-service-account: substratus-container-builder@${PROJECT_ID}.iam.gserviceaccount.com
  name: container-builder
  namespace: default
