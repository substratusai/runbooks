# allow the service account to create signed URLs via the iam.serviceAccounts.signBlob permission

resource "google_service_account_iam_member" "admin-account-iam" {
  service_account_id = google_service_account.gcp_manager.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.gcp_manager.email}"
}
