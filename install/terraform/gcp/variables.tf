variable "project_id" {
  type = string
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "zone" {
  type    = string
  default = "us-central1-a"
}

variable "attach_gpu_nodepools" {
  type        = bool
  default     = true
  description = "Whether to attach GPU nodepools to the cluster. These node pools fill in missing support for Node Autoprosioning of some GPU types."
}
