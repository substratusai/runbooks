resource "google_service_account" "model_builder" {
  project    = var.project_id
  account_id = "${var.name}-model-builder"
}

resource "google_service_account_iam_member" "model_builder_workload_identity" {
  service_account_id = google_service_account.model_builder.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/model-builder]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_project_iam_member" "model_builder_gar_repo_admin" {
  project = var.project_id
  role    = "roles/artifactregistry.repoAdmin"
  member  = "serviceAccount:${google_service_account.model_builder.email}"
}

resource "google_storage_bucket_iam_member" "model_builder_training_storage_admin" {
  bucket = google_storage_bucket.training.name
  role = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.model_builder.email}"
}

