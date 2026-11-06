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
  description = "Cloudflare zone of Archive, like archive.dev: the web app is on app.<domain> (ARCHIVE_URL), static files on a Worker of its own that the image workflow deploys, and the API, sign-in, MCP, webhooks and OAuth redirects on api.<domain>, with MCP and its OAuth on mcp.<domain> too, each a Cloud Run domain mapping behind Cloudflare's proxy. The zone is verified with Google as the archive-verify service account. Its token goes in CLOUDFLARE_API_TOKEN."
}

variable "github_repository" {
  type        = string
  description = "owner/repo whose main branch deploys each new image to the service, like Mulholland-Inc/archive; its Actions variables are set with GITHUB_TOKEN. Empty deploys nothing."
  default     = ""
}

variable "image_tag" {
  type        = string
  description = "Tag of ghcr.io/mulholland-inc/archive to run first, and with a browser- prefix of the archive-browser service; later deploys change the images outside Terraform."
  default     = "latest"
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
  default     = 2
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
  description = "Cloud SQL machine tier. db-custom-1-3840 is 1 dedicated vCPU and 3.75 GB, with room for about 100 connections; db-f1-micro allows only 25, which an instance per revision of a deploy runs out of."
  default     = "db-custom-1-3840"
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
