resource "google_project_iam_member" "" {
  project = var.project_id
  role    = "roles/artifactregistry.repoAdmin"
  member  = "serviceAccount:${google_service_account.container_builder.email}"
}

