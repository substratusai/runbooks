# roles/iam.serviceAccountTokenCreator

resource "google_service_account_iam_member" "admin-account-iam" {
  service_account_id = google_service_account.gcp_manager.name
  role               = "roles/iam.serviceAccountUser"
  member             = "user:jane@example.com"
}
