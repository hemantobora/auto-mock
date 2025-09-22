# terraform/modules/automock-ecs/variables.tf
# Variables for AutoMock ECS Fargate Infrastructure Module

variable "project_name" {
  description = "AutoMock project name"
  type        = string
  validation {
    condition     = can(regex("^[a-zA-Z0-9-]+$", var.project_name))
    error_message = "Project name must contain only alphanumeric characters and hyphens."
  }
}

variable "environment" {
  description = "Environment (dev, staging, prod)"
  type        = string
  default     = "dev"
  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "Environment must be one of: dev, staging, prod."
  }
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "ttl_hours" {
  description = "Infrastructure TTL in hours (0 = no TTL)"
  type        = number
  default     = 4
  validation {
    condition     = var.ttl_hours >= 0 && var.ttl_hours <= 168
    error_message = "TTL hours must be between 0 and 168 (1 week)."
  }
}

variable "custom_domain" {
  description = "Custom domain for the API (optional)"
  type        = string
  default     = ""
  validation {
    condition = var.custom_domain == "" || can(regex("^[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]?\\.[a-zA-Z]{2,}$", var.custom_domain))
    error_message = "Custom domain must be a valid domain name or empty string."
  }
}

variable "hosted_zone_id" {
  description = "Route53 hosted zone ID for custom domain"
  type        = string
  default     = ""
  validation {
    condition = var.hosted_zone_id == "" || can(regex("^Z[A-Z0-9]+$", var.hosted_zone_id))
    error_message = "Hosted zone ID must be a valid Route53 zone ID or empty string."
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

variable "enable_ttl_cleanup" {
  description = "Enable automatic infrastructure cleanup"
  type        = bool
  default     = true
}

variable "notification_email" {
  description = "Email for TTL notifications"
  type        = string
  default     = ""
  validation {
    condition = var.notification_email == "" || can(regex("^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$", var.notification_email))
    error_message = "Notification email must be a valid email address or empty string."
  }
}

variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.0.0.0/16"
  validation {
    condition     = can(cidrhost(var.vpc_cidr, 0))
    error_message = "VPC CIDR must be a valid CIDR block."
  }
}

variable "availability_zones" {
  description = "Number of availability zones to use"
  type        = number
  default     = 2
  validation {
    condition     = var.availability_zones >= 2 && var.availability_zones <= 3
    error_message = "Must use between 2 and 3 availability zones."
  }
}

variable "enable_nat_gateway" {
  description = "Enable NAT Gateway for private subnets"
  type        = bool
  default     = true
}

variable "mockserver_image" {
  description = "MockServer Docker image"
  type        = string
  default     = "mockserver/mockserver:5.15.0"
}

variable "log_retention_days" {
  description = "CloudWatch log retention in days"
  type        = number
  default     = 7
  validation {
    condition = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653], var.log_retention_days)
    error_message = "Log retention days must be a valid CloudWatch retention period."
  }
}

variable "enable_container_insights" {
  description = "Enable CloudWatch Container Insights"
  type        = bool
  default     = true
}

variable "auto_scaling_min_capacity" {
  description = "Minimum number of ECS tasks"
  type        = number
  default     = 1
  validation {
    condition     = var.auto_scaling_min_capacity >= 1 && var.auto_scaling_min_capacity <= 10
    error_message = "Minimum capacity must be between 1 and 10."
  }
}

variable "auto_scaling_max_capacity" {
  description = "Maximum number of ECS tasks"
  type        = number
  default     = 3
  validation {
    condition     = var.auto_scaling_max_capacity >= 1 && var.auto_scaling_max_capacity <= 20
    error_message = "Maximum capacity must be between 1 and 20."
  }
}

variable "cpu_utilization_threshold" {
  description = "CPU utilization threshold for auto scaling"
  type        = number
  default     = 70
  validation {
    condition     = var.cpu_utilization_threshold >= 10 && var.cpu_utilization_threshold <= 90
    error_message = "CPU utilization threshold must be between 10 and 90."
  }
}

variable "memory_utilization_threshold" {
  description = "Memory utilization threshold for auto scaling"
  type        = number
  default     = 80
  validation {
    condition     = var.memory_utilization_threshold >= 10 && var.memory_utilization_threshold <= 90
    error_message = "Memory utilization threshold must be between 10 and 90."
  }
}

variable "health_check_grace_period" {
  description = "Health check grace period in seconds"
  type        = number
  default     = 60
  validation {
    condition     = var.health_check_grace_period >= 30 && var.health_check_grace_period <= 300
    error_message = "Health check grace period must be between 30 and 300 seconds."
  }
}

variable "ssl_policy" {
  description = "SSL policy for HTTPS listener"
  type        = string
  default     = "ELBSecurityPolicy-TLS-1-2-2017-01"
  validation {
    condition = contains([
      "ELBSecurityPolicy-TLS-1-2-2017-01",
      "ELBSecurityPolicy-TLS-1-2-Ext-2018-06",
      "ELBSecurityPolicy-FS-1-2-Res-2019-08",
      "ELBSecurityPolicy-FS-1-2-Res-2020-10"
    ], var.ssl_policy)
    error_message = "SSL policy must be a valid ALB SSL policy."
  }
}

variable "additional_tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}

variable "enable_deletion_protection" {
  description = "Enable deletion protection for ALB"
  type        = bool
  default     = false
}

variable "enable_cross_zone_load_balancing" {
  description = "Enable cross-zone load balancing"
  type        = bool
  default     = true
}

variable "enable_http2" {
  description = "Enable HTTP/2 on ALB"
  type        = bool
  default     = true
}

variable "idle_timeout" {
  description = "ALB idle timeout in seconds"
  type        = number
  default     = 60
  validation {
    condition     = var.idle_timeout >= 1 && var.idle_timeout <= 4000
    error_message = "Idle timeout must be between 1 and 4000 seconds."
  }
}

variable "enable_waf" {
  description = "Enable AWS WAF for ALB"
  type        = bool
  default     = false
}

variable "backup_retention_days" {
  description = "S3 backup retention in days"
  type        = number
  default     = 30
  validation {
    condition     = var.backup_retention_days >= 1 && var.backup_retention_days <= 365
    error_message = "Backup retention must be between 1 and 365 days."
  }
}

# S3 Integration Variables
variable "config_bucket_name" {
  description = "Name of the S3 bucket for configuration storage (from S3 module)"
  type        = string
  default     = ""
}

variable "config_bucket_arn" {
  description = "ARN of the S3 bucket for configuration storage (from S3 module)"
  type        = string
  default     = ""
}

variable "s3_bucket_configuration" {
  description = "Complete S3 bucket configuration from the S3 module"
  type        = any
  default     = null
}