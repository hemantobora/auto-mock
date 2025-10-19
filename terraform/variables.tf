# terraform/variables.tf
# Root Terraform Variables

variable "project_name" {
  description = "AutoMock project name (user-friendly identifier)"
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.project_name))
    error_message = "Project name must contain only lowercase letters, numbers, and hyphens."
  }
}


variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "existing_bucket_name" {
  description = "Name of existing S3 bucket created by automock init"
  type        = string
  
  validation {
    condition     = can(regex("^auto-mock-.+", var.existing_bucket_name))
    error_message = "Bucket name must start with 'auto-mock-'."
  }
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
  description = "Minimum number of ECS tasks"
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

variable "ttl_hours" {
  description = "Infrastructure TTL in hours (0 = no TTL)"
  type        = number
  default     = 4

  validation {
    condition     = var.ttl_hours >= 0 && var.ttl_hours <= 168
    error_message = "TTL hours must be between 0 and 168 (7 days)."
  }
}

variable "enable_ttl_cleanup" {
  description = "Enable automatic infrastructure cleanup based on TTL"
  type        = bool
  default     = true
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
  description = "Email for TTL expiration notifications"
  type        = string
  default     = ""
}

variable "create_state_backend" {
  description = "Create S3 and DynamoDB for Terraform state backend"
  type        = bool
  default     = false
}

variable "cleanup_role_arn" {
  description = "IAM role ARN for cleanup Lambda (if user-provided)"
  type        = string
  default     = ""
}

variable "create_cleanup_roles" {
  description = "Whether to create IAM roles for cleanup"
  type        = bool
  default     = false
}