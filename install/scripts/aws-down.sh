#!/bin/bash

set -e
set -u

# Required env variables:
# : "$TOKEN $PROJECT"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KUBERENTES_DIR=${SCRIPT_DIR}/../kubernetes

export EKSCTL_ENABLE_CREDENTIAL_CACHE=1
export CLUSTER_NAME=substratus
export REGION=us-west-2
export ARTIFACTS_REPO_NAME=substratus
export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export ARTIFACTS_BUCKET_NAME=${AWS_ACCOUNT_ID}-substratus-artifacts

aws s3 rb s3://${ARTIFACTS_BUCKET_NAME} --region ${REGION} >/dev/null || true
aws ecr delete-repository --repository-name ${ARTIFACTS_REPO_NAME} >/dev/null || true
aws cloudformation delete-stack \
  --stack-name "Karpenter-${CLUSTER_NAME}" || true

envsubst <${KUBERENTES_DIR}/eks-cluster.yaml.tpl >${KUBERENTES_DIR}/eks-cluster.yaml
eksctl delete cluster -f ${KUBERENTES_DIR}/eks-cluster.yaml
