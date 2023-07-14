resource "google_storage_bucket" "models" {
  project       = var.project_id
  name          = "${var.project_id}-${local.name}-models"
  location      = var.region
  force_destroy = true

  uniform_bucket_level_access = true
}

resource "google_storage_bucket" "datasets" {
  project       = var.project_id
  name          = "${var.project_id}-${local.name}-datasets"
  location      = var.region
  force_destroy = true

  uniform_bucket_level_access = true
}

resource "google_storage_bucket" "notebooks" {
  project       = var.project_id
  name          = "${var.project_id}-${local.name}-notebooks"
  location      = var.region
  force_destroy = true

  uniform_bucket_level_access = true
}
