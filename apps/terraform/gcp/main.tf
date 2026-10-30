resource "google_project_service" "crm" {
  service            = "cloudresourcemanager.googleapis.com"
  disable_on_destroy = false
}

locals {
  url     = "https://app.${var.domain}"
  api_url = "https://api.${var.domain}"

  required = {
    DATABASE_URL       = "host=/cloudsql/${google_sql_database_instance.this.connection_name} user=${google_sql_user.this.name} password=${random_password.db.result} dbname=${google_sql_database.this.name}"
    ARCHIVE_SECRET_KEY = random_id.secret_key.hex
  }
  signin = {
    ARCHIVE_GOOGLE    = ["accounts.google.com", var.google_client_id, var.google_client_secret]
    ARCHIVE_MICROSOFT = ["login.microsoftonline.com", var.microsoft_client_id, var.microsoft_client_secret]
    ARCHIVE_SLACK     = ["slack.com", var.slack_client_id, var.slack_client_secret]
    ARCHIVE_GITHUB    = ["github.com", var.github_client_id, var.github_client_secret]
  }
  optional = merge({ ANTHROPIC_API_KEY = var.anthropic_api_key }, { for k, v in local.signin :
    k => v[1] != "" && v[2] != "" ? "https://${replace(urlencode(v[1]), "+", "%20")}:${replace(urlencode(v[2]), "+", "%20")}@${v[0]}" : ""
  })
  secrets = merge(local.required, local.optional)
  secret_names = setunion(toset(keys(local.required)), toset([
    for k, v in local.optional : k if nonsensitive(v != "")
  ]))

  services = ["run.googleapis.com", "sqladmin.googleapis.com", "secretmanager.googleapis.com", "iam.googleapis.com", "storage.googleapis.com", "artifactregistry.googleapis.com", "iamcredentials.googleapis.com", "sts.googleapis.com", "cloudtasks.googleapis.com", "siteverification.googleapis.com"]
}

resource "google_project_service" "this" {
  for_each           = toset(local.services)
  service            = each.value
  disable_on_destroy = false

  depends_on = [google_project_service.crm]
}

resource "google_artifact_registry_repository" "ghcr" {
  repository_id = "ghcr"
  location      = var.region
  format        = "DOCKER"
  mode          = "REMOTE_REPOSITORY"

  remote_repository_config {
    description = "ghcr.io"
    docker_repository {
      custom_repository {
        uri = "https://ghcr.io"
      }
    }
  }

  depends_on = [google_project_service.this]
}

resource "random_password" "db" {
  length  = 32
  special = false
}

resource "random_id" "secret_key" {
  byte_length = 32
}

resource "google_service_account" "this" {
  account_id = "archive"

  depends_on = [google_project_service.this]
}

resource "google_project_iam_member" "sql" {
  project = var.project
  role    = "roles/cloudsql.client"
  member  = google_service_account.this.member
}

resource "google_service_account" "tasks" {
  account_id = "archive-tasks"

  depends_on = [google_project_service.this]
}

resource "google_service_account_iam_member" "tasks" {
  service_account_id = google_service_account.tasks.name
  role               = "roles/iam.serviceAccountUser"
  member             = google_service_account.this.member
}

resource "google_cloud_tasks_queue" "this" {
  name     = "archive"
  location = var.region

  retry_config {
    min_backoff = "1s"
    max_backoff = "600s"
  }

  depends_on = [google_project_service.this]
}

resource "google_cloud_tasks_queue_iam_member" "this" {
  name     = google_cloud_tasks_queue.this.name
  location = var.region
  role     = "roles/cloudtasks.enqueuer"
  member   = google_service_account.this.member
}

resource "google_sql_database_instance" "this" {
  name                = "archive"
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
  name                        = "${var.project}-archive"
  location                    = var.region
  uniform_bucket_level_access = true

  depends_on = [google_project_service.this]
}

resource "google_storage_bucket_iam_member" "this" {
  bucket = google_storage_bucket.this.name
  role   = "roles/storage.objectAdmin"
  member = google_service_account.this.member
}

resource "google_secret_manager_secret" "this" {
  for_each  = local.secret_names
  secret_id = "archive-${replace(lower(each.key), "_", "-")}"

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

locals {
  image = "${var.region}-docker.pkg.dev/${var.project}/${google_artifact_registry_repository.ghcr.repository_id}/mulholland-inc/archive:${var.image_tag}"
  env = {
    ARCHIVE_URL      = local.url
    ARCHIVE_BLOB_URL = "gs://${google_storage_bucket.this.name}"
  }
}

resource "google_cloud_run_v2_service" "this" {
  name                 = "archive"
  location             = var.region
  ingress              = "INGRESS_TRAFFIC_ALL"
  invoker_iam_disabled = true
  deletion_protection  = var.deletion_protection

  template {
    service_account = google_service_account.this.email
    timeout         = "600s"

    scaling {
      min_instance_count = var.min_instances
      max_instance_count = var.max_instances
    }

    containers {
      image = local.image
      args  = ["serve"]

      ports {
        container_port = 8080
      }

      resources {
        cpu_idle          = true
        startup_cpu_boost = true
        limits = {
          cpu    = var.cpu
          memory = var.memory
        }
      }

      dynamic "env" {
        for_each = local.env
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

  lifecycle {
    ignore_changes = [template[0].containers[0].image, client, client_version]
  }

  depends_on = [
    google_project_iam_member.sql,
    google_cloud_tasks_queue_iam_member.this,
    google_service_account_iam_member.tasks,
    google_secret_manager_secret_iam_member.this,
    google_secret_manager_secret_version.this,
    google_storage_bucket_iam_member.this,
  ]
}

resource "google_cloud_run_v2_job" "this" {
  name                = "archive"
  location            = var.region
  deletion_protection = var.deletion_protection

  template {
    template {
      service_account = google_service_account.this.email
      timeout         = "600s"
      max_retries     = 0

      containers {
        image = local.image
        args  = ["help"]

        resources {
          limits = {
            cpu    = var.cpu
            memory = var.memory
          }
        }

        dynamic "env" {
          for_each = local.env
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
  }

  lifecycle {
    ignore_changes = [template[0].template[0].containers[0].image, client, client_version]
  }

  depends_on = [
    google_project_iam_member.sql,
    google_cloud_tasks_queue_iam_member.this,
    google_service_account_iam_member.tasks,
    google_secret_manager_secret_iam_member.this,
    google_secret_manager_secret_version.this,
    google_storage_bucket_iam_member.this,
  ]
}

data "cloudflare_zone" "this" {
  filter = {
    name = var.domain
  }
}

data "google_client_openid_userinfo" "me" {}

resource "google_service_account" "verify" {
  account_id = "archive-verify"

  depends_on = [google_project_service.this]
}

resource "google_service_account_iam_member" "verify" {
  service_account_id = google_service_account.verify.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "${endswith(data.google_client_openid_userinfo.me.email, ".gserviceaccount.com") ? "serviceAccount" : "user"}:${data.google_client_openid_userinfo.me.email}"
}

resource "google_project_iam_member" "verify" {
  project = var.project
  role    = "roles/run.admin"
  member  = google_service_account.verify.member
}

resource "time_sleep" "verify" {
  create_duration = "3m"

  depends_on = [google_service_account_iam_member.verify, google_project_iam_member.verify]
}

data "google_site_verification_token" "this" {
  provider            = google.verify
  type                = "INET_DOMAIN"
  identifier          = var.domain
  verification_method = "DNS_TXT"

  depends_on = [time_sleep.verify]
}

resource "cloudflare_dns_record" "verify" {
  zone_id = data.cloudflare_zone.this.zone_id
  name    = var.domain
  type    = "TXT"
  content = "\"${data.google_site_verification_token.this.token}\""
  ttl     = 1
}

resource "google_site_verification_web_resource" "this" {
  provider = google.verify
  site {
    type       = "INET_DOMAIN"
    identifier = var.domain
  }
  verification_method = "DNS_TXT"

  depends_on = [cloudflare_dns_record.verify]
}

resource "google_site_verification_owner" "me" {
  provider        = google.verify
  web_resource_id = google_site_verification_web_resource.this.id
  email           = data.google_client_openid_userinfo.me.email
}

resource "google_cloud_run_domain_mapping" "this" {
  provider = google.verify
  name     = "api.${var.domain}"
  location = var.region

  metadata {
    namespace = var.project
  }

  spec {
    route_name = google_cloud_run_v2_service.this.name
  }

  depends_on = [google_site_verification_web_resource.this]
}

resource "cloudflare_dns_record" "run" {
  zone_id = data.cloudflare_zone.this.zone_id
  name    = "api.${var.domain}"
  type    = "CNAME"
  content = "ghs.googlehosted.com"
  proxied = true
  ttl     = 1
}

resource "cloudflare_ruleset" "run" {
  zone_id = data.cloudflare_zone.this.zone_id
  name    = "archive origin TLS"
  kind    = "zone"
  phase   = "http_config_settings"
  rules = [{
    description       = "api.${var.domain} reaches Google over verified TLS"
    expression        = "http.host eq \"api.${var.domain}\""
    action            = "set_config"
    action_parameters = { ssl = "strict" }
    }, {
    description       = "Google's certificate challenges reach it over HTTP, before it has a certificate"
    expression        = "http.host eq \"api.${var.domain}\" and starts_with(http.request.uri.path, \"/.well-known/acme-challenge/\")"
    action            = "set_config"
    action_parameters = { ssl = "flexible" }
  }]
}

resource "cloudflare_workers_script" "app" {
  account_id  = data.cloudflare_zone.this.account.id
  script_name = "archive-app"
  main_module = "worker.js"
  content     = "export default { fetch: () => new Response(null, { status: 503 }) };"

  lifecycle {
    ignore_changes = all
  }
}

resource "cloudflare_workers_custom_domain" "app" {
  account_id = data.cloudflare_zone.this.account.id
  zone_id    = data.cloudflare_zone.this.zone_id
  hostname   = "app.${var.domain}"
  service    = cloudflare_workers_script.app.script_name
}

data "cloudflare_account_api_token_permission_groups_list" "this" {
  count      = var.github_repository != "" ? 1 : 0
  account_id = data.cloudflare_zone.this.account.id
}

resource "cloudflare_account_token" "app" {
  count      = var.github_repository != "" ? 1 : 0
  account_id = data.cloudflare_zone.this.account.id
  name       = "archive-app-deploy"
  policies = [{
    effect            = "allow"
    permission_groups = [for g in data.cloudflare_account_api_token_permission_groups_list.this[0].result : { id = g.id } if g.name == "Workers Scripts Write"]
    resources         = jsonencode({ "com.cloudflare.api.account.${data.cloudflare_zone.this.account.id}" = "*" })
  }]
}

resource "github_actions_secret" "app" {
  count       = var.github_repository != "" ? 1 : 0
  repository  = split("/", var.github_repository)[1]
  secret_name = "CLOUDFLARE_API_TOKEN"
  value       = cloudflare_account_token.app[0].value
}

resource "google_iam_workload_identity_pool" "github" {
  count                     = var.github_repository != "" ? 1 : 0
  workload_identity_pool_id = "github"

  depends_on = [google_project_service.this]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  count                              = var.github_repository != "" ? 1 : 0
  workload_identity_pool_id          = google_iam_workload_identity_pool.github[0].workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
    "attribute.ref"        = "assertion.ref"
  }
  attribute_condition = "assertion.repository == '${var.github_repository}' && assertion.ref == 'refs/heads/main'"
  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account" "deploy" {
  count      = var.github_repository != "" ? 1 : 0
  account_id = "archive-deploy"
}

resource "google_service_account_iam_member" "deploy" {
  count              = var.github_repository != "" ? 1 : 0
  service_account_id = google_service_account.deploy[0].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github[0].name}/attribute.repository/${var.github_repository}"
}

resource "google_cloud_run_v2_service_iam_member" "deploy" {
  count    = var.github_repository != "" ? 1 : 0
  name     = google_cloud_run_v2_service.this.name
  location = var.region
  role     = "roles/run.developer"
  member   = google_service_account.deploy[0].member
}

resource "google_artifact_registry_repository_iam_member" "deploy" {
  count      = var.github_repository != "" ? 1 : 0
  location   = var.region
  repository = google_artifact_registry_repository.ghcr.name
  role       = "roles/artifactregistry.reader"
  member     = google_service_account.deploy[0].member
}

resource "google_cloud_run_v2_job_iam_member" "deploy" {
  count    = var.github_repository != "" ? 1 : 0
  name     = google_cloud_run_v2_job.this.name
  location = var.region
  role     = "roles/run.developer"
  member   = google_service_account.deploy[0].member
}

resource "google_service_account_iam_member" "deploy_runs_as" {
  count              = var.github_repository != "" ? 1 : 0
  service_account_id = google_service_account.this.name
  role               = "roles/iam.serviceAccountUser"
  member             = google_service_account.deploy[0].member
}

resource "github_actions_variable" "deploy" {
  for_each = var.github_repository != "" ? tomap({
    GCP_PROJECT                    = var.project
    GCP_REGION                     = var.region
    GCP_SERVICE_ACCOUNT            = google_service_account.deploy[0].email
    GCP_WORKLOAD_IDENTITY_PROVIDER = google_iam_workload_identity_pool_provider.github[0].name
    CLOUDFLARE_ACCOUNT_ID          = data.cloudflare_zone.this.account.id
  }) : {}
  repository    = split("/", var.github_repository)[1]
  variable_name = each.key
  value         = each.value
}
