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
  description = "Extra environment variables, such as ARCHIVE_LLM_URL, ARCHIVE_LLM_MODEL and ARCHIVE_LLM_KEY."
  default     = {}
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
