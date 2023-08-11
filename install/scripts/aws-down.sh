#!/bin/bash

set -e
set -u

# Required env variables:
: "$AWS_ACCOUNT_ID $AWS_ACCESS_KEY_ID $AWS_SECRET_ACCESS_KEY"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
kubernetes_dir=${script_dir}/../kubernetes

EKSCTL_ENABLE_CREDENTIAL_CACHE=1
export CLUSTER_NAME=substratus
export REGION=us-west-2
export ARTIFACTS_REPO_NAME=${CLUSTER_NAME}
export ARTIFACTS_BUCKET_NAME=${AWS_ACCOUNT_ID}-${CLUSTER_NAME}-artifacts

(aws eks update-kubeconfig \
  --region ${REGION} \
  --name ${CLUSTER_NAME} &&
  kubectl delete deployments --namespace=karpenter --all &&
  kubectl delete deployments --namespace=kube-system --all) ||
  true

instance_ids=$(aws ec2 describe-instances \
  --filters "Name=tag:aws:eks:cluster-name,Values=${CLUSTER_NAME}" "Name=tag:karpenter.sh/managed-by,Values=${CLUSTER_NAME}" \
  --query "Reservations[*].Instances[*].InstanceId" \
  --output text \
  --region ${REGION})

if [[ ! -z "${instance_ids}" ]]; then
  aws ec2 terminate-instances --instance-ids ${instance_ids} --region ${REGION}
else
  echo "No instances found with the specified tags."
fi

envsubst <${kubernetes_dir}/aws/eks-cluster.yaml.tpl >${kubernetes_dir}/aws/eks-cluster.yaml
eksctl delete cluster -f ${kubernetes_dir}/aws/eks-cluster.yaml || true

aws cloudformation delete-stack \
  --stack-name "Karpenter-${CLUSTER_NAME}" \
  --region ${REGION} || true

aws ecr delete-repository \
  --repository-name ${ARTIFACTS_REPO_NAME} \
  --region ${REGION} >/dev/null || true

aws s3 rb s3://${ARTIFACTS_BUCKET_NAME} \
  --region ${REGION} >/dev/null || true
