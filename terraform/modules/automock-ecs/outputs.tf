# terraform/modules/automock-ecs/outputs.tf
# Outputs for AutoMock ECS Fargate Infrastructure

output "mockserver_url" {
  description = "URL to access the MockServer API"
  value       = var.custom_domain != "" ? "https://${var.custom_domain}" : "https://${aws_lb.main.dns_name}"
}

output "dashboard_url" {
  description = "URL to access the MockServer dashboard"
  value       = var.custom_domain != "" ? "https://${var.custom_domain}/mockserver/dashboard" : "https://${aws_lb.main.dns_name}/mockserver/dashboard"
}

output "load_balancer_dns" {
  description = "DNS name of the Application Load Balancer"
  value       = aws_lb.main.dns_name
}

output "load_balancer_arn" {
  description = "ARN of the Application Load Balancer"
  value       = aws_lb.main.arn
}

output "ecs_cluster_name" {
  description = "Name of the ECS cluster"
  value       = aws_ecs_cluster.main.name
}

output "ecs_cluster_arn" {
  description = "ARN of the ECS cluster"
  value       = aws_ecs_cluster.main.arn
}

output "ecs_service_name" {
  description = "Name of the ECS service"
  value       = aws_ecs_service.mockserver.name
}

output "ecs_service_arn" {
  description = "ARN of the ECS service"
  value       = aws_ecs_service.mockserver.id
}

output "config_bucket_name" {
  description = "Name of the S3 bucket for configuration storage"
  value       = local.config_bucket_name
}

output "config_bucket_arn" {
  description = "ARN of the S3 bucket for configuration storage"
  value       = local.config_bucket_arn
}

output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "private_subnet_ids" {
  description = "IDs of the private subnets"
  value       = aws_subnet.private[*].id
}

output "public_subnet_ids" {
  description = "IDs of the public subnets"
  value       = aws_subnet.public[*].id
}

output "ssl_certificate_arn" {
  description = "ARN of the SSL certificate (if custom domain is used)"
  value       = var.custom_domain != "" ? aws_acm_certificate.main[0].arn : null
}

output "route53_zone_id" {
  description = "Route53 hosted zone ID (if custom domain is used)"
  value       = var.custom_domain != "" ? var.hosted_zone_id : null
}

output "ttl_lambda_function_name" {
  description = "Name of the TTL cleanup Lambda function"
  value       = var.enable_ttl_cleanup && var.ttl_hours > 0 ? aws_lambda_function.ttl_cleanup[0].function_name : null
}

output "ttl_expiry_time" {
  description = "TTL expiry timestamp"
  value       = var.ttl_hours > 0 ? timeadd(timestamp(), "${var.ttl_hours}h") : null
}

output "deployment_info" {
  description = "Comprehensive deployment information"
  value = {
    project_name     = var.project_name
    environment      = var.environment
    region           = var.region
    instance_size    = var.instance_size
    mockserver_url   = var.custom_domain != "" ? "https://${var.custom_domain}" : "https://${aws_lb.main.dns_name}"
    dashboard_url    = var.custom_domain != "" ? "https://${var.custom_domain}/mockserver/dashboard" : "https://${aws_lb.main.dns_name}/mockserver/dashboard"
    
    infrastructure = {
      vpc_id           = aws_vpc.main.id
      cluster_name     = aws_ecs_cluster.main.name
      service_name     = aws_ecs_service.mockserver.name
      load_balancer_arn = aws_lb.main.arn
      config_bucket    = local.config_bucket_name
    }
    
    domain = {
      type            = var.custom_domain != "" ? "custom" : "auto"
      custom_domain   = var.custom_domain
      certificate_arn = var.custom_domain != "" ? aws_acm_certificate.main[0].arn : null
      hosted_zone_id  = var.custom_domain != "" ? var.hosted_zone_id : null
    }
    
    ttl = {
      enabled        = var.enable_ttl_cleanup && var.ttl_hours > 0
      hours          = var.ttl_hours
      expiry_time    = var.ttl_hours > 0 ? timeadd(timestamp(), "${var.ttl_hours}h") : null
      cleanup_lambda = var.enable_ttl_cleanup && var.ttl_hours > 0 ? aws_lambda_function.ttl_cleanup[0].function_name : null
    }
    
    tags = merge(local.common_tags, local.ttl_tags)
  }
}

output "terraform_state_info" {
  description = "Information for managing Terraform state"
  value = {
    module_path = path.module
    workspace   = terraform.workspace
    resources = {
      main_resources = [
        aws_vpc.main.id,
        aws_ecs_cluster.main.arn,
        aws_ecs_service.mockserver.arn,
        aws_lb.main.arn,
        aws_s3_bucket.config_bucket.arn
      ]
      conditional_resources = compact([
        var.custom_domain != "" ? aws_acm_certificate.main[0].arn : "",
        var.enable_ttl_cleanup && var.ttl_hours > 0 ? aws_lambda_function.ttl_cleanup[0].arn : ""
      ])
    }
  }
}