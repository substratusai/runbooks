resource "google_storage_bucket" "datasets" {
  project = var.project_id
  name          = "${var.project_id}-substratus-datasets"
  location      = var.region
  force_destroy = true

  uniform_bucket_level_access = true
}
