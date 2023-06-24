resource "google_service_account" "model_trainer" {
  project    = var.project_id
  account_id = "${var.name}-model-trainer"
}

resource "google_service_account_iam_member" "model_trainer_workload_identity" {
  service_account_id = google_service_account.model_trainer.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/model-trainer]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_storage_bucket_iam_member" "model_trainer_datasets_storage_admin" {
  bucket = google_storage_bucket.datasets.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.model_trainer.email}"
}

resource "google_storage_bucket_iam_member" "model_trainer_training_storage_admin" {
  bucket = google_storage_bucket.training.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.model_trainer.email}"
}

