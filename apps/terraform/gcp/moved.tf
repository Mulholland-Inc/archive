moved {
  from = google_project_service.crm
  to   = module.archive.google_project_service.crm
}

moved {
  from = google_project_service.this
  to   = module.archive.google_project_service.this
}

moved {
  from = google_artifact_registry_repository.ghcr
  to   = module.archive.google_artifact_registry_repository.ghcr
}

moved {
  from = random_password.db
  to   = module.archive.random_password.db
}

moved {
  from = random_id.secret_key
  to   = module.archive.random_id.secret_key
}

moved {
  from = google_service_account.this
  to   = module.archive.google_service_account.this
}

moved {
  from = google_project_iam_member.sql
  to   = module.archive.google_project_iam_member.sql
}

moved {
  from = google_service_account.tasks
  to   = module.archive.google_service_account.tasks
}

moved {
  from = google_service_account_iam_member.tasks
  to   = module.archive.google_service_account_iam_member.tasks
}

moved {
  from = google_cloud_tasks_queue.this
  to   = module.archive.google_cloud_tasks_queue.this
}

moved {
  from = google_cloud_tasks_queue_iam_member.this
  to   = module.archive.google_cloud_tasks_queue_iam_member.this
}

moved {
  from = google_sql_database_instance.this
  to   = module.archive.google_sql_database_instance.this
}

moved {
  from = google_sql_database.this
  to   = module.archive.google_sql_database.this
}

moved {
  from = google_sql_user.this
  to   = module.archive.google_sql_user.this
}

moved {
  from = google_storage_bucket.this
  to   = module.archive.google_storage_bucket.this
}

moved {
  from = google_storage_bucket_iam_member.this
  to   = module.archive.google_storage_bucket_iam_member.this
}

moved {
  from = google_secret_manager_secret.this
  to   = module.archive.google_secret_manager_secret.this
}

moved {
  from = google_secret_manager_secret_version.this
  to   = module.archive.google_secret_manager_secret_version.this
}

moved {
  from = google_secret_manager_secret_iam_member.this
  to   = module.archive.google_secret_manager_secret_iam_member.this
}

moved {
  from = google_cloud_run_v2_service.this
  to   = module.archive.google_cloud_run_v2_service.this
}

moved {
  from = google_cloud_run_v2_job.this
  to   = module.archive.google_cloud_run_v2_job.this
}

moved {
  from = google_service_account.verify
  to   = module.archive.google_service_account.verify
}

moved {
  from = google_service_account_iam_member.verify
  to   = module.archive.google_service_account_iam_member.verify
}

moved {
  from = google_project_iam_member.verify
  to   = module.archive.google_project_iam_member.verify
}

moved {
  from = time_sleep.verify
  to   = module.archive.time_sleep.verify
}

moved {
  from = cloudflare_dns_record.verify
  to   = module.archive.cloudflare_dns_record.verify
}

moved {
  from = google_site_verification_web_resource.this
  to   = module.archive.google_site_verification_web_resource.this
}

moved {
  from = google_site_verification_owner.me
  to   = module.archive.google_site_verification_owner.me
}

moved {
  from = google_cloud_run_domain_mapping.this
  to   = module.archive.google_cloud_run_domain_mapping.this
}

moved {
  from = cloudflare_dns_record.run
  to   = module.archive.cloudflare_dns_record.run
}

moved {
  from = cloudflare_ruleset.run
  to   = module.archive.cloudflare_ruleset.run
}

moved {
  from = cloudflare_workers_script.app
  to   = module.archive.cloudflare_workers_script.app
}

moved {
  from = cloudflare_workers_custom_domain.app
  to   = module.archive.cloudflare_workers_custom_domain.app
}

moved {
  from = cloudflare_account_token.app
  to   = module.archive.cloudflare_account_token.app
}

moved {
  from = github_actions_secret.app
  to   = module.archive.github_actions_secret.app
}

moved {
  from = google_iam_workload_identity_pool.github
  to   = module.archive.google_iam_workload_identity_pool.github
}

moved {
  from = google_iam_workload_identity_pool_provider.github
  to   = module.archive.google_iam_workload_identity_pool_provider.github
}

moved {
  from = google_service_account.deploy
  to   = module.archive.google_service_account.deploy
}

moved {
  from = google_service_account_iam_member.deploy
  to   = module.archive.google_service_account_iam_member.deploy
}

moved {
  from = google_cloud_run_v2_service_iam_member.deploy
  to   = module.archive.google_cloud_run_v2_service_iam_member.deploy
}

moved {
  from = google_artifact_registry_repository_iam_member.deploy
  to   = module.archive.google_artifact_registry_repository_iam_member.deploy
}

moved {
  from = google_cloud_run_v2_job_iam_member.deploy
  to   = module.archive.google_cloud_run_v2_job_iam_member.deploy
}

moved {
  from = google_service_account_iam_member.deploy_runs_as
  to   = module.archive.google_service_account_iam_member.deploy_runs_as
}

moved {
  from = github_actions_variable.deploy
  to   = module.archive.github_actions_variable.deploy
}
