terraform {
  backend "gcs" {
    #bucket  = ""
    prefix = "tfstate"
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
  enabled_service_apis = [
    "artifactregistry.googleapis.com",
    # "container.googleapis.com",
  ]
}

resource "google_project_service" "main" {
  for_each                   = toset(local.enabled_service_apis)
  project                    = var.project_id
  service                    = each.key
  disable_dependent_services = false
  disable_on_destroy         = false
}
