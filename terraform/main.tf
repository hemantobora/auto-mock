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
  name_prefix = "automock-${var.project_name}"
  
  common_tags = {
    Project     = "AutoMock"
    ProjectName = var.project_name
    Region      = var.aws_region
    CreatedAt   = timestamp()
  }
  
  # S3 configuration using existing bucket
  s3_config = {
    bucket_name       = data.aws_s3_bucket.config.id
    bucket_arn        = data.aws_s3_bucket.config.arn
    expectations_path = "expectations.json"
    metadata_path     = "deployment-metadata.json"
    versions_prefix   = "versions/"
  }
}

# Data sources
data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# Reference existing S3 bucket created by 'automock init'
# DO NOT create a new bucket - use the one that already exists
data "aws_s3_bucket" "config" {
  bucket = var.existing_bucket_name
}

# State Backend Module (creates S3 + DynamoDB for Terraform state)
module "state_backend" {
  source = "./modules/state-backend"

  count = var.create_state_backend ? 1 : 0

  region = var.aws_region
  tags   = local.common_tags
}

# ECS Infrastructure Module (VPC, ALB, ECS, Auto-Scaling)
module "ecs_infrastructure" {
  source = "./modules/automock-ecs"

  project_name  = var.project_name
  region        = var.aws_region
  instance_size = var.instance_size
  min_tasks     = var.min_tasks
  max_tasks     = var.max_tasks

  cleanup_role_arn   = var.cleanup_role_arn

  # Custom Domain Configuration (optional)
  custom_domain  = var.custom_domain
  hosted_zone_id = var.hosted_zone_id

  # S3 Configuration from data source
  config_bucket_name      = local.s3_config.bucket_name
  config_bucket_arn       = local.s3_config.bucket_arn
  s3_bucket_configuration = local.s3_config

  tags = local.common_tags
}