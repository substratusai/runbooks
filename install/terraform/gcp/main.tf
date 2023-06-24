terraform {
  backend "gcs" {
    #bucket  = ""
    prefix = "primary" # Allow for multiple instances of substratus with the same state bucket later.
  }
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "4.69.1"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "4.69.1"
    }
  }
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

locals {
  # Don't expose the name as an environment variable until the substratus controllers
  # support configurable names for buckets, service accounts, registries, etc.
  name = "substratus"
}
