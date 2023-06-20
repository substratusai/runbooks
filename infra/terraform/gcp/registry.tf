resource "google_artifact_registry_repository" "main" {
  project       = var.project_id
  location      = var.region
  repository_id = var.name
  description   = "Substratus Docker Registry"
  format        = "DOCKER"
}
