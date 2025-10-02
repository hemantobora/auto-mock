# terraform/modules/iam/variables.tf

variable "name_prefix" {
  description = "Prefix for resource names"
  type        = string
}

variable "config_bucket_arn" {
  description = "ARN of the S3 bucket containing configurations"
  type        = string
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default     = {}
}
