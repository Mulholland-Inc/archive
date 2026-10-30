data "google_project" "this" {}

locals {
  run_url = "https://${var.name}-${data.google_project.this.number}.${var.region}.run.app"
  url     = var.app_domain != "" ? "https://${var.app_domain}" : local.run_url
  api_url = var.api_domain != "" ? "https://${var.api_domain}" : local.url
  domains = toset(compact([var.app_domain, var.api_domain]))

  required = {
    DATABASE_URL        = "host=/cloudsql/${google_sql_database_instance.this.connection_name} user=${google_sql_user.this.name} password=${random_password.db.result} dbname=${google_sql_database.this.name}"
    ARCHIVE_SECRET_KEY  = random_id.secret_key.hex
    ARCHIVE_BLOB_SECRET = google_storage_hmac_key.this.secret
  }
  optional = {
    ANTHROPIC_API_KEY             = var.anthropic_api_key
    ARCHIVE_SMTP_URL              = var.smtp_url
    ARCHIVE_STRIPE_KEY            = var.stripe_key
    ARCHIVE_STRIPE_WEBHOOK_SECRET = var.stripe_webhook_secret
  }
  secrets = merge(local.required, local.optional)
  secret_names = setunion(toset(keys(local.required)), toset([
    for k, v in local.optional : k if nonsensitive(v != "")
  ]))

  services = ["run.googleapis.com", "sqladmin.googleapis.com", "secretmanager.googleapis.com", "iam.googleapis.com", "storage.googleapis.com"]
}

resource "google_project_service" "this" {
  for_each           = toset(local.services)
  service            = each.value
  disable_on_destroy = false
}

resource "random_password" "db" {
  length  = 32
  special = false
}

resource "random_id" "secret_key" {
  byte_length = 32
}

resource "google_service_account" "this" {
  account_id = var.name

  depends_on = [google_project_service.this]
}

resource "google_project_iam_member" "sql" {
  project = var.project
  role    = "roles/cloudsql.client"
  member  = google_service_account.this.member
}

resource "google_sql_database_instance" "this" {
  name                = var.name
  database_version    = "POSTGRES_18"
  region              = var.region
  deletion_protection = var.deletion_protection

  settings {
    edition = "ENTERPRISE"
    tier    = var.db_tier

    backup_configuration {
      enabled = true
    }

    dynamic "database_flags" {
      for_each = var.db_max_connections != null ? [var.db_max_connections] : []
      content {
        name  = "max_connections"
        value = tostring(database_flags.value)
      }
    }
  }

  depends_on = [google_project_service.this]
}

resource "google_sql_database" "this" {
  name     = "archive"
  instance = google_sql_database_instance.this.name
}

resource "google_sql_user" "this" {
  name     = "archive"
  instance = google_sql_database_instance.this.name
  password = random_password.db.result
}

resource "google_storage_bucket" "this" {
  name                        = "${var.project}-${var.name}"
  location                    = var.region
  uniform_bucket_level_access = true

  depends_on = [google_project_service.this]
}

resource "google_storage_bucket_iam_member" "this" {
  bucket = google_storage_bucket.this.name
  role   = "roles/storage.objectAdmin"
  member = google_service_account.this.member
}

resource "google_storage_hmac_key" "this" {
  service_account_email = google_service_account.this.email
}

resource "google_secret_manager_secret" "this" {
  for_each  = local.secret_names
  secret_id = "${var.name}-${replace(lower(each.key), "_", "-")}"

  replication {
    auto {}
  }

  depends_on = [google_project_service.this]
}

resource "google_secret_manager_secret_version" "this" {
  for_each    = google_secret_manager_secret.this
  secret      = each.value.id
  secret_data = local.secrets[each.key]
}

resource "google_secret_manager_secret_iam_member" "this" {
  for_each  = google_secret_manager_secret.this
  secret_id = each.value.id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.this.member
}

resource "google_cloud_run_v2_service" "this" {
  name                 = var.name
  location             = var.region
  ingress              = "INGRESS_TRAFFIC_ALL"
  invoker_iam_disabled = true
  deletion_protection  = var.deletion_protection

  template {
    service_account = google_service_account.this.email

    scaling {
      min_instance_count = var.min_instances
      max_instance_count = var.max_instances
    }

    containers {
      image = "ghcr.io/mulholland-inc/archive:${var.image_tag}"

      ports {
        container_port = 8080
      }

      resources {
        cpu_idle = false
        limits = {
          cpu    = var.cpu
          memory = var.memory
        }
      }

      dynamic "env" {
        for_each = merge(var.env, {
          ARCHIVE_URL         = local.url
          ARCHIVE_API_URL     = local.api_url
          ARCHIVE_ADMIN_EMAIL = var.admin_email
          ARCHIVE_BLOB_URL    = "s3://${google_storage_bucket.this.name}?endpoint=https://storage.googleapis.com&region=auto"
          ARCHIVE_BLOB_KEY    = google_storage_hmac_key.this.access_id
          }, var.smtp_from != "" ? {
          ARCHIVE_SMTP_FROM = var.smtp_from
        } : {})
        content {
          name  = env.key
          value = env.value
        }
      }

      dynamic "env" {
        for_each = google_secret_manager_secret.this
        content {
          name = env.key
          value_source {
            secret_key_ref {
              secret  = env.value.secret_id
              version = "latest"
            }
          }
        }
      }

      startup_probe {
        failure_threshold = 30
        period_seconds    = 10
        http_get {
          path = "/healthz"
        }
      }

      volume_mounts {
        name       = "cloudsql"
        mount_path = "/cloudsql"
      }
    }

    volumes {
      name = "cloudsql"
      cloud_sql_instance {
        instances = [google_sql_database_instance.this.connection_name]
      }
    }
  }

  depends_on = [
    google_project_iam_member.sql,
    google_secret_manager_secret_iam_member.this,
    google_secret_manager_secret_version.this,
    google_storage_bucket_iam_member.this,
  ]
}

resource "google_cloud_run_domain_mapping" "this" {
  for_each = local.domains
  name     = each.value
  location = var.region

  metadata {
    namespace = var.project
  }

  spec {
    route_name = google_cloud_run_v2_service.this.name
  }
}
