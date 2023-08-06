locals {
  # passed to cluster.tf
  vpc = {
    id                 = var.existing_vpc == null ? module.vpc[0].vpc_id : var.existing_vpc.id
    private_subnet_ids = var.existing_vpc == null ? module.vpc[0].private_subnets : var.existing_vpc.private_subnet_ids
    intra_subnet_ids   = var.existing_vpc == null ? module.vpc[0].intra_subnets : var.existing_vpc.intra_subnet_ids
  }

  # passed to substratus_irsa_iam_roles.tf and eks_irsa_iam_roles.tf
  eks_cluster = {
    name                       = var.existing_eks_cluster == null ? module.eks[0].cluster_name : var.existing_eks_cluster.name
    oidc_provider_arn          = var.existing_eks_cluster == null ? module.eks[0].oidc_provider_arn : var.existing_eks_cluster.oidc_provider_arn
    managed_node_groups        = var.existing_eks_cluster == null ? module.eks[0].eks_managed_node_groups : null
    certificate_authority_data = var.existing_eks_cluster == null ? module.eks[0].cluster_certificate_authority_data : ""
    endpoint                   = var.existing_eks_cluster == null ? module.eks[0].cluster_endpoint : ""
  }

  artifacts_bucket = {
    arn = var.existing_artifacts_bucket == null ? aws_s3_bucket.artifacts[0].arn : var.existing_artifacts_bucket.arn
    id  = var.existing_artifacts_bucket == null ? aws_s3_bucket.artifacts[0].id : var.existing_artifacts_bucket.id
  }

  ecr_repository_arn = var.existing_ecr_repository_arn == "" ? aws_ecr_repository.main[0].arn : var.existing_ecr_repository_arn
}
