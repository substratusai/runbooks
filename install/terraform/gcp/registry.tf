resource "google_artifact_registry_repository" "main" {
  project       = var.project_id
  location      = var.region
  repository_id = local.name
  description   = "Docker Registry for ${local.name}"
  format        = "DOCKER"
}
