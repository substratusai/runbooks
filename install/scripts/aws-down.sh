#!/bin/bash

set -e
set -u

# Required env variables:
: "$AWS_ACCOUNT_ID $AWS_ACCESS_KEY_ID $AWS_SECRET_ACCESS_KEY"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KUBERENTES_DIR=${SCRIPT_DIR}/../kubernetes

EKSCTL_ENABLE_CREDENTIAL_CACHE=1
export CLUSTER_NAME=substratus
export REGION=us-west-2
export ARTIFACTS_REPO_NAME=${CLUSTER_NAME}
export ARTIFACTS_BUCKET_NAME=${AWS_ACCOUNT_ID}-${CLUSTER_NAME}-artifacts

(aws eks update-kubeconfig \
  --region ${REGION} \
  --name ${CLUSTER_NAME} &&
  kubectl delete deployments --namespace=karpenter --all ||
  kubectl delete deployments --namespace=kube-system --all) ||
  true

aws iam delete-policy \
  --policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/KarpenterControllerPolicy-${CLUSTER_NAME} ||
  true

aws cloudformation delete-stack \
  --stack-name "Karpenter-${CLUSTER_NAME}" \
  --region ${REGION} || true

envsubst <${KUBERENTES_DIR}/eks-cluster.yaml.tpl >${KUBERENTES_DIR}/eks-cluster.yaml
cat ${KUBERENTES_DIR}/eks-cluster.yaml
eksctl delete cluster -f ${KUBERENTES_DIR}/eks-cluster.yaml || true

aws ecr delete-repository \
  --repository-name ${ARTIFACTS_REPO_NAME} \
  --region ${REGION} >/dev/null || true

aws s3 rb s3://${ARTIFACTS_BUCKET_NAME} \
  --region ${REGION} >/dev/null || true
