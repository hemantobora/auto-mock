# terraform/main.tf
# Root Terraform Configuration for AutoMock

terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.5"
    }
  }

  # Backend configuration will be generated dynamically by Go CLI
  # backend "s3" {}
}

# AWS Provider
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      ManagedBy = "AutoMock-Terraform"
      Project   = "AutoMock"
    }
  }
}

# Local variables
locals {
  name_prefix = "automock-${var.project_name}-${var.environment}"
  
  common_tags = {
    Project     = "AutoMock"
    ProjectName = var.project_name
    Environment = var.environment
    Region      = var.aws_region
    CreatedAt   = timestamp()
  }
}

# Data sources
data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# State Backend Module (creates S3 + DynamoDB for Terraform state)
module "state_backend" {
  source = "./modules/state-backend"

  count = var.create_state_backend ? 1 : 0

  region = var.aws_region
  tags   = local.common_tags
}

# S3 Configuration Bucket Module
module "s3_config" {
  source = "./modules/automock-s3"

  project_name = var.project_name
  environment  = var.environment
  region       = var.aws_region
  ttl_hours    = var.ttl_hours

  tags = local.common_tags
}

# ECS Infrastructure Module (VPC, ALB, ECS, Auto-Scaling, TTL)
module "ecs_infrastructure" {
  source = "./modules/automock-ecs"

  project_name  = var.project_name
  environment   = var.environment
  region        = var.aws_region
  instance_size = var.instance_size
  min_tasks     = var.min_tasks
  max_tasks     = var.max_tasks

  # TTL Configuration
  ttl_hours         = var.ttl_hours
  enable_ttl_cleanup = var.enable_ttl_cleanup
  notification_email = var.notification_email

  # Custom Domain Configuration (optional)
  custom_domain    = var.custom_domain
  hosted_zone_id   = var.hosted_zone_id

  # S3 Configuration from module output
  config_bucket_name       = module.s3_config.bucket_name
  config_bucket_arn        = module.s3_config.bucket_arn
  s3_bucket_configuration  = {
    bucket_name       = module.s3_config.bucket_name
    expectations_path = module.s3_config.expectations_key
    metadata_path     = module.s3_config.metadata_key
    versions_prefix   = module.s3_config.versions_prefix
  }

  tags = local.common_tags

  depends_on = [module.s3_config]
}
