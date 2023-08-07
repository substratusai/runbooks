#!/bin/bash

set -e
set -u

# Required env variables:
: "$TOKEN $PROJECT"

# Used by gcloud:
# TODO(bjb): pass AWS creds into script
export CLOUDSDK_AUTH_ACCESS_TOKEN=${TOKEN}
# Used by terraform:
export GOOGLE_OAUTH_ACCESS_TOKEN=${TOKEN}

INSTALL_OPERATOR="${INSTALL_OPERATOR:-yes}"
AUTO_APPROVE="${AUTO_APPROVE:-no}"

# Create terraform state bucket if one does not exist.
# TODO(bjb): establish a bucket

# Apply infrastructure.
cd terraform/aws

# Backend variables cannot be configured via env variables.
echo "bucket = \"${TF_BUCKET}\"" >>backend.tfvars
terraform init --backend-config=backend.tfvars

export TF_VAR_project_id=${PROJECT}
if [ "${AUTO_APPROVE}" == "yes" ]; then
  terraform apply -auto-approve
else
  terraform apply
fi
CLUSTER_NAME=$(terraform output --json cluster | jq -r '.name')
CLUSTER_REGION=$(terraform output --json cluster | jq -r '.region')
CLUSTER_ENDPOINT=$(terraform output --json cluster | jq -r '.endpoint')
LOAD_BALANCER_CONTROLLER_ROLE_NAME=$(terraform output --json irsas | jq -r '.load_balancer_controller_irsa_role.iam_role_name')

cd -

# Configure kubectl.
aws eks --region ${CLUSTER_REGION} update-kubeconfig --name ${CLUSTER_NAME}
# Install cluster-level components

# node-termination-handler: https://artifacthub.io/packages/helm/aws/aws-node-termination-handler
helm repo add eks https://aws.github.io/eks-charts
helm upgrade \
  --install aws-node-termination-handler \
  --namespace kube-system \
  --version 0.21.0 \
  eks/aws-node-termination-handler

# install EBS snapshotter?: https://github.com/kubernetes-csi/external-snapshotter#usage

# TODO(bjb): may not be needed if we can resolve 401 to 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/
# install aws-ebs-csi-driver: https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/install.md
helm repo add aws-ebs-csi-driver https://kubernetes-sigs.github.io/aws-ebs-csi-driver
helm repo update
helm upgrade \
  --install aws-ebs-csi-driver \
  --namespace kube-system \
  aws-ebs-csi-driver/aws-ebs-csi-driver

# TODO(bjb): is this needed? Is doing the work here preferred to doing it in terraform?
# install karpenter: https://karpenter.sh/docs/getting-started/getting-started-with-karpenter/
export KARPENTER_VERSION=v0.29.2
export AWS_PARTITION="aws"
export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export TEMPOUT=$(mktemp)
curl -fsSL https://raw.githubusercontent.com/aws/karpenter/"${KARPENTER_VERSION}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml >$TEMPOUT &&
  aws cloudformation deploy \
    --stack-name "Karpenter-${CLUSTER_NAME}" \
    --template-file "${TEMPOUT}" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides "ClusterName=${CLUSTER_NAME}"

eksctl create cluster -f - <<EOF
---
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: ${CLUSTER_NAME}
  region: ${CLUSTER_REGION}
  version: "1.27"
  tags:
    karpenter.sh/discovery: ${CLUSTER_NAME}

iam:
  withOIDC: true
  serviceAccounts:
  - metadata:
      name: karpenter
      namespace: karpenter
    roleName: ${CLUSTER_NAME}-karpenter
    attachPolicyARNs:
    - arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:policy/KarpenterControllerPolicy-${CLUSTER_NAME}
    roleOnly: true

iamIdentityMappings:
- arn: "arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME}"
  username: system:node:{{EC2PrivateDNSName}}
  groups:
  - system:bootstrappers
  - system:nodes

managedNodeGroups:
- instanceType: t3a.large
  amiFamily: AmazonLinux2
  name: ${CLUSTER_NAME}-ng
  desiredCapacity: 1
  minSize: 0
  maxSize: 3
EOF

export KARPENTER_IAM_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
echo $CLUSTER_ENDPOINT $KARPENTER_IAM_ROLE_ARN
aws iam create-service-linked-role --aws-service-name spot.amazonaws.com || true

# install the load balancer controller: https://docs.aws.amazon.com/eks/latest/userguide/aws-load-balancer-controller.html
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=${CLUSTER_NAME} \
  --set serviceAccount.create=false \
  --set serviceAccount.name=${LOAD_BALANCER_CONTROLLER_ROLE_NAME}

# Install the substratus operator.
# if [ "${INSTALL_OPERATOR}" == "yes" ]; then
#   kubectl apply -f kubernetes/namespace.yaml
#   kubectl apply -f kubernetes/config.yaml
#   kubectl apply -f kubernetes/system.yaml
# fi
