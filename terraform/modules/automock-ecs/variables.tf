# terraform/modules/automock-ecs/variables.tf
# Input Variables for AutoMock ECS Module

variable "project_name" {
  description = "AutoMock project name (user-friendly identifier)"
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.project_name))
    error_message = "Project name must contain only lowercase letters, numbers, and hyphens."
  }
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "instance_size" {
  description = "ECS task size (small, medium, large, xlarge)"
  type        = string
  default     = "small"

  validation {
    condition     = contains(["small", "medium", "large", "xlarge"], var.instance_size)
    error_message = "Instance size must be one of: small, medium, large, xlarge."
  }
}

variable "min_tasks" {
  description = "Minimum number of ECS tasks (for load testing, use 10+)"
  type        = number
  default     = 10

  validation {
    condition     = var.min_tasks >= 1 && var.min_tasks <= 200
    error_message = "Minimum tasks must be between 1 and 200."
  }
}

variable "max_tasks" {
  description = "Maximum number of ECS tasks"
  type        = number
  default     = 200

  validation {
    condition     = var.max_tasks >= 1 && var.max_tasks <= 200
    error_message = "Maximum tasks must be between 1 and 200."
  }
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

variable "cleanup_role_arn" {
  description = "IAM role ARN for cleanup Lambda (if user-provided)"
  type        = string
  default     = ""
}

variable "config_bucket_name" {
  description = "S3 bucket name for configuration (from external S3 module)"
  type        = string
  default     = ""
}

variable "config_bucket_arn" {
  description = "S3 bucket ARN for configuration"
  type        = string
  default     = ""
}

variable "s3_bucket_configuration" {
  description = "S3 bucket configuration details"
  type = object({
    bucket_name       = string
    expectations_path = string
    metadata_path     = string
    versions_prefix   = string
  })
  default = null
}

variable "tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}

variable "cpu_units" {
  description = "CPU units for the ECS task definition"
  type        = number
  default     = 256
}

variable "memory_units" {
  description = "Memory (in MiB) for the ECS task definition"
  type        = number
  default     = 512
}

