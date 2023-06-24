resource "google_storage_bucket" "training" {
  project       = var.project_id
  name          = "${var.project_id}-${local.name}-training"
  location      = var.region
  force_destroy = true

  uniform_bucket_level_access = true
}
