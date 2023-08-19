#!/bin/bash

set -e
set -u

# Required env variables:
: "$AWS_ACCOUNT_ID $AWS_ACCESS_KEY_ID $AWS_SECRET_ACCESS_KEY"
install_operator="${INSTALL_OPERATOR:-true}"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
kubernetes_dir=${script_dir}/../kubernetes

EKSCTL_ENABLE_CREDENTIAL_CACHE=1
karpenter_version=v0.29.2
export CLUSTER_NAME=substratus
export AWS_REGION=us-west-2
export ARTIFACTS_REPO_NAME=${CLUSTER_NAME}
export ARTIFACTS_BUCKET_NAME=${AWS_ACCOUNT_ID}-${CLUSTER_NAME}-artifacts
karpenter_iam_role_arn="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
tempout=$(mktemp)

aws s3 mb s3://${ARTIFACTS_BUCKET_NAME} \
  --region ${AWS_REGION} >/dev/null || true

aws ecr create-repository \
  --repository-name ${ARTIFACTS_REPO_NAME} \
  --region ${AWS_REGION} >/dev/null || true

curl -fsSL https://raw.githubusercontent.com/aws/karpenter/"${karpenter_version}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml >$tempout
aws cloudformation deploy \
  --stack-name "Karpenter-${CLUSTER_NAME}" \
  --template-file "${tempout}" \
  --capabilities CAPABILITY_NAMED_IAM \
  --region ${AWS_REGION} \
  --parameter-overrides "ClusterName=${CLUSTER_NAME}" || true

envsubst <${kubernetes_dir}/aws/templates/eks-cluster.yaml.tpl >${kubernetes_dir}/aws/eks-cluster.yaml
eksctl create cluster -f ${kubernetes_dir}/aws/eks-cluster.yaml ||
  eksctl upgrade cluster -f ${kubernetes_dir}/aws/eks-cluster.yaml

aws iam create-service-linked-role \
  --aws-service-name spot.amazonaws.com || true

# Logout of helm registry to perform an unauthenticated pull against the public ECR
helm registry logout public.ecr.aws || true
echo "---" >>${kubernetes_dir}/aws/system.yaml
helm template karpenter oci://public.ecr.aws/karpenter/karpenter \
  --create-namespace \
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
  >>${kubernetes_dir}/aws/system.yaml

echo "---" >>${kubernetes_dir}/aws/system.yaml
envsubst <${kubernetes_dir}/aws/templates/karpenter-provisioner.yaml.tpl >>${kubernetes_dir}/aws/system.yaml

# nvidia-device-plugin: https://github.com/NVIDIA/k8s-device-plugin#deployment-via-helm
helm repo add nvdp https://nvidia.github.io/k8s-device-plugin
echo "---" >>${kubernetes_dir}/aws/system.yaml
helm template nvdp nvdp/nvidia-device-plugin \
  --namespace nvidia-device-plugin \
  --create-namespace \
  --values ${kubernetes_dir}/aws/values/nvidia-eks-device-plugin.yaml \
  --version 0.14.1 \
  >>${kubernetes_dir}/aws/system.yaml

# Install the substratus operator.

if [ "${install_operator}" == "true" ]; then
  # kubectl apply -f ${kubernetes_dir}/namespace.yaml
  # kubectl apply -f ${kubernetes_dir}/config.yaml
  kubectl apply -f ${kubernetes_dir}/aws/system.yaml |
    grep -v 'is missing the kubectl.kubernetes.io/last-applied-configuration'
fi
