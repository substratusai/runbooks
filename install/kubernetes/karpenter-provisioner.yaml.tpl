apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
metadata:
  name: default
spec:
  subnetSelector:
    karpenter.sh/discovery: ${CLUSTER_NAME}
  securityGroupSelector:
    karpenter.sh/discovery: ${CLUSTER_NAME}
---
# https://karpenter.sh/docs/getting-started/getting-started-with-karpenter/
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: gpu
spec:
  provider:
    instanceProfile: eksctl-KarpenterNodeInstanceProfile-${CLUSTER_NAME}
    subnetSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
    securityGroupSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
  ttlSecondsAfterEmpty: 30
  consolidation:
    enabled: true
  taints:
    - key: nvidia.com/gpu
      value: "true"
      effect: NoSchedule
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
    - key: node.kubernetes.io/instance-type
      operator: In
      values:
        # aws ec2 describe-instance-types --region us-west-2 --query "InstanceTypes[?GpuInfo!=null].InstanceType" --output json | jq -r '.[]' | sort | grep -v dl1 | grep -v inf | grep -v p5 | grep -v trn1 | awk '{print "\""$1"\","}'
        [
          "g2.2xlarge",
          "g2.8xlarge",
          "g3.16xlarge",
          "g3.4xlarge",
          "g3.8xlarge",
          "g3s.xlarge",
          "g4ad.16xlarge",
          "g4ad.2xlarge",
          "g4ad.4xlarge",
          "g4ad.8xlarge",
          "g4ad.xlarge",
          "g4dn.12xlarge",
          "g4dn.16xlarge",
          "g4dn.2xlarge",
          "g4dn.4xlarge",
          "g4dn.8xlarge",
          "g4dn.metal",
          "g4dn.xlarge",
          "g5.12xlarge",
          "g5.16xlarge",
          "g5.24xlarge",
          "g5.2xlarge",
          "g5.48xlarge",
          "g5.4xlarge",
          "g5.8xlarge",
          "g5.xlarge",
          "g5g.16xlarge",
          "g5g.2xlarge",
          "g5g.4xlarge",
          "g5g.8xlarge",
          "g5g.metal",
          "g5g.xlarge",
          "p2.16xlarge",
          "p2.8xlarge",
          "p2.xlarge",
          "p3.16xlarge",
          "p3.2xlarge",
          "p3.8xlarge",
          "p3dn.24xlarge",
          "p4d.24xlarge",
        ]
