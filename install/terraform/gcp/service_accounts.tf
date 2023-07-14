resource "google_service_account" "container_builder" {
  project    = var.project_id
  account_id = "${local.name}-container-builder"
}

resource "google_service_account" "modeller" {
  project    = var.project_id
  account_id = "${local.name}-modeller"
}

resource "google_service_account" "model_server" {
  project    = var.project_id
  account_id = "${local.name}-model-server"
}

resource "google_service_account" "notebook" {
  project    = var.project_id
  account_id = "${local.name}-notebook"
}

resource "google_service_account" "data_loader" {
  project    = var.project_id
  account_id = "${local.name}-data-loader"
}

resource "google_service_account" "gcp_manager" {
  project    = var.project_id
  account_id = "${local.name}-gcp-manager"
}
