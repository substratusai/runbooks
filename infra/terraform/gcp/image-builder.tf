resource "google_service_account" "image_builder" {
  project    = var.project_id
  account_id = "${var.name}-image-builder"
}

resource "google_service_account_iam_member" "image_builder_workload_identity" {
  service_account_id = google_service_account.image_builder.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/image-builder]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_project_iam_member" "image_builder_gar_repo_admin" {
  project = var.project_id
  role    = "roles/artifactregistry.repoAdmin"
  member  = "serviceAccount:${google_service_account.image_builder.email}"
}

resource "google_storage_bucket_iam_member" "image_builder_datasets_storage_admin" {
  bucket = google_storage_bucket.datasets.name
  role = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.image_builder.email}"
}

