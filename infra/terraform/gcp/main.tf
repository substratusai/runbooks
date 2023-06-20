terraform {
  backend "gcs" {
    #bucket  = ""
    prefix = "tfstate"
  }
  required_providers {
    google = {
      source = "hashicorp/google"
      version = "4.69.1"
    }
    google-beta = {
      source = "hashicorp/google-beta"
      version = "4.69.1"
    }
  }
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}
