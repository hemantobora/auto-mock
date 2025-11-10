variable "project_name" { type = string }
variable "aws_region" { type = string }
variable "cpu_units" { type = number }
variable "memory_units" { type = number }
variable "worker_desired_count" { type = number }
variable "master_port" { type = number }
variable "log_retention_days" { type = number }
variable "locust_container_image" { type = string }
