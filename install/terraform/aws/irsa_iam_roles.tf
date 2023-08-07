data "aws_iam_policy" "eks_cni_policy" {
  name = "AmazonEKS_CNI_Policy"
}

data "aws_iam_policy" "iam_full_access" {
  name = "IAMFullAccess"
}

data "aws_iam_policy" "container_registry_full_access" {
  name = "AmazonEC2ContainerRegistryFullAccess"
}

data "aws_iam_policy" "s3_full_access" {
  name = "AmazonS3FullAccess"
}

module "substratus_irsa" {
  count            = local.create_cluster
  source           = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version          = "~> 5.28"
  role_name_prefix = "${var.name_prefix}-substratus-"
  role_policy_arns = {
    IAMFullAccess                        = data.aws_iam_policy.iam_full_access.arn
    AmazonEC2ContainerRegistryFullAccess = data.aws_iam_policy.container_registry_full_access.arn
    AmazonS3FullAccess                   = data.aws_iam_policy.s3_full_access.arn
  }

  oidc_providers = {
    main = {
      provider_arn               = local.eks_cluster.oidc_provider_arn
      namespace_service_accounts = ["substratus:substratus"]
    }
  }

  tags = var.tags
}

module "ebs_csi_irsa_role" {
  count   = local.create_cluster
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.28"

  role_name_prefix      = "ebs-csi"
  attach_ebs_csi_policy = true

  oidc_providers = {
    main = {
      provider_arn               = local.eks_cluster.oidc_provider_arn
      namespace_service_accounts = ["kube-system:ebs-csi-controller-sa"]
    }
  }

  tags = var.tags
}

module "load_balancer_controller_irsa_role" {
  count   = local.create_cluster
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.28"

  role_name_prefix                       = "load-balancer-controller"
  attach_load_balancer_controller_policy = true

  oidc_providers = {
    main = {
      provider_arn               = local.eks_cluster.oidc_provider_arn
      namespace_service_accounts = ["kube-system:aws-load-balancer-controller"]
    }
  }

  tags = var.tags
}

module "node_termination_handler_irsa_role" {
  count   = local.create_cluster
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.28"

  role_name_prefix                       = "node-termination-handler"
  attach_node_termination_handler_policy = true

  oidc_providers = {
    main = {
      provider_arn               = local.eks_cluster.oidc_provider_arn
      namespace_service_accounts = ["kube-system:aws-node"]
    }
  }

  tags = var.tags
}

module "vpc_cni_ipv4_irsa_role" {
  count   = local.create_cluster
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.28"

  role_name_prefix      = "vpc-cni-ipv4"
  attach_vpc_cni_policy = true
  vpc_cni_enable_ipv4   = true

  oidc_providers = {
    main = {
      provider_arn               = local.eks_cluster.oidc_provider_arn
      namespace_service_accounts = ["kube-system:aws-node"]
    }
  }

  tags = var.tags
}
