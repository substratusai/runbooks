apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
metadata:
  name: default
spec:
  instanceProfile: eksctl-KarpenterNodeInstanceProfile-${CLUSTER_NAME}
  subnetSelector:
    karpenter.sh/discovery: ${CLUSTER_NAME}
  securityGroupSelector:
    karpenter.sh/discovery: ${CLUSTER_NAME}
---
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: p4-gpu
spec:
  providerRef:
    name: default
  consolidation:
    enabled: true
  labels:
    aws.amazon.com/eks-accelerator: nvidia-a100
  taints:
    - key: nvidia.com/gpu
      value: "true"
      effect: "NoSchedule"
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
    - key: karpenter.k8s.aws/instance-family
      operator: In
      values: ["p4"]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["amd64"]
---
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: p3-gpu
spec:
  providerRef:
    name: default
  consolidation:
    enabled: true
  labels:
    aws.amazon.com/eks-accelerator: nvidia-tesla-v100
  taints:
    - key: nvidia.com/gpu
      value: "true"
      effect: "NoSchedule"
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
    - key: karpenter.k8s.aws/instance-family
      operator: In
      values: ["p3"]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["amd64"]
---
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: p2-gpu
spec:
  providerRef:
    name: default
  consolidation:
    enabled: true
  labels:
    aws.amazon.com/eks-accelerator: nvidia-tesla-k80
  taints:
    - key: nvidia.com/gpu
      value: "true"
      effect: "NoSchedule"
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
    - key: karpenter.k8s.aws/instance-family
      operator: In
      values: ["p2"]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["amd64"]
---
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: g5-gpu
spec:
  providerRef:
    name: default
  consolidation:
    enabled: true
  labels:
    aws.amazon.com/eks-accelerator: nvidia-a10g
  taints:
    - key: nvidia.com/gpu
      value: "true"
      effect: "NoSchedule"
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
    - key: karpenter.k8s.aws/instance-family
      operator: In
      values: ["g5"]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["amd64"]
---
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: g4ad-gpu
spec:
  providerRef:
    name: default
  consolidation:
    enabled: true
  taints:
    - key: amd.com/gpu
      value: "true"
      effect: "NoSchedule"
    - key: aws.amazon.com/eks-accelerator
      value: "amd-radeon-pro-v520"
      effect: "NoSchedule"
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
    - key: karpenter.k8s.aws/instance-family
      operator: In
      values: ["g4ad"]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["amd64"]
---
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: g4dn-gpu
spec:
  providerRef:
    name: default
  consolidation:
    enabled: true
  labels:
    aws.amazon.com/eks-accelerator: nvidia-t4
  taints:
    - key: nvidia.com/gpu
      value: "true"
      effect: "NoSchedule"
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
    - key: karpenter.k8s.aws/instance-family
      operator: In
      values: ["g4dn"]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["amd64"]
---
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: g3-gpu
spec:
  providerRef:
    name: default
  consolidation:
    enabled: true
  labels:
    aws.amazon.com/eks-accelerator: nvidia-tesla-m60
  taints:
    - key: nvidia.com/gpu
      value: "true"
      effect: "NoSchedule"
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
    - key: karpenter.k8s.aws/instance-family
      operator: In
      values: ["g3"]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["amd64"]
---
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: g2-gpu
spec:
  providerRef:
    name: default
  consolidation:
    enabled: true
  labels:
    aws.amazon.com/eks-accelerator: nvidia-grid-k520
  taints:
    - key: nvidia.com/gpu
      value: "true"
      effect: "NoSchedule"
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
    - key: karpenter.k8s.aws/instance-family
      operator: In
      values: ["g2"]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["amd64"]
---
