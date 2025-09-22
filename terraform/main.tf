# terraform/main.tf
# Main Terraform configuration for AutoMock ECS Fargate deployment

terraform {
  required_version = ">= 1.0"
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  
  # Uncomment and configure for remote state
  # backend "s3" {
  #   bucket = "your-terraform-state-bucket"
  #   key    = "automock/terraform.tfstate"
  #   region = "us-east-1"
  # }
}

# Configure the AWS Provider
provider "aws" {
  region = var.aws_region
  
  default_tags {
    tags = {
      ManagedBy = "AutoMock-Terraform"
      Project   = "AutoMock"
    }
  }
}

# Variables
variable "aws_region" {
  description = "AWS region for deployment"
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "AutoMock project name"
  type        = string
}

variable "environment" {
  description = "Environment (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "instance_size" {
  description = "ECS task size (small, medium, large, xlarge)"
  type        = string
  default     = "small"
}

variable "ttl_hours" {
  description = "Infrastructure TTL in hours (0 = no TTL)"
  type        = number
  default     = 4
}

variable "custom_domain" {
  description = "Custom domain for the API (optional)"
  type        = string
  default     = ""
}

variable "hosted_zone_id" {
  description = "Route53 hosted zone ID for custom domain"
  type        = string
  default     = ""
}

variable "notification_email" {
  description = "Email for TTL notifications"
  type        = string
  default     = ""
}

variable "enable_ttl_cleanup" {
  description = "Enable automatic infrastructure cleanup"
  type        = bool
  default     = true
}

# Deploy the AutoMock S3 Configuration Storage
module "automock_s3" {
  source = "./modules/automock-s3"
  
  project_name         = var.project_name
  environment          = var.environment
  region               = var.aws_region
  ttl_hours            = var.ttl_hours
  enable_versioning    = true
  enable_notifications = false  # Will enable later when needed
  
  # No ECS integration initially
  ecs_cluster_arn  = ""
  ecs_service_name = ""
}

# Deploy the AutoMock ECS Fargate infrastructure
module "automock_ecs" {
  source = "./modules/automock-ecs"
  
  project_name       = var.project_name
  environment        = var.environment
  region             = var.aws_region
  instance_size      = var.instance_size
  ttl_hours          = var.ttl_hours
  custom_domain      = var.custom_domain
  hosted_zone_id     = var.hosted_zone_id
  notification_email = var.notification_email
  enable_ttl_cleanup = var.enable_ttl_cleanup
  
  # Use S3 bucket from the S3 module
  config_bucket_name = module.automock_s3.bucket_name
  config_bucket_arn  = module.automock_s3.bucket_arn
  
  # S3 integration
  s3_bucket_configuration = module.automock_s3.bucket_configuration
  
  depends_on = [module.automock_s3]
}

# Outputs
output "mockserver_url" {
  description = "URL to access the MockServer API"
  value       = module.automock_ecs.mockserver_url
}

output "dashboard_url" {
  description = "URL to access the MockServer dashboard"
  value       = module.automock_ecs.dashboard_url
}

output "deployment_info" {
  description = "Complete deployment information"
  value       = module.automock_ecs.deployment_info
  sensitive   = false
}

output "config_bucket" {
  description = "S3 bucket for configuration storage"
  value       = module.automock_s3.bucket_name
}

output "s3_configuration" {
  description = "Complete S3 bucket configuration"
  value       = module.automock_s3.bucket_configuration
  sensitive   = false
}

output "infrastructure_summary" {
  description = "Infrastructure deployment summary"
  value = {
    project_name      = var.project_name
    environment       = var.environment
    region           = var.aws_region
    mockserver_url   = module.automock_ecs.mockserver_url
    dashboard_url    = module.automock_ecs.dashboard_url
    ttl_enabled      = var.enable_ttl_cleanup && var.ttl_hours > 0
    ttl_expiry       = module.automock_ecs.ttl_expiry_time
    domain_type      = var.custom_domain != "" ? "custom" : "auto-generated"
    
    management_commands = {
      view_logs    = "aws logs tail /ecs/${var.project_name}-${var.environment}/mockserver --follow"
      scale_up     = "aws ecs update-service --cluster ${module.automock_ecs.ecs_cluster_name} --service ${module.automock_ecs.ecs_service_name} --desired-count 3"
      scale_down   = "aws ecs update-service --cluster ${module.automock_ecs.ecs_cluster_name} --service ${module.automock_ecs.ecs_service_name} --desired-count 1"
      destroy      = "terraform destroy -auto-approve"
    }
  }
}

output "integration_summary" {
  description = "Summary of S3-ECS integration"
  value = {
    s3_bucket           = module.automock_s3.bucket_name
    s3_bucket_arn      = module.automock_s3.bucket_arn
    ecs_cluster_arn    = module.automock_ecs.ecs_cluster_arn
    ecs_service_name   = module.automock_ecs.ecs_service_name
    
    # Configuration paths
    expectations_path  = "s3://${module.automock_s3.bucket_name}/expectations.json"
    metadata_path     = "s3://${module.automock_s3.bucket_name}/project-metadata.json"
    versions_path     = "s3://${module.automock_s3.bucket_name}/versions/"
    
    # Integration status
    s3_notifications_enabled = false
    ecs_s3_permissions      = "Configured"
    config_reload_method    = "Manual or CLI-triggered"
    
    # Next steps
    next_steps = [
      "Use CLI to upload expectations.json to S3",
      "ECS tasks will automatically read from S3 on startup",
      "Use 'terraform output cli_integration_commands' for S3 management",
      "Consider enabling S3 notifications for automatic reloads"
    ]
  }
}

output "cli_integration_commands" {
  description = "CLI commands for S3-ECS integration"
  value = {
    upload_config   = "aws s3 cp expectations.json s3://${module.automock_s3.bucket_name}/expectations.json"
    download_config = "aws s3 cp s3://${module.automock_s3.bucket_name}/expectations.json expectations.json"
    reload_service  = "aws ecs update-service --cluster ${module.automock_ecs.ecs_cluster_name} --service ${module.automock_ecs.ecs_service_name} --force-new-deployment"
    view_logs      = "aws logs tail /ecs/${var.project_name}-${var.environment}/mockserver --follow"
    service_status = "aws ecs describe-services --cluster ${module.automock_ecs.ecs_cluster_name} --services ${module.automock_ecs.ecs_service_name}"
  }
}
