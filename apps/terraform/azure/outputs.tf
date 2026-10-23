output "url" {
  description = "Public URL of Archive."
  value       = local.url
}

output "database_fqdn" {
  description = "PostgreSQL Flexible Server host name."
  value       = azurerm_postgresql_flexible_server.this.fqdn
}

output "storage_account" {
  description = "Storage account holding the NFS share mounted at /data."
  value       = azurerm_storage_account.this.name
}
