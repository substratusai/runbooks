apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: ${CLUSTER_NAME}
  region: ${REGION}
  version: "1.27"
  tags:
    createdBy: eksctl
    environment: dev
    karpenter.sh/discovery: ${CLUSTER_NAME}

karpenter:
  createServiceAccount: true
  withSpotInterruptionQueue: true
  defaultInstanceProfile: "KarpenterNodeInstanceProfile-${CLUSTER_NAME}"
  version: "v0.29.0"

# if karpenter doesn't suffice: https://github.com/eksctl-io/eksctl/blob/main/examples/23-kubeflow-spot-instance.yaml
managedNodeGroups:
  - name: builder-ng
    privateNetworking: true
    labels: { role: builders }
    instanceTypes:
      - t3a.small
    #  - m6a.large
    volumeSize: 100
    minSize: 0
    maxSize: 3
    desiredCapacity: 2
    iam:
      withAddonPolicies:
        ebs: true
        imageBuilder: true
addons:
  - name: vpc-cni
    attachPolicyARNs:
      - arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy
  - name: kube-proxy
  - name: aws-ebs-csi-driver
    wellKnownPolicies:
      ebsCSIController: true
  - name: coredns

iamIdentityMappings:
  - arn: "arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME}"
    username: system:node:{{EC2PrivateDNSName}}
    groups:
      - system:bootstrappers
      - system:nodes

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
    - metadata:
        name: ebs-csi-controller-sa
        namespace: kube-system
      wellKnownPolicies:
        ebsCSIController: true
    - metadata:
        name: ${CLUSTER_NAME}
        namespace: ${CLUSTER_NAME}
      attachPolicy:
        Version: "2012-10-17"
        Statement:
          - Effect: Allow
            Action:
              - "ecr:*"
            Resource:
              - "arn:aws:ecr:::${ARTIFACTS_REPO_NAME}"
          - Effect: Allow
            Action:
              - "s3:*"
              - "s3-object-lambda:*"
            Resource:
              - "arn:aws:s3:::${ARTIFACTS_BUCKET_NAME}/*"
              - "arn:aws:s3:::${ARTIFACTS_BUCKET_NAME}"
    - metadata:
        name: aws-manager
        namespace: ${CLUSTER_NAME}
      attachPolicy:
        # https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html
        Version: "2012-10-17"
        Statement:
          - Effect: Allow
            Action:
              - "s3:PutObject"
              - "s3:GetObject"
            Resource:
              - "arn:aws:s3:::${ARTIFACTS_BUCKET_NAME}/*"
              - "arn:aws:s3:::${ARTIFACTS_BUCKET_NAME}"
