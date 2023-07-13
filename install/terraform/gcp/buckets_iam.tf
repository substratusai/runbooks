# Modeller #

resource "google_storage_bucket_iam_member" "modeller_datasets_storage_admin" {
  bucket = google_storage_bucket.datasets.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.modeller.email}"
}

resource "google_storage_bucket_iam_member" "modeller_models_storage_admin" {
  bucket = google_storage_bucket.models.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.modeller.email}"
}

# Model Server #

resource "google_storage_bucket_iam_member" "model_server_models_storage_admin" {
  bucket = google_storage_bucket.models.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.model_server.email}"
}

# Notebook #

resource "google_storage_bucket_iam_member" "notebook_datasets_storage_admin" {
  bucket = google_storage_bucket.datasets.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.notebook.email}"
}

resource "google_storage_bucket_iam_member" "notebook_models_storage_admin" {
  bucket = google_storage_bucket.models.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.notebook.email}"
}

# Data Loader #

resource "google_storage_bucket_iam_member" "data_loader_datasets_storage_admin" {
  bucket = google_storage_bucket.datasets.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.data_loader.email}"
}

