data "aws_availability_zones" "available" {}

locals {
  azs        = slice(data.aws_availability_zones.available.names, 0, 3)
  create_vpc = var.existing_vpc == null ? 1 : 0
}

module "vpc" {
  count           = local.create_vpc
  source          = "terraform-aws-modules/vpc/aws"
  version         = "5.1.1"
  name            = var.name_prefix
  cidr            = var.vpc_cidr
  azs             = local.azs
  private_subnets = [for k, v in local.azs : cidrsubnet(var.vpc_cidr, 6, k)]
  public_subnets  = [for k, v in local.azs : cidrsubnet(var.vpc_cidr, 6, k + 4)]
  intra_subnets   = [for k, v in local.azs : cidrsubnet(var.vpc_cidr, 6, k + 20)]

  public_subnet_ipv6_prefixes                    = [0, 1, 2]
  public_subnet_assign_ipv6_address_on_creation  = true
  private_subnet_ipv6_prefixes                   = [3, 4, 5]
  private_subnet_assign_ipv6_address_on_creation = true
  intra_subnet_ipv6_prefixes                     = [6, 7, 8]
  intra_subnet_assign_ipv6_address_on_creation   = true

  public_subnet_tags = {
    "kubernetes.io/role/elb" = 1
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = 1
  }

  create_database_subnet_group  = false
  manage_default_network_acl    = false
  manage_default_route_table    = false
  manage_default_security_group = false

  enable_dns_hostnames   = true
  enable_dns_support     = true
  enable_nat_gateway     = true
  single_nat_gateway     = true
  enable_ipv6            = true
  create_egress_only_igw = true
  enable_vpn_gateway     = false
  enable_dhcp_options    = false

  # VPC Flow Logs (Cloudwatch log group and IAM role will be created)
  enable_flow_log                      = false
  create_flow_log_cloudwatch_log_group = true
  create_flow_log_cloudwatch_iam_role  = true
  flow_log_max_aggregation_interval    = 60
  tags                                 = var.tags
}


# VPC Endpoints Module

module "endpoints" {
  count                      = local.create_vpc
  source                     = "terraform-aws-modules/vpc/aws//modules/vpc-endpoints"
  version                    = "5.1.1"
  vpc_id                     = module.vpc[0].vpc_id
  create_security_group      = true
  security_group_name_prefix = "${var.name_prefix}-endpoints-"
  security_group_description = "VPC endpoint security group"
  security_group_rules = {
    ingress_https = {
      description = "HTTPS from VPC"
      cidr_blocks = [module.vpc[0].vpc_cidr_block]
    }
  }

  endpoints = {
    s3 = {
      service = "s3"
      tags    = { Name = "s3-vpc-endpoint" }
    },
    ecr_api = {
      service             = "ecr.api"
      private_dns_enabled = true
      subnet_ids          = module.vpc[0].private_subnets
      policy              = data.aws_iam_policy_document.generic_endpoint_policy[0].json
    },
    ecr_dkr = {
      service             = "ecr.dkr"
      private_dns_enabled = true
      subnet_ids          = module.vpc[0].private_subnets
      policy              = data.aws_iam_policy_document.generic_endpoint_policy[0].json
    },
  }

  tags = merge(var.tags, {
    Endpoint = "true"
  })
}

data "aws_iam_policy_document" "generic_endpoint_policy" {
  count = local.create_vpc
  statement {
    effect    = "Deny"
    actions   = ["*"]
    resources = ["*"]

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    condition {
      test     = "StringNotEquals"
      variable = "aws:SourceVpc"
      values   = [module.vpc[0].vpc_id]
    }
  }
}
