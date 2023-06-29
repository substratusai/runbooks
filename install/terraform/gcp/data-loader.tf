resource "google_service_account" "data_loader" {
  project    = var.project_id
  account_id = "${local.name}-data-loader"
}

resource "google_service_account_iam_member" "data_loader_workload_identity" {
  service_account_id = google_service_account.data_loader.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/data-loader]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_storage_bucket_iam_member" "data_loader_datasets_storage_admin" {
  bucket = google_storage_bucket.datasets.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.data_loader.email}"
}

