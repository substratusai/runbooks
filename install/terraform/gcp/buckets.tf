resource "google_storage_bucket" "artifacts" {
  project       = var.project_id
  name          = "${var.project_id}-${local.name}-artifacts"
  location      = var.region
  force_destroy = true

  uniform_bucket_level_access = true
}
