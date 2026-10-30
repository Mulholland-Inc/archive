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
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.13"
    }
  }
}

provider "github" {
  owner = split("/", var.github_repository)[0]
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
