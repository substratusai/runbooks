variable "cluster_version" {
  description = "The version of the EKS cluster to deploy (i.e., this is used when var.existing_eks_cluster is null)"
  type        = string
  default     = "1.27"
}

variable "existing_eks_cluster" {
  description = "An existing EKS cluster to add substratus components to."
  type = object({
    name              = string
    oidc_provider_arn = string
  })
  default = null
}

variable "existing_vpc" {
  description = "An existing VPC to add substratus components to."
  type = object({
    id                 = string
    private_subnet_ids = list(string)
    intra_subnet_ids   = list(string)
  })
  default = null
}

variable "labels" {
  type = map(string)
  default = {
    GithubRepo = "substratus"
    GithubOrg  = "substratusai"
  }
}

variable "name_prefix" {
  description = "Prefix to use for resources"
  type        = string
  default     = "substratus-usw2"
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-west-2"
}

# will remove this before pushing to substratus repo
variable "tags" {
  type = map(string)
  default = {
    GithubRepo = "infrastructure"
    GithubOrg  = "substratusai"
  }
}

variable "vpc_cidr" {
  description = "The cidr block of the VPC if created by the module (e.g., used when var.existing_vpc is null)"
  type        = string
  default     = "10.0.0.0/16"
}
