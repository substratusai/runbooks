# Container Builder #

resource "google_project_iam_member" "container_builder_gar_repo_admin" {
  project = var.project_id
  role    = "roles/artifactregistry.repoAdmin"
  member  = "serviceAccount:${google_service_account.container_builder.email}"
}

# Model Trainer #

resource "google_storage_bucket_iam_member" "model_trainer_datasets_storage_admin" {
  bucket = google_storage_bucket.datasets.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.model_trainer.email}"
}

resource "google_storage_bucket_iam_member" "model_trainer_models_storage_admin" {
  bucket = google_storage_bucket.models.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.model_trainer.email}"
}

# Model Loader #

resource "google_storage_bucket_iam_member" "model_loader_models_storage_admin" {
  bucket = google_storage_bucket.models.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.model_loader.email}"
}

# Data Loader #

resource "google_storage_bucket_iam_member" "data_loader_datasets_storage_admin" {
  bucket = google_storage_bucket.datasets.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.data_loader.email}"
}

