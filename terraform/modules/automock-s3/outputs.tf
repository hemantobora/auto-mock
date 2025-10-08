# terraform/modules/automock-s3/outputs.tf
# Outputs for AutoMock S3 Configuration Storage

output "bucket_name" {
  description = "Name of the S3 configuration bucket"
  value       = aws_s3_bucket.config.bucket
}

output "bucket_arn" {
  description = "ARN of the S3 configuration bucket"
  value       = aws_s3_bucket.config.arn
}

output "bucket_domain_name" {
  description = "Domain name of the S3 bucket"
  value       = aws_s3_bucket.config.bucket_domain_name
}

output "bucket_regional_domain_name" {
  description = "Regional domain name of the S3 bucket"
  value       = aws_s3_bucket.config.bucket_regional_domain_name
}

output "expectations_key" {
  description = "S3 key for the current expectations file"
  value       = "expectations.json"
}

output "metadata_key" {
  description = "S3 key for the project metadata file"
  value       = "project-metadata.json"
}

output "versions_prefix" {
  description = "S3 prefix for expectation versions"
  value       = "versions/"
}

output "config_reload_lambda_arn" {
  description = "ARN of the config reload Lambda function"
  value       = var.enable_notifications ? aws_lambda_function.config_reload[0].arn : null
}

output "config_reload_lambda_name" {
  description = "Name of the config reload Lambda function"
  value       = var.enable_notifications ? aws_lambda_function.config_reload[0].function_name : null
}

output "versioning_enabled" {
  description = "Whether S3 versioning is enabled"
  value       = var.enable_versioning
}

output "notifications_enabled" {
  description = "Whether S3 event notifications are enabled"
  value       = var.enable_notifications
}

output "bucket_configuration" {
  description = "Complete S3 bucket configuration for CLI usage"
  value = {
    bucket_name    = aws_s3_bucket.config.bucket
    bucket_arn     = aws_s3_bucket.config.arn
    region         = var.region
    project_name   = var.project_name
    
    # File paths
    expectations_path = "expectations.json"
    metadata_path     = "project-metadata.json"
    versions_prefix   = "versions/"
    
    # Configuration
    versioning_enabled    = var.enable_versioning
    notifications_enabled = var.enable_notifications
    ttl_hours            = var.ttl_hours
    
    # Lambda integration
    reload_lambda_arn  = var.enable_notifications ? aws_lambda_function.config_reload[0].arn : null
    reload_lambda_name = var.enable_notifications ? aws_lambda_function.config_reload[0].function_name : null
    
    # Access patterns for CLI
    cli_operations = {
      read_expectations  = "s3://${aws_s3_bucket.config.bucket}/expectations.json"
      write_expectations = "s3://${aws_s3_bucket.config.bucket}/expectations.json"
      read_metadata      = "s3://${aws_s3_bucket.config.bucket}/project-metadata.json"
      write_metadata     = "s3://${aws_s3_bucket.config.bucket}/project-metadata.json"
      list_versions      = "s3://${aws_s3_bucket.config.bucket}/versions/"
    }
  }
}

output "iam_policy_document" {
  description = "IAM policy document for CLI access to this bucket"
  value = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AutoMockCLIBucketAccess"
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:GetObjectVersion",
          "s3:ListBucket",
          "s3:ListBucketVersions"
        ]
        Resource = [
          aws_s3_bucket.config.arn,
          "${aws_s3_bucket.config.arn}/*"
        ]
      }
    ]
  })
}

output "ecs_environment_variables" {
  description = "Environment variables for ECS task to access S3 config"
  value = {
    AUTOMOCK_CONFIG_BUCKET = aws_s3_bucket.config.bucket
    AUTOMOCK_CONFIG_KEY    = "expectations.json"
    AUTOMOCK_METADATA_KEY  = "project-metadata.json"
    AUTOMOCK_PROJECT_NAME  = var.project_name
    AUTOMOCK_REGION        = var.region
  }
}

output "ecs_task_policy_document" {
  description = "IAM policy document for ECS tasks to read from this bucket"
  value = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AutoMockECSReadAccess"
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:GetObjectVersion"
        ]
        Resource = [
          "${aws_s3_bucket.config.arn}/expectations.json",
          "${aws_s3_bucket.config.arn}/project-metadata.json"
        ]
      },
      {
        Sid    = "AutoMockECSListAccess"
        Effect = "Allow"
        Action = [
          "s3:ListBucket"
        ]
        Resource = [
          aws_s3_bucket.config.arn
        ]
        Condition = {
          StringLike = {
            "s3:prefix" = [
              "expectations.json",
              "project-metadata.json"
            ]
          }
        }
      }
    ]
  })
}

# CLI integration helpers
output "cli_commands" {
  description = "Useful CLI commands for managing this project"
  value = {
    view_expectations = "aws s3 cp s3://${aws_s3_bucket.config.bucket}/expectations.json - | jq ."
    edit_expectations = "aws s3 cp s3://${aws_s3_bucket.config.bucket}/expectations.json expectations.json && $EDITOR expectations.json && aws s3 cp expectations.json s3://${aws_s3_bucket.config.bucket}/expectations.json"
    view_metadata     = "aws s3 cp s3://${aws_s3_bucket.config.bucket}/project-metadata.json - | jq ."
    list_versions     = "aws s3 ls s3://${aws_s3_bucket.config.bucket}/versions/"
    backup_current    = "aws s3 cp s3://${aws_s3_bucket.config.bucket}/expectations.json s3://${aws_s3_bucket.config.bucket}/versions/expectations-backup-$(date +%Y%m%d-%H%M%S).json"
  }
}

output "monitoring_info" {
  description = "Information for monitoring S3 bucket usage"
  value = {
    cloudwatch_metrics = {
      bucket_size_bytes    = "AWS/S3 BucketSizeBytes for ${aws_s3_bucket.config.bucket}"
      number_of_objects    = "AWS/S3 NumberOfObjects for ${aws_s3_bucket.config.bucket}"
      all_requests        = "AWS/S3 AllRequests for ${aws_s3_bucket.config.bucket}"
    }
    
    s3_access_logs = {
      enabled = false
      note    = "Enable S3 access logging if detailed request tracking is needed"
    }
    
    cost_optimization = {
      lifecycle_rules_enabled = true
      versioning_cleanup_days = var.ttl_hours > 0 ? max(var.ttl_hours / 24, 7) : 30
      intelligent_tiering     = "Consider enabling for long-term projects"
    }
  }
}