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
