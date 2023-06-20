resource "google_service_account" "image_builder" {
  project    = var.project_id
  account_id = "${var.name}-image-builder"
}

resource "google_service_account_iam_member" "image_builder_workload_identity" {
  service_account_id = google_service_account.image_builder.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/image-builder]"
}

resource "google_project_iam_member" "image_builder_gar_repo_admin" {
  project = var.project_id
  role    = "roles/artifactregistry.repoAdmin"
  member  = "serviceAccount:${google_service_account.image_builder.email}"
}
