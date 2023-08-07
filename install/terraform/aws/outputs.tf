output "cluster" {
  value = local.eks_cluster
}

output "vpc" {
  value = local.vpc
}

output "irsas" {
  value = local.irsa_outputs
}
