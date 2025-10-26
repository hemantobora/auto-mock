# terraform/outputs.tf
# Root Terraform Outputs

# Helper locals so we can support both "plain module" and "[0]" forms
locals {
  ecs_mockserver_url = try(module.ecs_infrastructure.mockserver_url, module.ecs_infrastructure[0].mockserver_url, null)
  ecs_dashboard_url  = try(module.ecs_infrastructure.dashboard_url,  module.ecs_infrastructure[0].dashboard_url,  null)
  ecs_cluster_name   = try(module.ecs_infrastructure.cluster_name,   module.ecs_infrastructure[0].cluster_name,   null)
  ecs_service_name   = try(module.ecs_infrastructure.service_name,   module.ecs_infrastructure[0].service_name,   null)
  ecs_alb_dns_name   = try(module.ecs_infrastructure.alb_dns_name,   module.ecs_infrastructure[0].alb_dns_name,   null)
  ecs_alb_zone_id    = try(module.ecs_infrastructure.alb_zone_id,    module.ecs_infrastructure[0].alb_zone_id,    null)
  ecs_vpc_id         = try(module.ecs_infrastructure.vpc_id,         module.ecs_infrastructure[0].vpc_id,         null)
  ecs_pub_subnets    = try(module.ecs_infrastructure.public_subnet_ids,  module.ecs_infrastructure[0].public_subnet_ids,  null)
  ecs_priv_subnets   = try(module.ecs_infrastructure.private_subnet_ids, module.ecs_infrastructure[0].private_subnet_ids, null)

  # If you already have a data "aws_s3_bucket" "config", keep using it:
  cfg_bucket_name = try(data.aws_s3_bucket.config.id,  null)
  cfg_bucket_arn  = try(data.aws_s3_bucket.config.arn, null)
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

output "cluster_name" {
  description = "ECS cluster name"
  value       = local.ecs_cluster_name
}

output "service_name" {
  description = "ECS service name"
  value       = local.ecs_service_name
}

output "alb_dns_name" {
  description = "Application Load Balancer DNS name"
  value       = local.ecs_alb_dns_name
}

output "vpc_id" {
  description = "VPC ID"
  value       = local.ecs_vpc_id
}

output "integration_summary" {
  description = "Integration summary for CLI"
  value = {
    project_name   = var.project_name
    bucket_name    = local.cfg_bucket_name
    mockserver_url = local.ecs_mockserver_url
    dashboard_url  = local.ecs_dashboard_url
    region         = var.aws_region
  }
}

output "cli_integration_commands" {
  description = "CLI commands for interacting with the deployed infrastructure"
  value = {
    upload_expectations = "aws s3 cp expectations.json s3://${local.cfg_bucket_name}/configs/${var.project_name}/current.json"
    view_expectations   = "aws s3 cp s3://${local.cfg_bucket_name}/configs/${var.project_name}/current.json - | jq ."
    view_logs           = "aws logs tail /ecs/automock/${var.project_name}/mockserver --follow"
    scale_service       = "aws ecs update-service --cluster ${local.ecs_cluster_name} --service ${local.ecs_service_name} --desired-count 20"
  }
}

output "infrastructure_summary" {
  description = "Complete infrastructure summary"
  value = {
    cluster = {
      name = local.ecs_cluster_name
      arn  = try(module.ecs_infrastructure.cluster_arn, module.ecs_infrastructure[0].cluster_arn, null)
    }
    service = {
      name = local.ecs_service_name
      arn  = try(module.ecs_infrastructure.service_arn, module.ecs_infrastructure[0].service_arn, null)
    }
    load_balancer = {
      dns_name = local.ecs_alb_dns_name
      zone_id  = local.ecs_alb_zone_id
    }
    networking = {
      vpc_id             = local.ecs_vpc_id
      public_subnet_ids  = local.ecs_pub_subnets
      private_subnet_ids = local.ecs_priv_subnets
    }
    storage = {
      bucket_name       = local.cfg_bucket_name
      bucket_arn        = local.cfg_bucket_arn
      metadata_path     = "deployment-metadata.json"
    }
  }
}
