locals {
  # passed to cluster.tf
  vpc = {
    id                 = var.existing_vpc == null ? module.vpc[0].vpc_id : var.existing_vpc.id
    private_subnet_ids = var.existing_vpc == null ? module.vpc[0].private_subnets : var.existing_vpc.private_subnet_ids
    intra_subnet_ids   = var.existing_vpc == null ? module.vpc[0].intra_subnets : var.existing_vpc.intra_subnet_ids
    endpoints          = var.existing_vpc == null ? module.endpoints[0] : null
  }

  # passed to substratus_irsa_iam_roles.tf and eks_irsa_iam_roles.tf
  eks_cluster = {
    name                       = local.create_cluster == 1 ? module.eks[0].cluster_name : var.existing_eks_cluster.name
    oidc_provider_arn          = local.create_cluster == 1 ? module.eks[0].oidc_provider_arn : var.existing_eks_cluster.oidc_provider_arn
    managed_node_groups        = local.create_cluster == 1 ? module.eks[0].eks_managed_node_groups : null
    certificate_authority_data = local.create_cluster == 1 ? module.eks[0].cluster_certificate_authority_data : ""
    endpoint                   = local.create_cluster == 1 ? module.eks[0].cluster_endpoint : ""
    region                     = var.region
  }

  irsa_outputs = {
    ebs_csi_irsa_role                  = local.create_cluster == 1 ? module.ebs_csi_irsa_role[0] : {}
    load_balancer_controller_irsa_role = local.create_cluster == 1 ? module.load_balancer_controller_irsa_role[0] : {}
    node_termination_handler_irsa_role = local.create_cluster == 1 ? module.node_termination_handler_irsa_role[0] : {}
    substratus_irsa                    = local.create_cluster == 1 ? module.substratus_irsa[0] : {}
    vpc_cni_ipv4_irsa_role             = local.create_cluster == 1 ? module.vpc_cni_ipv4_irsa_role[0] : {}
  }
}
