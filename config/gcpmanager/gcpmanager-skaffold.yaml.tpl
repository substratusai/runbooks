apiVersion: skaffold/v3
kind: Config
metadata:
  name: gcpmanager
  labels: {}
build:
  tagPolicy:
    gitCommit: {}
  platforms: ["linux/amd64", "darwin/arm64"]
  cluster:
    resources:
      requests:
        cpu: 300m
        memory: 512Mi
    serviceAccount: container-builder
  artifacts:
    - image: us-central1-docker.pkg.dev/${PROJECT_ID}/substratus/gcpmanager
      # - image: substratusai/gcp-manager
      kaniko:
        dockerfile: Dockerfile.gcpmanager
        logFormat: text
        logTimestamp: true
        reproducible: true
        useNewRun: true
        verbosity: info
        cache:
          ttl: "24h"
manifests:
  rawYaml:
    - "config/gcpmanager/gcp-manager.yaml"
    - "config/gcpmanager/bootstrapper-job.yaml"
