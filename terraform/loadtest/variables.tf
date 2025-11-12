variable "project_name" {
  type        = string
  description = "Project name (base for resource names)"
}

variable "aws_region" {
  type        = string
  description = "AWS region"
}

variable "existing_bucket_name" {
  type        = string
  description = "Existing S3 bucket for backend/state and artifacts"
}

variable "cloud_provider" {
  type        = string
  description = "Cloud provider (aws)"
  default     = "aws"
}

variable "cpu_units" {
  type        = number
  description = "CPU units for tasks"
  default     = 256
}

variable "memory_units" {
  type        = number
  description = "Memory units (MB) for tasks"
  default     = 512
}

variable "worker_desired_count" {
  type        = number
  description = "Desired number of worker tasks"
  default     = 0
}

variable "locust_container_image" {
  type        = string
  description = "Container image to run in ECS tasks (placeholder demo by default)"
  # Default to official Locust image to avoid custom builds for easy adoption
  default     = "locustio/locust:2.31.2"
}

variable "init_container_image" {
  type        = string
  description = "Image for the init sidecar that fetches the active bundle (should have Python/pip)."
  # Use public Python image so no custom build is required by default
  default     = "python:3.11-slim"
}

variable "master_port" {
  type        = number
  description = "Port exposed by master UI container"
  default     = 80
}

variable "log_retention_days" {
  type        = number
  description = "CloudWatch log retention for ECS containers"
  default     = 7
}

# BYO networking toggles (align with mockserver root variables)
variable "use_existing_vpc" {
  type        = bool
  description = "If true, use an existing VPC instead of creating a new one."
  default     = false
}

variable "vpc_id" {
  type        = string
  description = "Existing VPC ID when use_existing_vpc = true."
  default     = ""
}

variable "use_existing_subnets" {
  type        = bool
  description = "If true, use existing subnets instead of creating new ones."
  default     = false
}

variable "public_subnet_ids" {
  type        = list(string)
  description = "Existing public subnet IDs when use_existing_subnets = true."
  default     = []
}

variable "use_existing_igw" {
  type        = bool
  description = "If true, use an existing Internet Gateway (skip creating one)."
  default     = false
}

variable "internet_gateway_id" {
  type        = string
  description = "Existing Internet Gateway ID when use_existing_igw = true."
  default     = ""
}

# Arbitrary environment variables to inject into Locust ECS task containers.
variable "extra_environment" {
  type        = map(string)
  description = "Map of KEY => VALUE environment variables added to both master and worker containers. Values are stored in task definition (not encrypted). Avoid putting long-lived secrets here."
  default     = {}
}

# BYO IAM roles for load test stack
variable "use_existing_iam_roles" {
  type        = bool
  description = "If true, use provided IAM role ARNs for ECS tasks instead of creating them."
  default     = false
}

variable "execution_role_arn" {
  type        = string
  description = "Existing ECS task execution role ARN when use_existing_iam_roles = true"
  default     = ""
}

variable "task_role_arn" {
  type        = string
  description = "Existing ECS task role ARN when use_existing_iam_roles = true"
  default     = ""
}

variable "use_existing_security_groups" {
  type        = bool
  description = "If true, use existing ALB & ECS security groups instead of creating them."
  default     = false
}

variable "alb_security_group_id" {
  type        = string
  description = "Existing ALB security group ID when use_existing_security_groups = true"
  default     = ""
}

variable "ecs_security_group_id" {
  type        = string
  description = "Existing ECS tasks security group ID when use_existing_security_groups = true"
  default     = ""
}
