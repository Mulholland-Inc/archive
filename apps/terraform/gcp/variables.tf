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
  description = "Domain of the web app, like app.archive.dev (ARCHIVE_URL), on Cloudflare: static files on a Worker of its own that the image workflow deploys. Set with api_domain; both empty run the web app as the archive-web Cloud Run service on its run.app URL."
  default     = ""
}

variable "api_domain" {
  type        = string
  description = "Domain of the API, sign-in, MCP, webhooks and OAuth redirects, like api.archive.dev (ARCHIVE_API_URL), on Cloudflare: a Cloud Run domain mapping behind Cloudflare's proxy. Set with app_domain; both empty use the service's run.app URL."
  default     = ""

  validation {
    condition     = (var.api_domain == "") == (var.app_domain == "") && (var.api_domain == "" || var.api_domain != var.app_domain)
    error_message = "Set app_domain and api_domain together, to two different domains, or leave both empty: the web app and the API are always on their own addresses."
  }

  validation {
    condition     = var.api_domain == "" || var.app_domain != ""
    error_message = "api_domain needs app_domain: the web app on its own domain calls the API on api_domain."
  }
}

variable "cloudflare_zone" {
  type        = string
  description = "Cloudflare zone holding app_domain and api_domain, like archive.dev, verified with Google as the archive-verify service account. Its token goes in CLOUDFLARE_API_TOKEN; the README lists the permissions it needs."
  default     = ""
}

variable "github_repository" {
  type        = string
  description = "owner/repo whose main branch deploys each new image to the service, like Mulholland-Inc/archive; its Actions variables are set with GITHUB_TOKEN. Empty deploys nothing."
  default     = ""
}

variable "image_tag" {
  type        = string
  description = "Tag of ghcr.io/mulholland-inc/archive and archive-web to run first; later deploys change the images outside Terraform."
  default     = "latest"
}

variable "env" {
  type        = map(string)
  description = "Extra environment variables, such as ARCHIVE_LOCALE."
  default     = {}
}

variable "google_client_id" {
  type        = string
  description = "OAuth client id of sign-in with Google (ARCHIVE_GOOGLE_CLIENT_ID); its redirect URL is <api>/auth/oidc/google/callback. Google shows on the sign-in page when this and the secret are set."
  default     = ""
}

variable "google_client_secret" {
  type        = string
  description = "OAuth client secret of sign-in with Google (ARCHIVE_GOOGLE_CLIENT_SECRET), kept in Secret Manager."
  default     = ""
  sensitive   = true
}

variable "microsoft_client_id" {
  type        = string
  description = "OAuth client id of sign-in with Microsoft (ARCHIVE_MICROSOFT_CLIENT_ID); its redirect URL is <api>/auth/oidc/microsoft/callback. Microsoft shows on the sign-in page when this and the secret are set."
  default     = ""
}

variable "microsoft_client_secret" {
  type        = string
  description = "OAuth client secret of sign-in with Microsoft (ARCHIVE_MICROSOFT_CLIENT_SECRET), kept in Secret Manager."
  default     = ""
  sensitive   = true
}

variable "slack_client_id" {
  type        = string
  description = "OAuth client id of sign-in with Slack (ARCHIVE_SLACK_CLIENT_ID); its redirect URL is <api>/auth/oidc/slack/callback. Slack shows on the sign-in page when this and the secret are set."
  default     = ""
}

variable "slack_client_secret" {
  type        = string
  description = "OAuth client secret of sign-in with Slack (ARCHIVE_SLACK_CLIENT_SECRET), kept in Secret Manager."
  default     = ""
  sensitive   = true
}

variable "github_client_id" {
  type        = string
  description = "OAuth client id of sign-in with GitHub (ARCHIVE_GITHUB_CLIENT_ID); its redirect URL is <api>/auth/oidc/github/callback. GitHub shows on the sign-in page when this and the secret are set."
  default     = ""
}

variable "github_client_secret" {
  type        = string
  description = "OAuth client secret of sign-in with GitHub (ARCHIVE_GITHUB_CLIENT_SECRET), kept in Secret Manager."
  default     = ""
  sensitive   = true
}

variable "anthropic_api_key" {
  type        = string
  description = "Anthropic API key for the in-app agent (ANTHROPIC_API_KEY), kept in Secret Manager. Empty leaves the agent off."
  default     = ""
  sensitive   = true
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
