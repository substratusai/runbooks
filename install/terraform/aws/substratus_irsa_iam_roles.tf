resource "aws_iam_policy" "ecr_writer" {
  count       = var.create_substratus_irsa_roles == true ? 1 : 0
  name        = "${var.name_prefix}-ecr-writer"
  description = "A policy allowing full access to the ${local.artifacts_bucket.id} bucket"

  policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Effect" : "Allow",
        "Action" : [
          "ecr:*"
        ],
        "Resource" : local.ecr_repository_arn
      }
    ]
  })

  tags = var.tags
}

resource "aws_iam_policy" "s3_full_bucket_access" {
  count       = var.create_substratus_irsa_roles == true ? 1 : 0
  name        = "${var.name_prefix}-AmazonS3FullAccess"
  description = "A policy allowing full access to the ${local.artifacts_bucket.id} bucket"

  policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Effect" : "Allow",
        "Action" : [
          "s3:*",
          "s3-object-lambda:*"
        ],
        "Resource" : [
          "${local.artifacts_bucket.arn}",
          "${local.artifacts_bucket.arn}/*",
        ]
      }
    ]
  })

  tags = var.tags
}

resource "aws_iam_policy" "s3_readonly_bucket_access" {
  count       = var.create_substratus_irsa_roles == true ? 1 : 0
  name        = "${var.name_prefix}-AmazonS3ReadOnlyAccess"
  description = "A policy allowing read-only access to the ${local.artifacts_bucket.id} bucket"

  policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Effect" : "Allow",
        "Action" : [
          "s3:Get*",
          "s3:List*",
          "s3-object-lambda:Get*",
          "s3-object-lambda:List*"
        ],
        "Resource" : [
          "${local.artifacts_bucket.arn}",
          "${local.artifacts_bucket.arn}/*",
        ]
      }
    ]
  })

  tags = var.tags
}

module "container_builder_irsa" {
  count   = var.create_substratus_irsa_roles == true ? 1 : 0
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.28"

  role_name_prefix = "${var.name_prefix}-container-builder-"
  role_policy_arns = {
    ECRWriter                        = aws_iam_policy.ecr_writer[0].arn
    SubstratusAmazonS3ReadOnlyAccess = aws_iam_policy.s3_readonly_bucket_access[0].arn
  }

  oidc_providers = {
    main = {
      provider_arn               = local.eks_cluster.oidc_provider_arn
      namespace_service_accounts = ["default:container-builder"]
    }
  }

  tags = var.tags
}

module "modeller_irsa" {
  count   = var.create_substratus_irsa_roles == true ? 1 : 0
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.28"

  role_name_prefix = "${var.name_prefix}-modeller-"
  role_policy_arns = {
    SubstratusAmazonS3FullAccess = aws_iam_policy.s3_full_bucket_access[0].arn
  }

  oidc_providers = {
    main = {
      provider_arn               = local.eks_cluster.oidc_provider_arn
      namespace_service_accounts = ["default:modeller"]
    }
  }

  tags = var.tags
}

module "model_server_irsa" {
  count   = var.create_substratus_irsa_roles == true ? 1 : 0
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.28"

  role_name_prefix = "${var.name_prefix}-model-server-"
  role_policy_arns = {
    SubstratusAmazonS3FullAccess = aws_iam_policy.s3_full_bucket_access[0].arn
  }

  oidc_providers = {
    main = {
      provider_arn               = local.eks_cluster.oidc_provider_arn
      namespace_service_accounts = ["default:model-server"]
    }
  }

  tags = var.tags
}

module "notebook_irsa" {
  count   = var.create_substratus_irsa_roles == true ? 1 : 0
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.28"

  role_name_prefix = "${var.name_prefix}-notebook-"
  role_policy_arns = {
    SubstratusAmazonS3FullAccess = aws_iam_policy.s3_full_bucket_access[0].arn
  }

  oidc_providers = {
    main = {
      provider_arn               = local.eks_cluster.oidc_provider_arn
      namespace_service_accounts = ["default:notebook"]
    }
  }

  tags = var.tags
}

module "data_loader_irsa" {
  count   = var.create_substratus_irsa_roles == true ? 1 : 0
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.28"

  role_name_prefix = "${var.name_prefix}-data-loader-"
  role_policy_arns = {
    SubstratusAmazonS3FullAccess = aws_iam_policy.s3_full_bucket_access[0].arn
  }

  oidc_providers = {
    main = {
      provider_arn               = local.eks_cluster.oidc_provider_arn
      namespace_service_accounts = ["default:data-loader"]
    }
  }

  tags = var.tags
}

module "aws_manager_irsa" {
  count   = var.create_substratus_irsa_roles == true ? 1 : 0
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.28"

  role_name_prefix = "${var.name_prefix}-aws-manager-"
  role_policy_arns = {
    SubstratusAmazonS3FullAccess = aws_iam_policy.s3_full_bucket_access[0].arn
  }

  oidc_providers = {
    main = {
      provider_arn               = local.eks_cluster.oidc_provider_arn
      namespace_service_accounts = ["substratus:aws-manager"]
    }
  }

  tags = var.tags
}
