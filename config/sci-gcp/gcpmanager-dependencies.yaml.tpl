# also created by gcpmanager-dependencies.sh, but having it here ensures cleanup
# TODO Update skaffold to make it work
apiVersion: v1
kind: Namespace
metadata:
  name: substratus
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: skaffold-container-builder
  namespace: default
  annotations:
    iam.gke.io/gcp-service-account: skaffold-container-builder@${PROJECT_ID}.iam.gserviceaccount.com
