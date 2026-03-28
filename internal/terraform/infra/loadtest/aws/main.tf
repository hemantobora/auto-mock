terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# Root wraps a versioned module so it can later be replaced with a remote source
module "aws_locust" {
  # source = "./modules/locust-aws"
  source = "git::https://github.com/hemantobora/automock-terraform.git//modules/locust-aws"

  project_name         = var.project_name
  aws_region           = var.aws_region
  existing_bucket_name = var.existing_bucket_name
  # Sizing: master defaults to 1 vCPU/2GB, workers default to cpu_units/memory_units
  master_cpu_units     = var.master_cpu_units
  master_memory_units  = var.master_memory_units
  worker_cpu_units     = var.cpu_units
  worker_memory_units  = var.memory_units
  worker_desired_count = var.worker_desired_count
  master_port          = var.master_port
  log_retention_days   = var.log_retention_days
  locust_container_image = var.locust_container_image
  init_container_image   = var.init_container_image

  # BYO networking passthrough
  use_existing_vpc       = var.use_existing_vpc
  vpc_id                 = var.vpc_id
  use_existing_subnets   = var.use_existing_subnets
  public_subnet_ids      = var.public_subnet_ids
  use_existing_igw       = var.use_existing_igw
  internet_gateway_id    = var.internet_gateway_id
  extra_environment      = var.extra_environment
  use_existing_iam_roles = var.use_existing_iam_roles
  execution_role_arn     = var.execution_role_arn
  task_role_arn          = var.task_role_arn
  use_existing_security_groups = var.use_existing_security_groups
  alb_security_group_id        = var.alb_security_group_id
  ecs_security_group_id        = var.ecs_security_group_id
}
