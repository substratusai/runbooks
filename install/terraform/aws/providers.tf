provider "aws" {
  region = var.region
}

provider "kubernetes" {
  host                   = local.eks_cluster.endpoint
  cluster_ca_certificate = base64decode(local.eks_cluster.certificate_authority_data)

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    # This requires the awscli to be installed locally where Terraform is executed
    args = ["eks", "get-token", "--cluster-name", local.eks_cluster.name]
  }
}
