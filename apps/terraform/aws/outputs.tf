output "url" {
  description = "Public URL of Archive."
  value       = local.url
}

output "load_balancer_dns_name" {
  description = "DNS name to point domain at."
  value       = aws_lb.this.dns_name
}

output "bucket" {
  description = "S3 bucket holding the files."
  value       = aws_s3_bucket.this.bucket
}

output "database_endpoint" {
  description = "RDS endpoint."
  value       = aws_db_instance.this.endpoint
}
