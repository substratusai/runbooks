output "artifacts_bucket" {
  value = {
    arn = local.artifacts_bucket.arn
    id  = local.artifacts_bucket.id
  }
}

output "cluster_name" {
  value = local.eks_cluster.name
}

output "cluster_region" {
  value = var.region
}

output "cluster" {
  value = {
    name              = local.eks_cluster.name
    oidc_provider_arn = local.eks_cluster.oidc_provider_arn
  }
}

output "ecr_repository_arn" {
  value = local.ecr_repository_arn
}

output "vpc" {
  value = {
    id                 = local.vpc.id
    private_subnet_ids = local.vpc.private_subnet_ids
    intra_subnet_ids   = local.vpc.intra_subnet_ids
  }
}
