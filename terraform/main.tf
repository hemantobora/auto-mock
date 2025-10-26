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
  count  = (var.cloud_provider == "aws" && var.create_state_backend) ? 1 : 0

  # pin a tag/sha for reproducibility
  source = "git::https://github.com/hemantobora/automock-terraform.git//modules/aws/state-backend"

  region = var.aws_region
  tags   = local.common_tags
}


# ECS Infrastructure Module (VPC, ALB, ECS, Auto-Scaling)
module "ecs_infrastructure" {
  count  = var.cloud_provider == "aws" ? 1 : 0
  source = "git::https://github.com/hemantobora/automock-terraform.git//modules/aws/ecs"

  project_name  = var.project_name
  region        = var.aws_region
  instance_size = var.instance_size
  min_tasks     = var.min_tasks
  max_tasks     = var.max_tasks
  cpu_units     = var.cpu_units
  memory_units  = var.memory_units

  # S3 Configuration from data source
  config_bucket_name      = local.s3_config.bucket_name
  config_bucket_arn       = local.s3_config.bucket_arn
  s3_bucket_configuration = local.s3_config

  # Networking
  use_existing_vpc         = var.use_existing_vpc
  vpc_id                   = var.vpc_id

  use_existing_subnets     = var.use_existing_subnets
  public_subnet_ids        = var.public_subnet_ids
  private_subnet_ids       = var.private_subnet_ids

  use_existing_igw         = var.use_existing_igw
  internet_gateway_id      = var.internet_gateway_id

  use_existing_nat         = var.use_existing_nat
  nat_gateway_ids          = var.nat_gateway_ids

  use_existing_security_groups = var.use_existing_security_groups
  security_group_ids           = var.security_group_ids  # [alb_sg, ecs_sg]

  use_existing_iam_roles        = var.use_existing_iam_roles
  task_execution_role_arn       = var.execution_role_arn
  task_role_arn                 = var.task_role_arn


  tags = local.common_tags
}
