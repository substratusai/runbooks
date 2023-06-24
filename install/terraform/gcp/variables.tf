# Avoid setting defualts in variables to avoid multiple levels
# of defaults. Defaults are set in `terraform.tfvars`.

variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "zone" {
  type = string
}
