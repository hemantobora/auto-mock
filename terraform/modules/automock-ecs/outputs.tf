# terraform/modules/automock-ecs/outputs.tf
# Output Values for AutoMock ECS Module

output "mockserver_url" {
  description = "URL to access the MockServer API"
  value       = var.custom_domain != "" ? "https://${var.custom_domain}" : "http://${aws_lb.main.dns_name}"
}

output "dashboard_url" {
  description = "URL to access the MockServer Dashboard"
  value       = var.custom_domain != "" ? "https://${var.custom_domain}:8443/mockserver/dashboard" : "http://${aws_lb.main.dns_name}:8080/mockserver/dashboard"
}

output "alb_dns_name" {
  description = "DNS name of the Application Load Balancer"
  value       = aws_lb.main.dns_name
}

output "alb_zone_id" {
  description = "Zone ID of the Application Load Balancer"
  value       = aws_lb.main.zone_id
}

output "cluster_name" {
  description = "Name of the ECS cluster"
  value       = aws_ecs_cluster.main.name
}

output "cluster_arn" {
  description = "ARN of the ECS cluster"
  value       = aws_ecs_cluster.main.arn
}

output "service_name" {
  description = "Name of the ECS service"
  value       = aws_ecs_service.mockserver.name
}

output "service_arn" {
  description = "ARN of the ECS service"
  value       = aws_ecs_service.mockserver.id
}

output "task_definition_arn" {
  description = "ARN of the ECS task definition"
  value       = aws_ecs_task_definition.mockserver.arn
}

output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "IDs of the public subnets"
  value       = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  description = "IDs of the private subnets"
  value       = aws_subnet.private[*].id
}

output "alb_security_group_id" {
  description = "ID of the ALB security group"
  value       = aws_security_group.alb.id
}

output "ecs_security_group_id" {
  description = "ID of the ECS tasks security group"
  value       = aws_security_group.ecs_tasks.id
}

output "config_bucket" {
  description = "S3 bucket name for configuration"
  value       = local.s3_config.bucket_name
}

output "ttl_expiry" {
  description = "TTL expiry timestamp (if enabled)"
  value       = var.ttl_hours > 0 && var.enable_ttl_cleanup ? timeadd(timestamp(), "${var.ttl_hours}h") : "N/A"
}

output "region" {
  description = "AWS region"
  value       = var.region
}

output "project_name" {
  description = "Project name"
  value       = var.project_name
}

output "environment" {
  description = "Environment"
  value       = var.environment
}

output "infrastructure_summary" {
  description = "Complete infrastructure summary"
  value = {
    project     = var.project_name
    environment = var.environment
    region      = var.region
    endpoints = {
      api       = var.custom_domain != "" ? "https://${var.custom_domain}" : "http://${aws_lb.main.dns_name}"
      dashboard = var.custom_domain != "" ? "https://${var.custom_domain}:8443/mockserver/dashboard" : "http://${aws_lb.main.dns_name}:8080/mockserver/dashboard"
    }
    compute = {
      cluster        = aws_ecs_cluster.main.name
      service        = aws_ecs_service.mockserver.name
      instance_size  = var.instance_size
      min_tasks      = var.min_tasks
      max_tasks      = var.max_tasks
      current_tasks  = var.min_tasks
    }
    storage = {
      config_bucket = local.s3_config.bucket_name
    }
    ttl = {
      enabled    = var.enable_ttl_cleanup
      hours      = var.ttl_hours
      expiry     = var.ttl_hours > 0 && var.enable_ttl_cleanup ? timeadd(timestamp(), "${var.ttl_hours}h") : "N/A"
    }
  }
}

output "cli_integration_commands" {
  description = "CLI commands for integration and management"
  value = {
    health_check       = "curl ${var.custom_domain != "" ? "https://${var.custom_domain}" : "http://${aws_lb.main.dns_name}"}/mockserver/status"
    list_expectations  = "curl ${var.custom_domain != "" ? "https://${var.custom_domain}" : "http://${aws_lb.main.dns_name}"}/mockserver/expectation"
    clear_expectations = "curl -X PUT ${var.custom_domain != "" ? "https://${var.custom_domain}" : "http://${aws_lb.main.dns_name}"}/mockserver/clear"
    view_logs          = "aws logs tail /ecs/automock/${var.project_name}/mockserver --follow --region ${var.region}"
    scale_service      = "aws ecs update-service --cluster ${aws_ecs_cluster.main.name} --service ${aws_ecs_service.mockserver.name} --desired-count <COUNT> --region ${var.region}"
  }
}

output "integration_summary" {
  description = "Integration details for S3 bucket and configuration"
  value = {
    s3_bucket          = local.s3_config.bucket_name
    expectations_path  = local.s3_config.expectations_path
    metadata_path      = local.s3_config.metadata_path
    versions_prefix    = local.s3_config.versions_prefix
    upload_command     = "aws s3 cp expectations.json s3://${local.s3_config.bucket_name}/${local.s3_config.expectations_path}"
    download_command   = "aws s3 cp s3://${local.s3_config.bucket_name}/${local.s3_config.expectations_path} expectations.json"
  }
}
