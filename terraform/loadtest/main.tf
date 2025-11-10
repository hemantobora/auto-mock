terraform {
  required_version = ">= 1.5.0"
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
  source = "./modules/locust-aws"

  project_name         = var.project_name
  aws_region           = var.aws_region
  cpu_units            = var.cpu_units
  memory_units         = var.memory_units
  worker_desired_count = var.worker_desired_count
  master_port          = var.master_port
  log_retention_days   = var.log_retention_days
  locust_container_image = var.locust_container_image
}
