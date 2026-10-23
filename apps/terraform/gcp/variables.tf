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

variable "image_tag" {
  type        = string
  description = "Tag of ghcr.io/mulholland-inc/archive to run."
  default     = "latest"
}

variable "admin_email" {
  type        = string
  description = "Email of the first owner."
}

variable "instance" {
  type        = string
  description = "Prefix of every Postgres role of this instance (ARCHIVE_INSTANCE)."
  default     = "a"
}

variable "env" {
  type        = map(string)
  description = "Extra environment variables, such as ARCHIVE_LLM_URL, ARCHIVE_LLM_MODEL and ARCHIVE_EMBED_MODEL."
  default     = {}
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

variable "deletion_protection" {
  type        = bool
  description = "Protect the database and service from deletion."
  default     = true
}
