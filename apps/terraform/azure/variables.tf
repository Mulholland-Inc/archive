variable "subscription_id" {
  type        = string
  description = "Azure subscription ID; null uses ARM_SUBSCRIPTION_ID."
  default     = null
}

variable "location" {
  type        = string
  description = "Azure region to deploy into."
  default     = "westeurope"
}

variable "name" {
  type        = string
  description = "Name used for every resource."
  default     = "archive"
}

variable "image_tag" {
  type        = string
  description = "Tag of ghcr.io/mulholland-inc/archive to run."
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
  description = "OAuth client secret of sign-in with Google (ARCHIVE_GOOGLE_CLIENT_SECRET), kept in the container app's secrets."
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
  description = "OAuth client secret of sign-in with Microsoft (ARCHIVE_MICROSOFT_CLIENT_SECRET), kept in the container app's secrets."
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
  description = "OAuth client secret of sign-in with Slack (ARCHIVE_SLACK_CLIENT_SECRET), kept in the container app's secrets."
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
  description = "OAuth client secret of sign-in with GitHub (ARCHIVE_GITHUB_CLIENT_SECRET), kept in the container app's secrets."
  default     = ""
  sensitive   = true
}

variable "cpu" {
  type        = number
  description = "Container CPU cores."
  default     = 0.75
}

variable "memory" {
  type        = string
  description = "Container memory."
  default     = "1.5Gi"
}

variable "db_sku" {
  type        = string
  description = "PostgreSQL Flexible Server SKU."
  default     = "B_Standard_B1ms"
}

variable "db_storage_mb" {
  type        = number
  description = "PostgreSQL storage in MB."
  default     = 32768
}

variable "share_quota" {
  type        = number
  description = "Size of the file share in GiB (at least 100)."
  default     = 100
}
