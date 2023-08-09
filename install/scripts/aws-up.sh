#!/bin/bash

set -e
set -u

# Required env variables:
: "$AWS_ACCOUNT_ID $AWS_ACCESS_KEY_ID $AWS_SECRET_ACCESS_KEY"

install_operator="${INSTALL_OPERATOR:-yes}"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
kubernetes_dir=${script_dir}/../kubernetes

eksctl_enable_credential_cache=1
export CLUSTER_NAME=substratus
export REGION=us-west-2
export ARTIFACTS_REPO_NAME=${CLUSTER_NAME}
export ARTIFACTS_BUCKET_NAME=${AWS_ACCOUNT_ID}-${CLUSTER_NAME}-artifacts
tempout=$(mktemp)

aws s3 mb s3://${ARTIFACTS_BUCKET_NAME} \
  --region ${REGION} >/dev/null || true

aws ecr create-repository \
  --repository-name ${ARTIFACTS_REPO_NAME} \
  --region ${REGION} >/dev/null || true

envsubst <${kubernetes_dir}/aws/eks-cluster.yaml.tpl >${kubernetes_dir}/aws/eks-cluster.yaml
eksctl create cluster -f ${kubernetes_dir}/aws/eks-cluster.yaml ||
  eksctl upgrade cluster -f ${kubernetes_dir}/aws/eks-cluster.yaml

aws iam create-service-linked-role \
  --aws-service-name spot.amazonaws.com || true

aws eks update-kubeconfig \
  --region ${REGION} \
  --name ${CLUSTER_NAME}

envsubst <${kubernetes_dir}/aws/karpenter-provisioner.yaml.tpl >${kubernetes_dir}/aws/karpenter-provisioner.yaml
kubectl apply -f ${kubernetes_dir}/aws/karpenter-provisioner.yaml

# nvidia-device-plugin: https://github.com/NVIDIA/k8s-device-plugin#deployment-via-helm
helm repo add nvdp https://nvidia.github.io/k8s-device-plugin
helm upgrade \
  --install nvdp nvdp/nvidia-device-plugin \
  --namespace nvidia-device-plugin \
  --create-namespace \
  --values ${kubernetes_dir}/aws/nvidia-eks-device-plugin.yaml \
  --version 0.14.1

# Install the substratus operator.
if [ "${install_operator}" == "yes" ]; then
  kubectl apply -f ${kubernetes_dir}/namespace.yaml
  kubectl apply -f ${kubernetes_dir}/config.yaml
  kubectl apply -f ${kubernetes_dir}/system.yaml
fi
