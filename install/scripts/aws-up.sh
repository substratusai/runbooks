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
export karpenter_version=v0.29.2
export karpenter_iam_role_arn="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
tempout=$(mktemp)

aws s3 mb s3://${ARTIFACTS_BUCKET_NAME} \
  --region ${REGION} >/dev/null || true

aws ecr create-repository \
  --repository-name ${ARTIFACTS_REPO_NAME} \
  --region ${REGION} >/dev/null || true

# install karpenter: https://karpenter.sh/docs/getting-started/getting-started-with-karpenter/
curl -fsSL https://raw.githubusercontent.com/aws/karpenter/"${karpenter_version}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml >$tempout &&
  aws cloudformation deploy \
    --stack-name "Karpenter-${CLUSTER_NAME}" \
    --template-file "${tempout}" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides "ClusterName=${CLUSTER_NAME}" \
    --region ${REGION}

envsubst <${kubernetes_dir}/aws/eks-cluster.yaml.tpl >${kubernetes_dir}/aws/eks-cluster.yaml
eksctl create cluster -f ${kubernetes_dir}/aws/eks-cluster.yaml ||
  eksctl upgrade cluster -f ${kubernetes_dir}/aws/eks-cluster.yaml

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
  --version ${karpenter_version} \
  --namespace karpenter \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${karpenter_iam_role_arn} \
  --set settings.aws.clusterName=${CLUSTER_NAME} \
  --set settings.aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
  --set settings.aws.interruptionQueueName=${CLUSTER_NAME} \
  --set controller.resources.requests.cpu=1 \
  --set controller.resources.requests.memory=1Gi \
  --set controller.resources.limits.cpu=1 \
  --set controller.resources.limits.memory=1Gi \
  --wait

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
