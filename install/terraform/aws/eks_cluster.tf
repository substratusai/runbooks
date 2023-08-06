locals {
  create_cluster = var.existing_eks_cluster == null ? 1 : 0
  # We need to lookup K8s taint effect from the AWS API value
  taint_effects = {
    NO_SCHEDULE        = "NoSchedule"
    NO_EXECUTE         = "NoExecute"
    PREFER_NO_SCHEDULE = "PreferNoSchedule"
  }

  # The following locals are used to configure tags for the EKS cluster's Auto
  # Scaling Groups managed by the cluster autoscaler.

  # `cluster_autoscaler_label_tags` contains the tags related to the Kubernetes
  # labels applied to the nodes in the cluster's managed node groups.
  # Each tag has a key formed from the node group's name and label name, and a
  # value containing the autoscaling group's name, the corresponding
  # Kubernetes label key, and its value. These tags are used by the cluster
  # autoscaler to determine how nodes should be scaled based on their labels.
  cluster_autoscaler_label_tags = local.eks_cluster.managed_node_groups != null ? merge([
    for name, group in local.eks_cluster.managed_node_groups : {
      for label_name, label_value in coalesce(group.node_group_labels, {}) : "${name}|label|${label_name}" => {
        autoscaling_group = group.node_group_autoscaling_group_names[0],
        key               = "k8s.io/cluster-autoscaler/node-template/label/${label_name}",
        value             = label_value,
      }
    }
  ]...) : {}

  # `cluster_autoscaler_taint_tags`  contains tags related to the Kubernetes
  # taints applied to the nodes in the cluster's managed node groups.
  # Each tag's key includes the node group's name and taint key, and its value
  # contains information about the taint, such as its value and effect.
  # These tags allow the cluster autoscaler to respect the taints when scaling nodes.
  cluster_autoscaler_taint_tags = local.eks_cluster.managed_node_groups != null ? merge([
    for name, group in local.eks_cluster.managed_node_groups : {
      for taint in coalesce(group.node_group_taints, []) : "${name}|taint|${taint.key}" => {
        autoscaling_group = group.node_group_autoscaling_group_names[0],
        key               = "k8s.io/cluster-autoscaler/node-template/taint/${taint.key}"
        value             = "${taint.value}:${local.taint_effects[taint.effect]}"
      }
    }
  ]...) : {}

  # `cluster_autoscaler_asg_tags` combines the above label and taint tags into a
  # single map, which is then used to create the actual tags on the AWS ASGs
  # through the `aws_autoscaling_group_tag` resource. The tags are only applied
  # if `existing_eks_cluster` is `null`, ensuring they are only created for new
  # clusters.
  cluster_autoscaler_asg_tags = merge(
    local.cluster_autoscaler_label_tags,
    local.cluster_autoscaler_taint_tags
  )
}

data "aws_ec2_instance_types" "gpu" {
  filter {
    name = "instance-type"
    # from: aws ec2 describe-instance-types --region us-west-2 --query "InstanceTypes[?GpuInfo!=null].InstanceType" --output json | jq -r '.[]' | awk -F. '{print "\"" $1 ".*\","}' | uniq
    # non-CUDA supported types added and commented out for now though these have accelerators of some kind
    values = [
      # "dl1.*", # no CUDA support
      # "inf1.*" # no CUDA support
      # "inf2.*" # no CUDA support
      "g2.*",
      "g3.*",
      "g3s.*",
      "g4ad.*",
      "g4dn.*",
      "g5.*",
      # "g5g.*", exclude g5g as these are ARM machines
      "p2.*",
      "p3.*",
      "p3dn.*",
      "p4d.*",
      # "p5.*", # no CUDA support
      # "trn1.*", # no CUDA support
      # "trn1n32.*", # no CUDA support
    ]
  }
}

data "aws_ami" "eks_default" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["amazon-eks-node-${var.cluster_version}-v*"]
  }
  filter {
    name   = "architecture"
    values = ["x86_64"]
  }
}

data "aws_ami" "deep_learning" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name = "name"
    # they don't produce images on any Ubuntu OS newer than this :shrug:
    values = ["Deep Learning AMI (Ubuntu 18.04) Version ??.?"]
  }
  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

  filter {
    name   = "state"
    values = ["available"]
  }
}

module "eks" {
  count                          = local.create_cluster
  source                         = "terraform-aws-modules/eks/aws"
  version                        = "19.15.4"
  cluster_name                   = var.name_prefix
  cluster_version                = var.cluster_version
  cluster_endpoint_public_access = true
  cluster_ip_family              = "ipv4"
  vpc_id                         = local.vpc.id
  subnet_ids                     = local.vpc.private_subnet_ids
  control_plane_subnet_ids       = local.vpc.intra_subnet_ids
  manage_aws_auth_configmap      = true

  eks_managed_node_group_defaults = {
    # We are using the IRSA created below for permissions
    # However, we have to deploy with the policy attached FIRST (when creating a fresh cluster)
    # and then turn this off after the cluster/node group is created. Without this initial policy,
    # the VPC CNI fails to assign IPs and nodes cannot join the cluster
    # See https://github.com/aws/containers-roadmap/issues/1666 for more context
    iam_role_attach_cni_policy = true
    subnet_ids                 = local.vpc.private_subnet_ids
    labels                     = var.labels
    ebs_optimized              = true
    disable_api_termination    = false
    enable_monitoring          = true
    use_custom_launch_template = false
    force_update_version       = true
  }

  eks_managed_node_groups = {
    builder = {
      # By default, the module creates a launch template to ensure tags are propagated to instances, etc.,
      # so we need to disable it to use the default template provided by the AWS EKS managed node group service
      name_prefix  = "container-builder"
      ami_id       = data.aws_ami.eks_default.image_id
      disk_size    = 100
      min_size     = 1
      max_size     = 3
      desired_size = 1
      instance_types = [
        "t3a.large"
      ]
      capacity_type       = "SPOT"
      local_storage_types = ["ssd"]
      block_device_mappings = {
        xvda = {
          device_name = "/dev/xvda"
          ebs = {
            volume_size           = 100
            volume_type           = "gp3"
            iops                  = 3000
            throughput            = 150
            encrypted             = true
            delete_on_termination = true
          }
        }
      }
    }

    gpu = {
      name_prefix  = "gpu"
      description  = "GPU node launch template"
      min_size     = 0
      max_size     = 32
      desired_size = 0

      ami_id         = data.aws_ami.deep_learning.image_id
      capacity_type  = "SPOT"
      instance_types = sort(data.aws_ec2_instance_types.gpu.instance_types)

      update_config = {
        max_unavailable_percentage = 100
      }

      local_storage_types = ["ssd"]
      block_device_mappings = {
        xvda = {
          device_name = "/dev/xvda"
          ebs = {
            volume_size           = 100
            volume_type           = "gp3"
            iops                  = 3000
            throughput            = 150
            encrypted             = true
            delete_on_termination = true
          }
        }
      }

      metadata_options = {
        http_endpoint          = "enabled"
        http_tokens            = "required"
        instance_metadata_tags = "disabled"
      }

      create_iam_role          = true
      iam_role_name            = "eks-managed-gpu-node-group"
      iam_role_use_name_prefix = false
      iam_role_description     = "EKS managed GPU node group"
      iam_role_tags = {
        Purpose = "Protector of the kubelet"
      }
      iam_role_additional_policies = {
        AmazonEC2ContainerRegistryReadOnly = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
      }
    }
  }
  tags = var.tags
}

# ASG tags are needed for the cluster to work with the labels and taints of the
# node groups
resource "aws_autoscaling_group_tag" "cluster_autoscaler_label_tags" {
  for_each               = var.existing_eks_cluster == null ? local.cluster_autoscaler_asg_tags : {}
  autoscaling_group_name = each.value.autoscaling_group

  tag {
    key                 = each.value.key
    value               = each.value.value
    propagate_at_launch = false
  }
}
