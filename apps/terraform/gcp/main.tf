resource "google_project_service" "crm" {
  service            = "cloudresourcemanager.googleapis.com"
  disable_on_destroy = false
}

data "google_project" "this" {
  depends_on = [google_project_service.crm]
}

locals {
  run_url = "https://${var.name}-${data.google_project.this.number}.${var.region}.run.app"
  url     = var.app_domain != "" ? "https://${var.app_domain}" : local.run_url
  api_url = var.api_domain != "" ? "https://${var.api_domain}" : local.url
  split   = var.api_domain != "" && var.api_domain != var.app_domain
  domains = toset(compact([var.app_domain, var.api_domain]))
  run     = local.split ? var.api_domain : var.app_domain

  required = {
    DATABASE_URL       = "host=/cloudsql/${google_sql_database_instance.this.connection_name} user=${google_sql_user.this.name} password=${random_password.db.result} dbname=${google_sql_database.this.name}"
    ARCHIVE_SECRET_KEY = random_id.secret_key.hex
  }
  optional = {
    ANTHROPIC_API_KEY               = var.anthropic_api_key
    ARCHIVE_GOOGLE_CLIENT_SECRET    = var.google_client_secret
    ARCHIVE_MICROSOFT_CLIENT_SECRET = var.microsoft_client_secret
    ARCHIVE_SLACK_CLIENT_SECRET     = var.slack_client_secret
    ARCHIVE_GITHUB_CLIENT_SECRET    = var.github_client_secret
  }
  client_ids = { for k, v in {
    ARCHIVE_GOOGLE_CLIENT_ID    = var.google_client_id
    ARCHIVE_MICROSOFT_CLIENT_ID = var.microsoft_client_id
    ARCHIVE_SLACK_CLIENT_ID     = var.slack_client_id
    ARCHIVE_GITHUB_CLIENT_ID    = var.github_client_id
  } : k => v if v != "" }
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
  account_id = var.name

  depends_on = [google_project_service.this]
}

resource "google_project_iam_member" "sql" {
  project = var.project
  role    = "roles/cloudsql.client"
  member  = google_service_account.this.member
}

resource "google_service_account" "tasks" {
  account_id = "${var.name}-tasks"

  depends_on = [google_project_service.this]
}

resource "google_service_account_iam_member" "tasks" {
  service_account_id = google_service_account.tasks.name
  role               = "roles/iam.serviceAccountUser"
  member             = google_service_account.this.member
}

resource "google_cloud_tasks_queue" "this" {
  name     = var.name
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

locals {
  image = "${var.region}-docker.pkg.dev/${var.project}/${google_artifact_registry_repository.ghcr.repository_id}/mulholland-inc/archive:${var.image_tag}"
  env = merge(var.env, local.client_ids, {
    ARCHIVE_URL      = local.url
    ARCHIVE_API_URL  = local.api_url
    ARCHIVE_BLOB_URL = "gs://${google_storage_bucket.this.name}"

    ARCHIVE_TASKS_QUEUE           = google_cloud_tasks_queue.this.id
    ARCHIVE_TASKS_SERVICE_ACCOUNT = google_service_account.tasks.email
  })
}

resource "google_cloud_run_v2_service" "this" {
  name                 = var.name
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
  name                = var.name
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
  count = length(local.domains) > 0 ? 1 : 0
  filter = {
    name = var.cloudflare_zone
  }
}

data "google_client_openid_userinfo" "me" {}

resource "google_service_account" "verify" {
  count      = local.run != "" ? 1 : 0
  account_id = "${var.name}-verify"

  depends_on = [google_project_service.this]
}

resource "google_service_account_iam_member" "verify" {
  count              = local.run != "" ? 1 : 0
  service_account_id = google_service_account.verify[0].name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "${endswith(data.google_client_openid_userinfo.me.email, ".gserviceaccount.com") ? "serviceAccount" : "user"}:${data.google_client_openid_userinfo.me.email}"
}

resource "google_project_iam_member" "verify" {
  count   = local.run != "" ? 1 : 0
  project = var.project
  role    = "roles/run.admin"
  member  = google_service_account.verify[0].member
}

resource "time_sleep" "verify" {
  count           = local.run != "" ? 1 : 0
  create_duration = "3m"

  depends_on = [google_service_account_iam_member.verify, google_project_iam_member.verify]
}

data "google_site_verification_token" "this" {
  count               = local.run != "" ? 1 : 0
  provider            = google.verify
  type                = "INET_DOMAIN"
  identifier          = var.cloudflare_zone
  verification_method = "DNS_TXT"

  depends_on = [time_sleep.verify]
}

resource "cloudflare_dns_record" "verify" {
  count   = local.run != "" ? 1 : 0
  zone_id = data.cloudflare_zone.this[0].zone_id
  name    = var.cloudflare_zone
  type    = "TXT"
  content = "\"${data.google_site_verification_token.this[0].token}\""
  ttl     = 1
}

resource "google_site_verification_web_resource" "this" {
  count    = local.run != "" ? 1 : 0
  provider = google.verify
  site {
    type       = "INET_DOMAIN"
    identifier = var.cloudflare_zone
  }
  verification_method = "DNS_TXT"

  depends_on = [cloudflare_dns_record.verify]
}

resource "google_cloud_run_domain_mapping" "this" {
  count    = local.run != "" ? 1 : 0
  provider = google.verify
  name     = local.run
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
  count   = local.run != "" ? 1 : 0
  zone_id = data.cloudflare_zone.this[0].zone_id
  name    = local.run
  type    = "CNAME"
  content = "ghs.googlehosted.com"
  proxied = true
  ttl     = 1
}

resource "cloudflare_ruleset" "run" {
  count   = local.run != "" ? 1 : 0
  zone_id = data.cloudflare_zone.this[0].zone_id
  name    = "${var.name} origin TLS"
  kind    = "zone"
  phase   = "http_config_settings"
  rules = [{
    description       = "${local.run} reaches Google over verified TLS"
    expression        = "http.host eq \"${local.run}\""
    action            = "set_config"
    action_parameters = { ssl = "strict" }
    }, {
    description       = "Google's certificate challenges reach it over HTTP, before it has a certificate"
    expression        = "http.host eq \"${local.run}\" and starts_with(http.request.uri.path, \"/.well-known/acme-challenge/\")"
    action            = "set_config"
    action_parameters = { ssl = "flexible" }
  }]
}

resource "cloudflare_workers_script" "app" {
  count       = local.split ? 1 : 0
  account_id  = data.cloudflare_zone.this[0].account.id
  script_name = "${var.name}-app"
  main_module = "worker.js"
  content     = "export default { fetch: () => new Response(null, { status: 503 }) };"

  lifecycle {
    ignore_changes = all
  }
}

resource "cloudflare_workers_custom_domain" "app" {
  count      = local.split ? 1 : 0
  account_id = data.cloudflare_zone.this[0].account.id
  zone_id    = data.cloudflare_zone.this[0].zone_id
  hostname   = var.app_domain
  service    = cloudflare_workers_script.app[0].script_name
}

data "cloudflare_account_api_token_permission_groups_list" "this" {
  count      = local.split && var.github_repository != "" ? 1 : 0
  account_id = data.cloudflare_zone.this[0].account.id
}

resource "cloudflare_account_token" "app" {
  count      = local.split && var.github_repository != "" ? 1 : 0
  account_id = data.cloudflare_zone.this[0].account.id
  name       = "${var.name}-app-deploy"
  policies = [{
    effect            = "allow"
    permission_groups = [for g in data.cloudflare_account_api_token_permission_groups_list.this[0].result : { id = g.id } if g.name == "Workers Scripts Write"]
    resources         = jsonencode({ "com.cloudflare.api.account.${data.cloudflare_zone.this[0].account.id}" = "*" })
  }]
}

resource "github_actions_secret" "app" {
  count       = local.split && var.github_repository != "" ? 1 : 0
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
  account_id = "${var.name}-deploy"
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
  for_each = var.github_repository != "" ? merge({
    GCP_PROJECT                    = var.project
    GCP_REGION                     = var.region
    GCP_SERVICE                    = google_cloud_run_v2_service.this.name
    GCP_SERVICE_ACCOUNT            = google_service_account.deploy[0].email
    GCP_WORKLOAD_IDENTITY_PROVIDER = google_iam_workload_identity_pool_provider.github[0].name
    }, local.split ? {
    ARCHIVE_API_URL       = local.api_url
    CLOUDFLARE_ACCOUNT_ID = data.cloudflare_zone.this[0].account.id
    CLOUDFLARE_WORKER     = cloudflare_workers_script.app[0].script_name
  } : {}) : {}
  repository    = split("/", var.github_repository)[1]
  variable_name = each.key
  value         = each.value
}
