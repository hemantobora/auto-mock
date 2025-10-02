# terraform/outputs.tf
# Root Terraform Outputs

output "mockserver_url" {
  description = "URL to access the MockServer API"
  value       = module.ecs_infrastructure.mockserver_url
}

output "dashboard_url" {
  description = "URL to access the MockServer Dashboard"
  value       = module.ecs_infrastructure.dashboard_url
}

output "config_bucket" {
  description = "S3 bucket name for configuration"
  value       = module.s3_config.bucket_name
}

output "infrastructure_summary" {
  description = "Complete infrastructure summary"
  value       = module.ecs_infrastructure.infrastructure_summary
}

output "cli_integration_commands" {
  description = "CLI commands for integration and management"
  value       = module.ecs_infrastructure.cli_integration_commands
}

output "integration_summary" {
  description = "Integration details for S3 bucket and configuration"
  value       = module.ecs_infrastructure.integration_summary
}

output "cluster_name" {
  description = "ECS cluster name"
  value       = module.ecs_infrastructure.cluster_name
}

output "service_name" {
  description = "ECS service name"
  value       = module.ecs_infrastructure.service_name
}

output "alb_dns_name" {
  description = "ALB DNS name"
  value       = module.ecs_infrastructure.alb_dns_name
}

output "vpc_id" {
  description = "VPC ID"
  value       = module.ecs_infrastructure.vpc_id
}

output "region" {
  description = "AWS region"
  value       = var.aws_region
}

output "project_name" {
  description = "Project name"
  value       = var.project_name
}

output "ttl_expiry" {
  description = "TTL expiry timestamp"
  value       = module.ecs_infrastructure.ttl_expiry
}
