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
  consolidation:
    enabled: true
  taints:
    - key: nvidia.com/gpu
      value: "true"
      effect: "NoSchedule"
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
    - key: karpenter.k8s.aws/instance-category
      operator: In
      values: ["g", "p"]
    - key: karpenter.k8s.aws/instance-family
      operator: NotIn
      values: ["p5"]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["amd64"]
