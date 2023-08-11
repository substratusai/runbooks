apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
metadata:
  name: default
spec:
  instanceProfile: KarpenterNodeInstanceProfile-${CLUSTER_NAME}
  subnetSelector:
    karpenter.sh/discovery: ${CLUSTER_NAME}
  securityGroupSelector:
    karpenter.sh/discovery: ${CLUSTER_NAME}
---
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: nvidia-gpu
spec:
  providerRef:
    name: default
  consolidation:
    enabled: true
  # These well-known labels (specifically karpenter.k8s.aws/instance-gpu-name)
  # will guide karpenter in accelerator and instance type selection:
  # https://karpenter.sh/v0.29/concepts/scheduling/#labels
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
      values: [
        "p4de",
        "p4d",
        "p3dn",
        "p3",
        "p2",
        "g2",
        "g3",
        "g4",
        "g5",
      ]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["amd64"]
