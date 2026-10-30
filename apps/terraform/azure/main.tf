locals {
  url = "https://${var.name}.${azurerm_container_app_environment.this.default_domain}"
  client_ids = { for k, v in {
    ARCHIVE_GOOGLE_CLIENT_ID    = var.google_client_id
    ARCHIVE_MICROSOFT_CLIENT_ID = var.microsoft_client_id
    ARCHIVE_SLACK_CLIENT_ID     = var.slack_client_id
    ARCHIVE_GITHUB_CLIENT_ID    = var.github_client_id
  } : k => v if v != "" }
  client_secrets = { for k, v in {
    ARCHIVE_GOOGLE_CLIENT_SECRET    = var.google_client_secret
    ARCHIVE_MICROSOFT_CLIENT_SECRET = var.microsoft_client_secret
    ARCHIVE_SLACK_CLIENT_SECRET     = var.slack_client_secret
    ARCHIVE_GITHUB_CLIENT_SECRET    = var.github_client_secret
  } : k => v if nonsensitive(v != "") }
}

resource "random_string" "suffix" {
  length  = 8
  special = false
  upper   = false
}

resource "random_password" "db" {
  length  = 32
  special = false
}

resource "random_id" "secret_key" {
  byte_length = 32
}

resource "azurerm_resource_group" "this" {
  name     = var.name
  location = var.location
}

resource "azurerm_log_analytics_workspace" "this" {
  name                = var.name
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  retention_in_days   = 30
}

resource "azurerm_virtual_network" "this" {
  name                = var.name
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  address_space       = ["10.0.0.0/16"]
}

resource "azurerm_subnet" "apps" {
  name                 = "apps"
  resource_group_name  = azurerm_resource_group.this.name
  virtual_network_name = azurerm_virtual_network.this.name
  address_prefixes     = ["10.0.0.0/23"]
  service_endpoints    = ["Microsoft.Storage"]

  delegation {
    name = "apps"
    service_delegation {
      name    = "Microsoft.App/environments"
      actions = ["Microsoft.Network/virtualNetworks/subnets/join/action"]
    }
  }
}

resource "azurerm_storage_account" "this" {
  name                       = "${var.name}${random_string.suffix.result}"
  location                   = azurerm_resource_group.this.location
  resource_group_name        = azurerm_resource_group.this.name
  account_kind               = "FileStorage"
  account_tier               = "Premium"
  account_replication_type   = "LRS"
  https_traffic_only_enabled = false

  network_rules {
    default_action             = "Deny"
    virtual_network_subnet_ids = [azurerm_subnet.apps.id]
  }
}

resource "azurerm_storage_share" "this" {
  name               = "data"
  storage_account_id = azurerm_storage_account.this.id
  quota              = var.share_quota
  enabled_protocol   = "NFS"
}

resource "azurerm_postgresql_flexible_server" "this" {
  name                          = "${var.name}-${random_string.suffix.result}"
  location                      = azurerm_resource_group.this.location
  resource_group_name           = azurerm_resource_group.this.name
  version                       = "18"
  sku_name                      = var.db_sku
  storage_mb                    = var.db_storage_mb
  administrator_login           = "archive"
  administrator_password        = random_password.db.result
  public_network_access_enabled = true
  backup_retention_days         = 7

  lifecycle {
    ignore_changes = [zone]
  }
}

resource "azurerm_postgresql_flexible_server_configuration" "extensions" {
  name      = "azure.extensions"
  server_id = azurerm_postgresql_flexible_server.this.id
  value     = "VECTOR"
}

resource "azurerm_postgresql_flexible_server_firewall_rule" "azure" {
  name             = "azure"
  server_id        = azurerm_postgresql_flexible_server.this.id
  start_ip_address = "0.0.0.0"
  end_ip_address   = "0.0.0.0"
}

resource "azurerm_postgresql_flexible_server_database" "this" {
  name      = "archive"
  server_id = azurerm_postgresql_flexible_server.this.id
}

resource "azurerm_container_app_environment" "this" {
  name                       = var.name
  location                   = azurerm_resource_group.this.location
  resource_group_name        = azurerm_resource_group.this.name
  log_analytics_workspace_id = azurerm_log_analytics_workspace.this.id
  infrastructure_subnet_id   = azurerm_subnet.apps.id

  workload_profile {
    name                  = "Consumption"
    workload_profile_type = "Consumption"
  }
}

resource "azurerm_container_app_environment_storage" "this" {
  name                         = "data"
  container_app_environment_id = azurerm_container_app_environment.this.id
  nfs_server_url               = "${azurerm_storage_account.this.name}.file.core.windows.net"
  share_name                   = "/${azurerm_storage_account.this.name}/${azurerm_storage_share.this.name}"
  access_mode                  = "ReadWrite"
}

resource "azurerm_container_app" "this" {
  name                         = var.name
  container_app_environment_id = azurerm_container_app_environment.this.id
  resource_group_name          = azurerm_resource_group.this.name
  revision_mode                = "Single"
  workload_profile_name        = "Consumption"

  secret {
    name  = "database-url"
    value = "postgres://archive:${random_password.db.result}@${azurerm_postgresql_flexible_server.this.fqdn}:5432/${azurerm_postgresql_flexible_server_database.this.name}?sslmode=require"
  }

  secret {
    name  = "secret-key"
    value = random_id.secret_key.hex
  }

  dynamic "secret" {
    for_each = local.client_secrets
    content {
      name  = replace(lower(secret.key), "_", "-")
      value = secret.value
    }
  }

  ingress {
    external_enabled = true
    target_port      = 8080

    traffic_weight {
      latest_revision = true
      percentage      = 100
    }
  }

  template {
    min_replicas = 1
    max_replicas = 1

    volume {
      name         = "data"
      storage_name = azurerm_container_app_environment_storage.this.name
      storage_type = "NfsAzureFile"
    }

    init_container {
      name    = "chown"
      image   = "busybox:1.37"
      cpu     = 0.25
      memory  = "0.5Gi"
      command = ["chown", "70:70", "/data"]

      volume_mounts {
        name = "data"
        path = "/data"
      }
    }

    container {
      name   = "archive"
      image  = "ghcr.io/mulholland-inc/archive:${var.image_tag}"
      cpu    = var.cpu
      memory = var.memory

      dynamic "env" {
        for_each = merge(var.env, local.client_ids, {
          ARCHIVE_URL         = local.url
        })
        content {
          name  = env.key
          value = env.value
        }
      }

      env {
        name        = "DATABASE_URL"
        secret_name = "database-url"
      }

      env {
        name        = "ARCHIVE_SECRET_KEY"
        secret_name = "secret-key"
      }

      dynamic "env" {
        for_each = local.client_secrets
        content {
          name        = env.key
          secret_name = replace(lower(env.key), "_", "-")
        }
      }

      volume_mounts {
        name = "data"
        path = "/data"
      }

      startup_probe {
        transport               = "HTTP"
        port                    = 8080
        path                    = "/healthz"
        failure_count_threshold = 30
        interval_seconds        = 10
      }
    }
  }

  depends_on = [
    azurerm_postgresql_flexible_server_configuration.extensions,
    azurerm_postgresql_flexible_server_firewall_rule.azure,
  ]
}
