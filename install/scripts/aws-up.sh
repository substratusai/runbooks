#!/bin/bash

set -e
set -u

# Required env variables:
# : "$TOKEN $PROJECT"

# # TODO(bjb): pass AWS creds into script
# export CLOUDSDK_AUTH_ACCESS_TOKEN=${TOKEN}

# INSTALL_OPERATOR="${INSTALL_OPERATOR:-yes}"
export EKSCTL_ENABLE_CREDENTIAL_CACHE=1
export CLUSTER_NAME=substratus
export REGION=us-west-2
export ARTIFACTS_REPO_NAME=substratus
export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export ARTIFACTS_BUCKET_NAME=${AWS_ACCOUNT_ID}-substratus-artifacts

aws s3 mb s3://${ARTIFACTS_BUCKET_NAME} --region ${REGION} || true
aws ecr create-repository --repository-name ${ARTIFACTS_REPO_NAME} || true

# install karpenter: https://karpenter.sh/docs/getting-started/getting-started-with-karpenter/
export KARPENTER_VERSION=v0.29.2
export AWS_PARTITION="aws"
export TEMPOUT=$(mktemp)
curl -fsSL https://raw.githubusercontent.com/aws/karpenter/"${KARPENTER_VERSION}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml >$TEMPOUT &&
  aws cloudformation deploy \
    --stack-name "Karpenter-${CLUSTER_NAME}" \
    --template-file "${TEMPOUT}" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides "ClusterName=${CLUSTER_NAME}"

envsubst <../kubernetes/eks-cluster.yaml.tpl >../kubernetes/eks-cluster.yaml
eksctl create cluster -f ../kubernetes/eks-cluster.yaml || eksctl upgrade cluster -f ../kubernetes/eks-cluster.yaml

export KARPENTER_IAM_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
aws iam create-service-linked-role --aws-service-name spot.amazonaws.com || true
aws eks --region ${REGION} update-kubeconfig --name ${CLUSTER_NAME}
# Logout of helm registry to perform an unauthenticated pull against the public ECR
helm registry logout public.ecr.aws || true

helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version ${KARPENTER_VERSION} --namespace karpenter --create-namespace \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
  --set settings.aws.clusterName=${CLUSTER_NAME} \
  --set settings.aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
  --set settings.aws.interruptionQueueName=${CLUSTER_NAME} \
  --set controller.resources.requests.cpu=1 \
  --set controller.resources.requests.memory=1Gi \
  --set controller.resources.limits.cpu=1 \
  --set controller.resources.limits.memory=1Gi \
  --wait

envsubst <../kubernetes/karpenter-provisioner.yaml.tpl >../kubernetes/karpenter-provisioner.yaml.yaml
kubectl apply -f ../kubernetes/karpenter-provisioner.yaml

# node-termination-handler: https://artifacthub.io/packages/helm/aws/aws-node-termination-handler
helm repo add eks https://aws.github.io/eks-charts
helm upgrade \
  --install aws-node-termination-handler \
  --namespace kube-system \
  --version 0.21.0 \
  eks/aws-node-termination-handler

# Install the substratus operator.
# if [ "${INSTALL_OPERATOR}" == "yes" ]; then
#   kubectl apply -f kubernetes/namespace.yaml
#   kubectl apply -f kubernetes/config.yaml
#   kubectl apply -f kubernetes/system.yaml
# fi
