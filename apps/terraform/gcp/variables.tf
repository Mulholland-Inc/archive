variable "project" {
  type        = string
  description = "Google Cloud project ID."
}

variable "region" {
  type        = string
  description = "Region to deploy into."
  default     = "us-central1"
}

variable "name" {
  type        = string
  description = "Name used for every resource."
  default     = "archive"
}

variable "app_domain" {
  type        = string
  description = "Domain of the web app and sign-in, like app.archive.dev (ARCHIVE_URL), on Cloudflare. Empty uses the run.app URL."
  default     = ""
}

variable "api_domain" {
  type        = string
  description = "Domain of the API, MCP, webhooks and OAuth redirects, like api.archive.dev (ARCHIVE_API_URL), on Cloudflare. Empty uses the app's address."
  default     = ""
}

variable "cloudflare_account_id" {
  type        = string
  description = "Cloudflare account whose Worker serves app_domain and api_domain. Its token goes in CLOUDFLARE_API_TOKEN."
  default     = ""
}

variable "cloudflare_zone_id" {
  type        = string
  description = "Cloudflare zone holding app_domain and api_domain."
  default     = ""
}

variable "github_repository" {
  type        = string
  description = "owner/repo whose main branch may deploy new images to the service, like Mulholland-Inc/archive. Empty allows none."
  default     = ""
}

variable "image_tag" {
  type        = string
  description = "Tag of ghcr.io/mulholland-inc/archive to run first; later deploys change the image outside Terraform."
  default     = "latest"
}

variable "admin_email" {
  type        = string
  description = "Email of the first owner."
}

variable "env" {
  type        = map(string)
  description = "Extra environment variables, such as ARCHIVE_OIDC and ARCHIVE_LOCALE."
  default     = {}
}

variable "anthropic_api_key" {
  type        = string
  description = "Anthropic API key for the in-app agent (ANTHROPIC_API_KEY), kept in Secret Manager. Empty leaves the agent off."
  default     = ""
  sensitive   = true
}

variable "min_instances" {
  type        = number
  description = "Cloud Run instances kept running; scheduled and table-triggered workflows only run while one is up."
  default     = 1
}

variable "max_instances" {
  type        = number
  description = "Most Cloud Run instances."
  default     = 1
}

variable "cpu" {
  type        = string
  description = "Cloud Run CPU limit."
  default     = "1"
}

variable "memory" {
  type        = string
  description = "Cloud Run memory limit."
  default     = "2Gi"
}

variable "db_tier" {
  type        = string
  description = "Cloud SQL machine tier."
  default     = "db-g1-small"
}

variable "db_max_connections" {
  type        = number
  description = "Cloud SQL max_connections. Null keeps the tier's default."
  default     = null
}

variable "deletion_protection" {
  type        = bool
  description = "Protect the database and service from deletion."
  default     = true
}
