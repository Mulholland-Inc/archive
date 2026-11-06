output "url" {
  description = "Public URL of the web app and sign-in (ARCHIVE_URL)."
  value       = local.url
}

output "api_url" {
  description = "Public URL of the API, webhooks and OAuth redirects; MCP is at /mcp too."
  value       = local.api_url
}

output "mcp_url" {
  description = "Public URL of the MCP server, for MCP clients like Claude and ChatGPT, which sign in with OAuth."
  value       = local.mcp_url
}

output "bucket" {
  description = "GCS bucket holding the files."
  value       = google_storage_bucket.this.name
}

output "service" {
  description = "Name of the Cloud Run service running the API."
  value       = google_cloud_run_v2_service.this.name
}

output "service_account" {
  description = "Email of the service account the API runs as."
  value       = google_service_account.this.email
}

output "tasks_queue" {
  description = "Cloud Tasks queue that delivers due workflows and files to the API, named after the service, which finds it from the metadata server."
  value       = google_cloud_tasks_queue.this.id
}

output "database_connection_name" {
  description = "Cloud SQL connection name."
  value       = google_sql_database_instance.this.connection_name
}

output "github_variables" {
  description = "The Actions variables the image and terraform workflows of github_repository read; set them once."
  value = var.github_repository == "" ? {} : {
    GCP_PROJECT                    = var.project
    GCP_REGION                     = var.region
    GCP_SERVICE_ACCOUNT            = google_service_account.deploy[0].email
    GCP_TERRAFORM_SERVICE_ACCOUNT  = google_service_account.terraform[0].email
    GCP_WORKLOAD_IDENTITY_PROVIDER = google_iam_workload_identity_pool_provider.github[0].name
    CLOUDFLARE_ACCOUNT_ID          = data.cloudflare_zone.this.account.id
    ARCHIVE_DOMAIN                 = var.domain
  }
}
