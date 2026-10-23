data "google_project" "this" {}

locals {
  url = "https://${var.name}-${data.google_project.this.number}.${var.region}.run.app"
  secrets = {
    DATABASE_URL        = "host=/cloudsql/${google_sql_database_instance.this.connection_name} user=${google_sql_user.this.name} password=${random_password.db.result} dbname=${google_sql_database.this.name}"
    ARCHIVE_SECRET_KEY  = random_id.secret_key.hex
    ARCHIVE_BLOB_SECRET = google_storage_hmac_key.this.secret
  }
}

resource "google_project_service" "this" {
  for_each           = toset(["run.googleapis.com", "sqladmin.googleapis.com", "secretmanager.googleapis.com", "iam.googleapis.com"])
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
  for_each  = toset(["DATABASE_URL", "ARCHIVE_SECRET_KEY", "ARCHIVE_BLOB_SECRET"])
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
      min_instance_count = 1
      max_instance_count = 1
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
          ARCHIVE_ADMIN_EMAIL = var.admin_email
          ARCHIVE_INSTANCE    = var.instance
          ARCHIVE_BLOB_URL    = "s3://${google_storage_bucket.this.name}?endpoint=https://storage.googleapis.com&region=auto"
          ARCHIVE_BLOB_KEY    = google_storage_hmac_key.this.access_id
        })
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
