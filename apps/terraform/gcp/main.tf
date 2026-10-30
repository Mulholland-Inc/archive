terraform {
  required_version = ">= 1.9"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.28"
    }
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project
  region  = var.region
}

provider "google" {
  alias                       = "verify"
  project                     = var.project
  region                      = var.region
  impersonate_service_account = "${var.name}-verify@${var.project}.iam.gserviceaccount.com"
  scopes                      = ["https://www.googleapis.com/auth/cloud-platform", "https://www.googleapis.com/auth/siteverification"]
}

provider "github" {
  owner = split("/", var.github_repository)[0]
}

module "archive" {
  source = "./module"

  providers = {
    google        = google
    google.verify = google.verify
  }

  project                 = var.project
  region                  = var.region
  name                    = var.name
  app_domain              = var.app_domain
  api_domain              = var.api_domain
  cloudflare_zone         = var.cloudflare_zone
  github_repository       = var.github_repository
  image_tag               = var.image_tag
  env                     = var.env
  google_client_id        = var.google_client_id
  google_client_secret    = var.google_client_secret
  microsoft_client_id     = var.microsoft_client_id
  microsoft_client_secret = var.microsoft_client_secret
  slack_client_id         = var.slack_client_id
  slack_client_secret     = var.slack_client_secret
  github_client_id        = var.github_client_id
  github_client_secret    = var.github_client_secret
  anthropic_api_key       = var.anthropic_api_key
  min_instances           = var.min_instances
  max_instances           = var.max_instances
  cpu                     = var.cpu
  memory                  = var.memory
  db_tier                 = var.db_tier
  db_max_connections      = var.db_max_connections
  deletion_protection     = var.deletion_protection
}

output "url" {
  value = module.archive.url
}

output "api_url" {
  value = module.archive.api_url
}

output "bucket" {
  value = module.archive.bucket
}

output "database_connection_name" {
  value = module.archive.database_connection_name
}

output "cli" {
  value = module.archive.cli
}
