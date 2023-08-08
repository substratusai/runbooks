#!/bin/bash

set -e
set -u

# Required env variables:
: "$AWS_ACCOUNT_ID $AWS_ACCESS_KEY_ID $AWS_SECRET_ACCESS_KEY"

INSTALL_OPERATOR="${INSTALL_OPERATOR:-yes}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KUBERENTES_DIR=${SCRIPT_DIR}/../kubernetes

EKSCTL_ENABLE_CREDENTIAL_CACHE=1
export CLUSTER_NAME=substratus
export REGION=us-west-2
export ARTIFACTS_REPO_NAME=${CLUSTER_NAME}
export ARTIFACTS_BUCKET_NAME=${AWS_ACCOUNT_ID}-${CLUSTER_NAME}-artifacts
export KARPENTER_VERSION=v0.29.2
export AWS_PARTITION="aws"
export KARPENTER_IAM_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
TEMPOUT=$(mktemp)

aws s3 mb s3://${ARTIFACTS_BUCKET_NAME} \
  --region ${REGION} >/dev/null || true

aws ecr create-repository \
  --repository-name ${ARTIFACTS_REPO_NAME} \
  --region ${REGION} >/dev/null || true

# install karpenter: https://karpenter.sh/docs/getting-started/getting-started-with-karpenter/
curl -fsSL https://raw.githubusercontent.com/aws/karpenter/"${KARPENTER_VERSION}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml >$TEMPOUT &&
  aws cloudformation deploy \
    --stack-name "Karpenter-${CLUSTER_NAME}" \
    --template-file "${TEMPOUT}" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides "ClusterName=${CLUSTER_NAME}" \
    --region ${REGION}

envsubst <${KUBERENTES_DIR}/eks-cluster.yaml.tpl >${KUBERENTES_DIR}/eks-cluster.yaml
cat ${KUBERENTES_DIR}/eks-cluster.yaml
eksctl create cluster -f ${KUBERENTES_DIR}/eks-cluster.yaml ||
  eksctl upgrade cluster -f ${KUBERENTES_DIR}/eks-cluster.yaml

aws iam create-service-linked-role \
  --aws-service-name spot.amazonaws.com || true

aws eks update-kubeconfig \
  --region ${REGION} \
  --name ${CLUSTER_NAME}

# Logout of helm registry to perform an unauthenticated pull against the public ECR
helm registry logout public.ecr.aws || true
helm upgrade \
  --create-namespace \
  --install karpenter oci://public.ecr.aws/karpenter/karpenter \
  --version ${KARPENTER_VERSION} \
  --namespace karpenter \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
  --set settings.aws.clusterName=${CLUSTER_NAME} \
  --set settings.aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
  --set settings.aws.interruptionQueueName=${CLUSTER_NAME} \
  --set controller.resources.requests.cpu=1 \
  --set controller.resources.requests.memory=1Gi \
  --set controller.resources.limits.cpu=1 \
  --set controller.resources.limits.memory=1Gi \
  --wait

envsubst <${KUBERENTES_DIR}/karpenter-provisioner.yaml.tpl >${KUBERENTES_DIR}/karpenter-provisioner.yaml
cat ${KUBERENTES_DIR}/karpenter-provisioner.yaml
kubectl apply -f ${KUBERENTES_DIR}/karpenter-provisioner.yaml

# node-termination-handler: https://artifacthub.io/packages/helm/aws/aws-node-termination-handler
helm repo add eks https://aws.github.io/eks-charts
helm upgrade \
  --install aws-node-termination-handler \
  --namespace kube-system \
  --version 0.21.0 \
  eks/aws-node-termination-handler

# nvidia-device-plugin: https://github.com/NVIDIA/k8s-device-plugin#deployment-via-helm
helm repo add nvdp https://nvidia.github.io/k8s-device-plugin
helm upgrade \
  --install nvdp nvdp/nvidia-device-plugin \
  --namespace nvidia-device-plugin \
  --create-namespace \
  --version 0.14.1

# Install the substratus operator.
if [ "${INSTALL_OPERATOR}" == "yes" ]; then
  kubectl apply -f ${KUBERENTES_DIR}/namespace.yaml
  kubectl apply -f ${KUBERENTES_DIR}/config.yaml
  kubectl apply -f ${KUBERENTES_DIR}/system.yaml
fi
