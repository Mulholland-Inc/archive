output "url" {
  description = "Public URL of the web app and sign-in (ARCHIVE_URL)."
  value       = local.url
}

output "api_url" {
  description = "Public URL of the API, MCP, webhooks and OAuth redirects (ARCHIVE_API_URL)."
  value       = local.api_url
}

output "deploy" {
  description = "GitHub Actions variables for deploying new images."
  value = var.github_repository == "" ? null : {
    GCP_PROJECT                    = var.project
    GCP_WORKLOAD_IDENTITY_PROVIDER = google_iam_workload_identity_pool_provider.github[0].name
    GCP_SERVICE_ACCOUNT            = google_service_account.deploy[0].email
    GCP_REGION                     = var.region
    GCP_SERVICE                    = google_cloud_run_v2_service.this.name
  }
}

output "bucket" {
  description = "GCS bucket holding the files."
  value       = google_storage_bucket.this.name
}

output "database_connection_name" {
  description = "Cloud SQL connection name."
  value       = google_sql_database_instance.this.connection_name
}
