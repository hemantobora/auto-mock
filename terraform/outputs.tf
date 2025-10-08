# terraform/outputs.tf
# Root Terraform Outputs

output "mockserver_url" {
  description = "MockServer API endpoint URL"
  value       = module.ecs_infrastructure.mockserver_url
}

output "dashboard_url" {
  description = "MockServer dashboard URL"
  value       = module.ecs_infrastructure.dashboard_url
}

output "config_bucket" {
  description = "S3 configuration bucket name"
  value       = data.aws_s3_bucket.config.id
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
  description = "Application Load Balancer DNS name"
  value       = module.ecs_infrastructure.alb_dns_name
}

output "vpc_id" {
  description = "VPC ID"
  value       = module.ecs_infrastructure.vpc_id
}

output "ttl_expiry" {
  description = "Infrastructure TTL expiry timestamp"
  value       = var.ttl_hours > 0 ? timeadd(timestamp(), "${var.ttl_hours}h") : null
}

output "integration_summary" {
  description = "Integration summary for CLI"
  value = {
    project_name   = var.project_name
    bucket_name    = data.aws_s3_bucket.config.id
    mockserver_url = module.ecs_infrastructure.mockserver_url
    dashboard_url  = module.ecs_infrastructure.dashboard_url
    region         = var.aws_region
    ttl_hours      = var.ttl_hours
    ttl_expiry     = var.ttl_hours > 0 ? timeadd(timestamp(), "${var.ttl_hours}h") : null
  }
}

output "cli_integration_commands" {
  description = "CLI commands for interacting with the deployed infrastructure"
  value = {
    upload_expectations = "aws s3 cp expectations.json s3://${local.s3_config.bucket_name}/configs/${var.project_name}/current.json"
    view_expectations   = "aws s3 cp s3://${local.s3_config.bucket_name}/configs/${var.project_name}/current.json - | jq ."
    view_logs         = "aws logs tail /ecs/automock/${var.project_name}/mockserver --follow"
    scale_service     = "aws ecs update-service --cluster ${module.ecs_infrastructure.cluster_name} --service ${module.ecs_infrastructure.service_name} --desired-count 20"
  }
}

output "infrastructure_summary" {
  description = "Complete infrastructure summary"
  value = {
    cluster = {
      name = module.ecs_infrastructure.cluster_name
      arn  = module.ecs_infrastructure.cluster_arn
    }
    service = {
      name = module.ecs_infrastructure.service_name
      arn  = module.ecs_infrastructure.service_arn
    }
    load_balancer = {
      dns_name = module.ecs_infrastructure.alb_dns_name
      zone_id  = module.ecs_infrastructure.alb_zone_id
    }
    networking = {
      vpc_id             = module.ecs_infrastructure.vpc_id
      public_subnet_ids  = module.ecs_infrastructure.public_subnet_ids
      private_subnet_ids = module.ecs_infrastructure.private_subnet_ids
    }
    storage = {
      bucket_name       = data.aws_s3_bucket.config.id
      bucket_arn        = data.aws_s3_bucket.config.arn
      expectations_path = "expectations.json"
      metadata_path     = "deployment-metadata.json"
    }
  }
}

output "cost_estimate" {
  description = "Estimated monthly cost breakdown"
  value = {
    note = "Estimates based on us-east-1 pricing"
    components = {
      ecs_fargate_monthly  = "$864.00 (10 tasks, 24/7)"
      alb_monthly          = "$16.00"
      nat_gateway_monthly  = "$64.00 (2 NAT gateways)"
      data_transfer_monthly = "$9.00 (estimated)"
      total_monthly        = "$953.00"
      actual_with_ttl      = var.ttl_hours > 0 ? format("$%.2f", (953.00 / 730.0) * var.ttl_hours) : "N/A (no TTL)"
    }
  }
}