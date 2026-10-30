output "url" {
  description = "Public URL of the web app and sign-in (ARCHIVE_URL)."
  value       = local.url
}

output "api_url" {
  description = "Public URL of the API, MCP, webhooks and OAuth redirects (ARCHIVE_API_URL)."
  value       = local.api_url
}

output "dns_records" {
  description = "DNS records to create for each mapped domain."
  value       = { for d, m in google_cloud_run_domain_mapping.this : d => m.status[0].resource_records }
}

output "bucket" {
  description = "GCS bucket holding the files."
  value       = google_storage_bucket.this.name
}

output "database_connection_name" {
  description = "Cloud SQL connection name."
  value       = google_sql_database_instance.this.connection_name
}
