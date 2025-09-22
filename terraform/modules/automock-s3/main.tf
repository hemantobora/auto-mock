# terraform/modules/automock-s3/main.tf
# AutoMock S3 Configuration Storage Module

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  required_version = ">= 1.0"
}

# Variables
variable "project_name" {
  description = "AutoMock project name"
  type        = string
}

variable "environment" {
  description = "Environment (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "ttl_hours" {
  description = "Infrastructure TTL in hours (affects lifecycle policies)"
  type        = number
  default     = 4
}

variable "enable_versioning" {
  description = "Enable S3 versioning for expectation history"
  type        = bool
  default     = true
}

variable "enable_notifications" {
  description = "Enable S3 event notifications for config changes"
  type        = bool
  default     = true
}

variable "ecs_cluster_arn" {
  description = "ECS cluster ARN for triggering reloads"
  type        = string
  default     = ""
}

variable "ecs_service_name" {
  description = "ECS service name for triggering reloads"
  type        = string
  default     = ""
}

# Local values
locals {
  name_prefix = "automock-${var.project_name}-${var.environment}"
  
  common_tags = {
    Project     = "AutoMock"
    ProjectName = var.project_name
    Environment = var.environment
    ManagedBy   = "Terraform"
    CreatedAt   = timestamp()
    Region      = var.region
  }
  
  ttl_tags = var.ttl_hours > 0 ? {
    TTL        = var.ttl_hours
    TTLExpiry  = timeadd(timestamp(), "${var.ttl_hours}h")
    AutoDelete = "true"
  } : {}
}

# Random suffix for unique naming
resource "random_id" "suffix" {
  byte_length = 4
}

# Data sources
data "aws_caller_identity" "current" {}

# S3 Bucket for AutoMock configuration and metadata
resource "aws_s3_bucket" "config" {
  bucket        = "${local.name_prefix}-config-${random_id.suffix.hex}"
  force_destroy = true

  tags = merge(local.common_tags, local.ttl_tags, {
    Purpose = "AutoMock Configuration Storage"
    ConfigType = "MockServer"
  })
}

# S3 Bucket Versioning - Essential for expectation history
resource "aws_s3_bucket_versioning" "config" {
  bucket = aws_s3_bucket.config.id
  
  versioning_configuration {
    status = var.enable_versioning ? "Enabled" : "Suspended"
  }
}

# S3 Bucket Encryption
resource "aws_s3_bucket_server_side_encryption_configuration" "config" {
  bucket = aws_s3_bucket.config.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
    bucket_key_enabled = true
  }
}

# S3 Bucket Public Access Block - Security best practice
resource "aws_s3_bucket_public_access_block" "config" {
  bucket = aws_s3_bucket.config.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# S3 Bucket Lifecycle Configuration
resource "aws_s3_bucket_lifecycle_configuration" "config" {
  depends_on = [aws_s3_bucket_versioning.config]
  bucket     = aws_s3_bucket.config.id

  rule {
    id     = "expectation_versions_cleanup"
    status = "Enabled"

    # Keep current version indefinitely during TTL period
    filter {
      prefix = "expectations"
    }

    # Cleanup old versions of expectations
    noncurrent_version_expiration {
      noncurrent_days = var.ttl_hours > 0 ? max(var.ttl_hours / 24, 7) : 30
    }
  }

  rule {
    id     = "temp_files_cleanup"
    status = "Enabled"

    # Clean up temporary files quickly
    filter {
      prefix = "temp/"
    }

    expiration {
      days = 1
    }
  }

  # TTL-based bucket cleanup
  dynamic "rule" {
    for_each = var.ttl_hours > 0 ? [1] : []
    content {
      id     = "ttl_cleanup"
      status = "Enabled"

      # Delete all objects when TTL expires
      expiration {
        days = max(var.ttl_hours / 24, 1)
      }

      noncurrent_version_expiration {
        noncurrent_days = max(var.ttl_hours / 24, 1)
      }
    }
  }
}

# S3 Bucket Notification for config changes
resource "aws_s3_bucket_notification" "config_changes" {
  count  = var.enable_notifications ? 1 : 0
  bucket = aws_s3_bucket.config.id

  # Notify on expectations.json changes
  lambda_function {
    lambda_function_arn = aws_lambda_function.config_reload[0].arn
    events              = ["s3:ObjectCreated:*", "s3:ObjectRemoved:*"]
    filter_prefix       = "expectations.json"
  }

  depends_on = [aws_lambda_permission.s3_invoke[0]]
}

# Lambda function for ECS service reload on config changes
resource "aws_lambda_function" "config_reload" {
  count = var.enable_notifications ? 1 : 0

  filename         = data.archive_file.config_reload_zip[0].output_path
  function_name    = "${local.name_prefix}-config-reload"
  role            = aws_iam_role.config_reload[0].arn
  handler         = "index.handler"
  runtime         = "python3.9"
  timeout         = 60
  source_code_hash = data.archive_file.config_reload_zip[0].output_base64sha256

  environment {
    variables = {
      ECS_CLUSTER_ARN  = var.ecs_cluster_arn
      ECS_SERVICE_NAME = var.ecs_service_name
      PROJECT_NAME     = var.project_name
      ENVIRONMENT      = var.environment
    }
  }

  tags = merge(local.common_tags, local.ttl_tags, {
    Purpose = "Config Reload Trigger"
  })
}

# Create config reload Lambda zip file
data "archive_file" "config_reload_zip" {
  count = var.enable_notifications ? 1 : 0

  type        = "zip"
  output_path = "/tmp/config_reload_${local.name_prefix}.zip"
  
  source {
    content = templatefile("${path.module}/scripts/config_reload.py", {
      project_name = var.project_name
      environment  = var.environment
    })
    filename = "index.py"
  }
}

# IAM Role for config reload Lambda
resource "aws_iam_role" "config_reload" {
  count = var.enable_notifications ? 1 : 0
  
  name = "${local.name_prefix}-config-reload-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = merge(local.common_tags, local.ttl_tags)
}

# IAM Policy for config reload Lambda
resource "aws_iam_role_policy" "config_reload" {
  count = var.enable_notifications ? 1 : 0
  
  name = "${local.name_prefix}-config-reload-policy"
  role = aws_iam_role.config_reload[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:${var.region}:${data.aws_caller_identity.current.account_id}:*"
      },
      {
        Effect = "Allow"
        Action = [
          "ecs:UpdateService",
          "ecs:DescribeServices"
        ]
        Resource = [
          var.ecs_cluster_arn,
          "${var.ecs_cluster_arn}/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:GetObjectVersion"
        ]
        Resource = "${aws_s3_bucket.config.arn}/*"
      }
    ]
  })
}

# Lambda permission for S3 to invoke function
resource "aws_lambda_permission" "s3_invoke" {
  count = var.enable_notifications ? 1 : 0

  statement_id  = "AllowS3Invoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.config_reload[0].function_name
  principal     = "s3.amazonaws.com"
  source_arn    = aws_s3_bucket.config.arn
}

# Default project structure in S3
resource "aws_s3_object" "project_metadata" {
  bucket = aws_s3_bucket.config.bucket
  key    = "project-metadata.json"
  
  content = jsonencode({
    project_name    = var.project_name
    environment     = var.environment
    created_at      = timestamp()
    region          = var.region
    ttl_hours       = var.ttl_hours
    ttl_expiry      = var.ttl_hours > 0 ? timeadd(timestamp(), "${var.ttl_hours}h") : null
    bucket_name     = aws_s3_bucket.config.bucket
    versioning      = var.enable_versioning
    notifications   = var.enable_notifications
    infrastructure = {
      type = "ecs-fargate"
      cluster_arn = var.ecs_cluster_arn
      service_name = var.ecs_service_name
    }
  })

  content_type = "application/json"
  
  tags = merge(local.common_tags, {
    ConfigType = "ProjectMetadata"
  })
}

# Default expectations structure (empty, to be populated by CLI)
resource "aws_s3_object" "default_expectations" {
  bucket = aws_s3_bucket.config.bucket
  key    = "expectations.json"
  
  content = jsonencode([
    {
      httpRequest = {
        method = "GET"
        path   = "/health"
      }
      httpResponse = {
        statusCode = 200
        headers = {
          "Content-Type" = "application/json"
          "Access-Control-Allow-Origin" = "*"
        }
        body = {
          status = "healthy"
          service = "automock"
          project = var.project_name
          environment = var.environment
        }
      }
    }
  ])

  content_type = "application/json"
  
  tags = merge(local.common_tags, {
    ConfigType = "MockServerExpectations"
  })
}

# Versions directory structure
resource "aws_s3_object" "versions_readme" {
  bucket = aws_s3_bucket.config.bucket
  key    = "versions/README.md"
  
  content = <<-EOT
# AutoMock Expectation Versions

This directory contains historical versions of your MockServer expectations.

## Version Format
- `expectations-v1.json` - Initial version
- `expectations-v2.json` - Second version
- etc.

## Automatic Cleanup
- Versions older than ${var.ttl_hours > 0 ? "${max(var.ttl_hours / 24, 7)} days" : "30 days"} are automatically deleted
- Current version is always preserved in `/expectations.json`

## CLI Usage
```bash
# View version history
./automock versions --project ${var.project_name}

# Restore previous version
./automock restore --project ${var.project_name} --version v2

# Compare versions
./automock diff --project ${var.project_name} --from v1 --to v2
```
  EOT

  content_type = "text/markdown"
  
  tags = merge(local.common_tags, {
    ConfigType = "Documentation"
  })
}