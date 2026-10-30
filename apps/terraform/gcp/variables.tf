variable "project" {
  type = string
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "name" {
  type    = string
  default = "archive"
}

variable "app_domain" {
  type = string
}

variable "api_domain" {
  type = string
}

variable "cloudflare_zone" {
  type = string
}

variable "github_repository" {
  type    = string
  default = ""
}

variable "image_tag" {
  type    = string
  default = "latest"
}

variable "env" {
  type    = map(string)
  default = {}
}

variable "google_client_id" {
  type    = string
  default = ""
}

variable "google_client_secret" {
  type      = string
  default   = ""
  sensitive = true
}

variable "microsoft_client_id" {
  type    = string
  default = ""
}

variable "microsoft_client_secret" {
  type      = string
  default   = ""
  sensitive = true
}

variable "slack_client_id" {
  type    = string
  default = ""
}

variable "slack_client_secret" {
  type      = string
  default   = ""
  sensitive = true
}

variable "github_client_id" {
  type    = string
  default = ""
}

variable "github_client_secret" {
  type      = string
  default   = ""
  sensitive = true
}

variable "anthropic_api_key" {
  type      = string
  default   = ""
  sensitive = true
}

variable "min_instances" {
  type    = number
  default = 0
}

variable "max_instances" {
  type    = number
  default = 1
}

variable "cpu" {
  type    = string
  default = "1"
}

variable "memory" {
  type    = string
  default = "2Gi"
}

variable "db_tier" {
  type    = string
  default = "db-f1-micro"
}

variable "db_max_connections" {
  type    = number
  default = null
}

variable "deletion_protection" {
  type    = bool
  default = true
}
