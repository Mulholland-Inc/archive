variable "region" {
  type        = string
  description = "AWS region to deploy into."
  default     = "us-east-1"
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
  description = "OAuth client secret of sign-in with Google (ARCHIVE_GOOGLE_CLIENT_SECRET), kept in Secrets Manager."
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
  description = "OAuth client secret of sign-in with Microsoft (ARCHIVE_MICROSOFT_CLIENT_SECRET), kept in Secrets Manager."
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
  description = "OAuth client secret of sign-in with Slack (ARCHIVE_SLACK_CLIENT_SECRET), kept in Secrets Manager."
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
  description = "OAuth client secret of sign-in with GitHub (ARCHIVE_GITHUB_CLIENT_SECRET), kept in Secrets Manager."
  default     = ""
  sensitive   = true
}

variable "vpc_id" {
  type        = string
  description = "VPC to deploy into; null uses the default VPC."
  default     = null
}

variable "subnet_ids" {
  type        = list(string)
  description = "Public subnets in at least two zones; null uses every subnet of the VPC."
  default     = null
}

variable "domain" {
  type        = string
  description = "Public host name pointed at the load balancer; null uses the load balancer's own name."
  default     = null
}

variable "certificate_arn" {
  type        = string
  description = "ACM certificate for domain; null serves plain HTTP."
  default     = null
}

variable "cpu" {
  type        = number
  description = "Fargate task CPU units."
  default     = 1024
}

variable "memory" {
  type        = number
  description = "Fargate task memory in MiB."
  default     = 2048
}

variable "db_instance_class" {
  type        = string
  description = "RDS instance class."
  default     = "db.t4g.small"
}

variable "db_storage" {
  type        = number
  description = "RDS storage in GiB."
  default     = 20
}

variable "deletion_protection" {
  type        = bool
  description = "Protect the database from deletion."
  default     = true
}
