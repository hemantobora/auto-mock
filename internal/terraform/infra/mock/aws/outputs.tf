# terraform/outputs.tf
# Root Terraform Outputs

# Helper locals so we can support both "plain module" and "[0]" forms
locals {
  ecs_mockserver_url = try(module.ecs_infrastructure.mockserver_url, module.ecs_infrastructure[0].mockserver_url, null)
  ecs_dashboard_url  = try(module.ecs_infrastructure.dashboard_url,  module.ecs_infrastructure[0].dashboard_url,  null)
  ecs_cluster_name   = try(module.ecs_infrastructure.cluster_name,   module.ecs_infrastructure[0].cluster_name,   null)
  ecs_service_name   = try(module.ecs_infrastructure.service_name,   module.ecs_infrastructure[0].service_name,   null)

  # If you already have a data "aws_s3_bucket" "config", keep using it:
  cfg_bucket_name = try(data.aws_s3_bucket.config.id,  null)
}

output "mockserver_url" {
  description = "MockServer API endpoint URL"
  value       = local.ecs_mockserver_url
}

output "dashboard_url" {
  description = "MockServer dashboard URL"
  value       = local.ecs_dashboard_url
}

output "config_bucket" {
  description = "S3 configuration bucket name"
  value       = local.cfg_bucket_name
}

output "integration_summary" {
  description = "Integration summary for CLI"
  value = {
    project_name   = var.project_name
    mockserver_url = local.ecs_mockserver_url
    dashboard_url  = local.ecs_dashboard_url
    region         = var.aws_region
  }
}

output "cli_integration_commands" {
  description = "CLI commands for interacting with the deployed infrastructure"
  value = module.ecs_infrastructure[0].cli_integration_commands
}

output "infrastructure_summary" {
  description = "Complete infrastructure summary"
  value = module.ecs_infrastructure[0].infrastructure_summary
}
