resource "google_service_account" "data_puller" {
  project    = var.project_id
  account_id = "${var.name}-data-puller"
}

