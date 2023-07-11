resource "google_service_account" "container_builder" {
  project    = var.project_id
  account_id = "${local.name}-container-builder"
}

resource "google_service_account" "model_trainer" {
  project    = var.project_id
  account_id = "${local.name}-model-trainer"
}

resource "google_service_account" "model_loader" {
  project    = var.project_id
  account_id = "${local.name}-model-loader"
}

resource "google_service_account" "data_loader" {
  project    = var.project_id
  account_id = "${local.name}-data-loader"
}
