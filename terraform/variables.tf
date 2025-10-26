##############################################
# AutoMock Root Variables (AWS)
# Descriptions included for clarity
##############################################

variable "create_state_backend" {
  description = "Create S3 and DynamoDB for Terraform state backend"
  type        = bool
  default     = false
}

# ───────── General ─────────
variable "project_name" {
  description = "Unique name of the AutoMock project; used for tagging and naming AWS resources."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.project_name))
    error_message = "Project name must contain only lowercase letters, numbers, and hyphens."
  }
}

variable "aws_region" {
  description = "AWS region in which to deploy infrastructure (e.g., us-east-1)."
  type        = string
  default     = "us-east-1"
}

variable "instance_size" {
  description = "Container instance size (e.g., small, medium, large) that maps to CPU/memory settings."
  type        = string
  default     = "small"

  validation {
    condition     = contains(["small", "medium", "large", "xlarge"], var.instance_size)
    error_message = "Instance size must be one of: small, medium, large, xlarge."
  }
}

variable "existing_bucket_name" {
  description = "Name of the S3 bucket that stores mock configuration and Terraform state."
  type        = string

  validation {
    condition     = can(regex("^auto-mock-.+", var.existing_bucket_name))
    error_message = "Bucket name must start with 'auto-mock-'."
  }  
}

# ───────── Compute Resources ─────────
variable "cpu_units" {
  description = "Number of CPU units allocated to the ECS task (1024 = 1 vCPU)."
  type        = number
  default     = 256
}

variable "memory_units" {
  description = "Amount of memory (in MiB) allocated to the ECS task."
  type        = number
  default     = 512
}

variable "min_tasks" {
  description = "Minimum number of ECS tasks to keep running in the service."
  type        = number
  default     = 10

  validation {
    condition     = var.min_tasks >= 1 && var.min_tasks <= 200
    error_message = "Minimum tasks must be between 1 and 200."
  }
}

variable "max_tasks" {
  description = "Maximum number of ECS tasks allowed for auto-scaling."
  type        = number
  default     = 200

  validation {
    condition     = var.max_tasks >= 1 && var.max_tasks <= 200
    error_message = "Maximum tasks must be between 1 and 200."
  }
}

# ───────── Networking (BYO or Create) ─────────
variable "use_existing_vpc" {
  description = "If true, use an existing VPC instead of creating a new one."
  type        = bool
  default     = false
}

variable "vpc_id" {
  description = "Existing VPC ID (e.g., vpc-xxxx) if using an existing VPC."
  type        = string
  default     = ""
}

variable "use_existing_subnets" {
  description = "If true, use existing subnets instead of creating new ones."
  type        = bool
  default     = false
}

variable "public_subnet_ids" {
  description = "List of existing public subnet IDs for ALB and NAT Gateway."
  type        = list(string)
  default     = []
}

variable "private_subnet_ids" {
  description = "List of existing private subnet IDs for ECS tasks."
  type        = list(string)
  default     = []
}

variable "use_existing_igw" {
  description = "If true, use an existing Internet Gateway attached to the VPC."
  type        = bool
  default     = false
}

variable "internet_gateway_id" {
  description = "ID of an existing Internet Gateway (e.g., igw-xxxx) if use_existing_igw = true."
  type        = string
  default     = ""
}

variable "use_existing_nat" {
  description = "If true, use existing NAT Gateway(s) for private subnet egress."
  type        = bool
  default     = false
}

variable "nat_gateway_ids" {
  description = "List of existing NAT Gateway IDs (e.g., nat-xxxx) if use_existing_nat = true."
  type        = list(string)
  default     = []
}

variable "use_existing_security_groups" {
  description = "If true, use existing security groups instead of creating new ones."
  type        = bool
  default     = false
}

variable "security_group_ids" {
  description = "List of security group IDs to use when use_existing_security_groups = true (index 0 = ALB, 1 = ECS)."
  type        = list(string)
  default     = []
}

# ───────── IAM Roles ─────────
variable "use_existing_iam_roles" {
  description = "If true, use pre-created IAM roles for ECS execution and task instead of creating new ones."
  type        = bool
  default     = false
}

variable "execution_role_arn" {
  description = "ARN of the existing ECS execution role (needed when use_existing_iam_roles = true)."
  type        = string
  default     = ""
}

variable "task_role_arn" {
  description = "ARN of the existing ECS task role (needed when use_existing_iam_roles = true)."
  type        = string
  default     = ""
}