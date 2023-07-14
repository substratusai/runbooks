resource "google_service_account_iam_member" "container_builder_workload_identity" {
  service_account_id = google_service_account.container_builder.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/container-builder]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_service_account_iam_member" "modeller_workload_identity" {
  service_account_id = google_service_account.modeller.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/modeller]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_service_account_iam_member" "model_server_workload_identity" {
  service_account_id = google_service_account.model_server.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/model-server]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_service_account_iam_member" "notebook_workload_identity" {
  service_account_id = google_service_account.notebook.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/notebook]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_service_account_iam_member" "data_loader_workload_identity" {
  service_account_id = google_service_account.data_loader.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/data-loader]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_service_account_iam_member" "gcp_manager_workload_identity" {
  service_account_id = google_service_account.gcp_manager.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[substratus/gcp-manager]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}
