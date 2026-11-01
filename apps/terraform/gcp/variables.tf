variable "project" {
  type        = string
  description = "Google Cloud project ID."
}

variable "region" {
  type        = string
  description = "Region to deploy into."
  default     = "us-central1"
}

variable "domain" {
  type        = string
  description = "Cloudflare zone of Archive, like archive.dev: the web app is on app.<domain> (ARCHIVE_URL), static files on a Worker of its own that the image workflow deploys, and the API, sign-in, MCP, webhooks and OAuth redirects on api.<domain>, a Cloud Run domain mapping behind Cloudflare's proxy. The zone is verified with Google as the archive-verify service account. Its token goes in CLOUDFLARE_API_TOKEN; the README lists the permissions it needs."
}

variable "github_repository" {
  type        = string
  description = "owner/repo whose main branch deploys each new image to the service, like Mulholland-Inc/archive; its Actions variables are set with GITHUB_TOKEN. Empty deploys nothing."
  default     = ""
}

variable "image_tag" {
  type        = string
  description = "Tag of ghcr.io/mulholland-inc/archive to run first; later deploys change the image outside Terraform."
  default     = "latest"
}

variable "google_client_id" {
  type        = string
  description = "OAuth client id of sign-in with Google; its redirect URL is <api>/auth/oidc/google/callback. Google shows on the sign-in page when this and the secret are set."
  default     = ""
}

variable "google_client_secret" {
  type        = string
  description = "OAuth client secret of sign-in with Google, kept in Secret Manager with the id as ARCHIVE_GOOGLE."
  default     = ""
  sensitive   = true
}

variable "microsoft_client_id" {
  type        = string
  description = "OAuth client id of sign-in with Microsoft; its redirect URL is <api>/auth/oidc/microsoft/callback. Microsoft shows on the sign-in page when this and the secret are set."
  default     = ""
}

variable "microsoft_client_secret" {
  type        = string
  description = "OAuth client secret of sign-in with Microsoft, kept in Secret Manager with the id as ARCHIVE_MICROSOFT."
  default     = ""
  sensitive   = true
}

variable "slack_client_id" {
  type        = string
  description = "OAuth client id of sign-in with Slack; its redirect URL is <api>/auth/oidc/slack/callback. Slack shows on the sign-in page when this and the secret are set."
  default     = ""
}

variable "slack_client_secret" {
  type        = string
  description = "OAuth client secret of sign-in with Slack, kept in Secret Manager with the id as ARCHIVE_SLACK."
  default     = ""
  sensitive   = true
}

variable "github_client_id" {
  type        = string
  description = "OAuth client id of sign-in with GitHub; its redirect URL is <api>/auth/oidc/github/callback. GitHub shows on the sign-in page when this and the secret are set."
  default     = ""
}

variable "github_client_secret" {
  type        = string
  description = "OAuth client secret of sign-in with GitHub, kept in Secret Manager with the id as ARCHIVE_GITHUB."
  default     = ""
  sensitive   = true
}

variable "anthropic_api_key" {
  type        = string
  description = "Anthropic API key for the in-app agent (ANTHROPIC_API_KEY), kept in Secret Manager. Empty leaves the agent off."
  default     = ""
  sensitive   = true
}

variable "typesafe_api_key" {
  type        = string
  description = "TypeSafe API key for archive.browse (TYPESAFE_API_KEY), kept in Secret Manager. Empty leaves browse off."
  default     = ""
  sensitive   = true
}

variable "browser_max_instances" {
  type        = number
  description = "Most instances of the archive-browser service, which runs one browser per instance at a time for archive.browse."
  default     = 10
}

variable "min_instances" {
  type        = number
  description = "Cloud Run instances kept running. 0 scales to zero; Cloud Tasks starts one when a workflow or file is due."
  default     = 0
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
  default     = "db-f1-micro"
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
