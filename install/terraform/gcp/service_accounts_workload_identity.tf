resource "google_service_account_iam_member" "container_builder_workload_identity" {
  service_account_id = google_service_account.container_builder.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/container-builder]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_service_account_iam_member" "model_trainer_workload_identity" {
  service_account_id = google_service_account.model_trainer.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/model-trainer]"

  # Workload identity pool does not exist until the first cluster exists.
  depends_on = [google_container_cluster.main]
}

resource "google_service_account_iam_member" "model_loader_workload_identity" {
  service_account_id = google_service_account.model_loader.id
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/model-loader]"

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

