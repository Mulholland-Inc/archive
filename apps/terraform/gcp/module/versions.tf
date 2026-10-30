terraform {
  required_version = ">= 1.9"

  required_providers {
    google = {
      source                = "hashicorp/google"
      version               = "~> 7.0"
      configuration_aliases = [google.verify]
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
