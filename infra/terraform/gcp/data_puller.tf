resource "google_service_account" "data_puller" {
  project    = var.project_id
  account_id = "${var.name}-data-puller"
}

resource "google_service_account_iam_member" "data_puller_workload_identity" {
  service_account_id = google_service_account.data_puller.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/data-puller]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_storage_bucket_iam_member" "data_puller_datasets_storage_admin" {
  bucket = google_storage_bucket.datasets.name
  role = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.data_puller.email}"
}

