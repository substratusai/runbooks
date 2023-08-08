#!/bin/bash

set -e
set -u

# Required env variables:
# : "$TOKEN $PROJECT"

export EKSCTL_ENABLE_CREDENTIAL_CACHE=1
export CLUSTER_NAME=substratus
export REGION=us-west-2
export ARTIFACTS_REPO_NAME=substratus
export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export ARTIFACTS_BUCKET_NAME=${AWS_ACCOUNT_ID}-substratus-artifacts

aws s3 rb s3://${ARTIFACTS_BUCKET_NAME} --region ${REGION} || true
aws ecr delete-repository --repository-name ${ARTIFACTS_REPO_NAME} || true

aws cloudformation delete-stack \
  --stack-name "Karpenter-${CLUSTER_NAME}" || true

envsubst <../kubernetes/eks-cluster.yaml.tpl >../kubernetes/eks-cluster.yaml
eksctl delete cluster -f ../kubernetes/eks-cluster.yaml
