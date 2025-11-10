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
  # Using a tiny demo image by default; replace with a Locust image and bootstrap if needed
  default     = "nginxdemos/hello:plain-text"
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
