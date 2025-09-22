# terraform/modules/automock-ecs/ttl.tf
# Auto-Teardown TTL Configuration with EventBridge

# Lambda function for TTL cleanup
resource "aws_lambda_function" "ttl_cleanup" {
  count = var.enable_ttl_cleanup && var.ttl_hours > 0 ? 1 : 0

  filename         = data.archive_file.ttl_cleanup_zip[0].output_path
  function_name    = "${local.name_prefix}-ttl-cleanup"
  role            = aws_iam_role.ttl_cleanup[0].arn
  handler         = "index.handler"
  runtime         = "python3.9"
  timeout         = 300
  source_code_hash = data.archive_file.ttl_cleanup_zip[0].output_base64sha256

  environment {
    variables = {
      CLUSTER_NAME        = aws_ecs_cluster.main.name
      SERVICE_NAME        = aws_ecs_service.mockserver.name
      ALB_ARN            = aws_lb.main.arn
      VPC_ID             = aws_vpc.main.id
      CONFIG_BUCKET      = local.config_bucket_name
      NOTIFICATION_EMAIL = var.notification_email
      TTL_HOURS          = var.ttl_hours
      PROJECT_NAME       = var.project_name
      ENVIRONMENT        = var.environment
    }
  }

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-ttl-cleanup"
  })
}

# Create TTL cleanup Lambda zip file
data "archive_file" "ttl_cleanup_zip" {
  count = var.enable_ttl_cleanup && var.ttl_hours > 0 ? 1 : 0

  type        = "zip"
  output_path = "/tmp/ttl_cleanup_${local.name_prefix}.zip"
  
  source {
    content = templatefile("${path.module}/scripts/ttl_cleanup.py", {
      sns_topic_arn = var.notification_email != "" ? aws_sns_topic.ttl_notifications[0].arn : ""
    })
    filename = "index.py"
  }
}

# IAM Role for TTL cleanup Lambda
resource "aws_iam_role" "ttl_cleanup" {
  count = var.enable_ttl_cleanup && var.ttl_hours > 0 ? 1 : 0
  
  name = "${local.name_prefix}-ttl-cleanup-role"

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

# IAM Policy for TTL cleanup
resource "aws_iam_role_policy" "ttl_cleanup" {
  count = var.enable_ttl_cleanup && var.ttl_hours > 0 ? 1 : 0
  
  name = "${local.name_prefix}-ttl-cleanup-policy"
  role = aws_iam_role.ttl_cleanup[0].id

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
          "ecs:DeleteService",
          "ecs:DeleteCluster",
          "ecs:DescribeServices",
          "ecs:ListServices"
        ]
        Resource = [
          aws_ecs_cluster.main.arn,
          "${aws_ecs_cluster.main.arn}/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "elbv2:DeleteLoadBalancer",
          "elbv2:DeleteTargetGroup",
          "elbv2:DescribeLoadBalancers",
          "elbv2:DescribeTargetGroups"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "ec2:DeleteVpc",
          "ec2:DeleteSubnet",
          "ec2:DeleteInternetGateway",
          "ec2:DeleteNatGateway",
          "ec2:DeleteRouteTable",
          "ec2:DeleteSecurityGroup",
          "ec2:ReleaseAddress",
          "ec2:DetachInternetGateway",
          "ec2:DisassociateRouteTable",
          "ec2:DescribeVpcs",
          "ec2:DescribeSubnets",
          "ec2:DescribeInternetGateways",
          "ec2:DescribeNatGateways",
          "ec2:DescribeRouteTables",
          "ec2:DescribeSecurityGroups",
          "ec2:DescribeAddresses"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "s3:DeleteBucket",
          "s3:DeleteObject",
          "s3:ListBucket"
        ]
        Resource = [
          local.config_bucket_arn,
          "${local.config_bucket_arn}/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "route53:DeleteHostedZone",
          "route53:ChangeResourceRecordSets",
          "route53:ListResourceRecordSets"
        ]
        Resource = var.custom_domain != "" ? [
          "arn:aws:route53:::hostedzone/${var.hosted_zone_id}"
        ] : []
      },
      {
        Effect = "Allow"
        Action = [
          "acm:DeleteCertificate",
          "acm:DescribeCertificate"
        ]
        Resource = var.custom_domain != "" ? [
          aws_acm_certificate.main[0].arn
        ] : []
      },
      {
        Effect = "Allow"
        Action = [
          "sns:Publish"
        ]
        Resource = var.notification_email != "" ? [
          aws_sns_topic.ttl_notifications[0].arn
        ] : []
      }
    ]
  })
}

# EventBridge rule for TTL check
resource "aws_cloudwatch_event_rule" "ttl_check" {
  count = var.enable_ttl_cleanup && var.ttl_hours > 0 ? 1 : 0

  name                = "${local.name_prefix}-ttl-check"
  description         = "Check TTL expiry for AutoMock infrastructure"
  schedule_expression = "rate(15 minutes)"

  tags = merge(local.common_tags, local.ttl_tags)
}

# EventBridge target for TTL Lambda
resource "aws_cloudwatch_event_target" "ttl_lambda" {
  count = var.enable_ttl_cleanup && var.ttl_hours > 0 ? 1 : 0

  rule      = aws_cloudwatch_event_rule.ttl_check[0].name
  target_id = "TriggerTTLCleanup"
  arn       = aws_lambda_function.ttl_cleanup[0].arn

  input = jsonencode({
    project_name = var.project_name
    environment  = var.environment
    ttl_hours    = var.ttl_hours
    created_at   = timestamp()
  })
}

# Lambda permission for EventBridge
resource "aws_lambda_permission" "allow_eventbridge" {
  count = var.enable_ttl_cleanup && var.ttl_hours > 0 ? 1 : 0

  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.ttl_cleanup[0].function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.ttl_check[0].arn
}

# SNS Topic for notifications
resource "aws_sns_topic" "ttl_notifications" {
  count = var.notification_email != "" ? 1 : 0

  name = "${local.name_prefix}-ttl-notifications"

  tags = merge(local.common_tags, local.ttl_tags)
}

# SNS Subscription for email notifications
resource "aws_sns_topic_subscription" "email" {
  count = var.notification_email != "" ? 1 : 0

  topic_arn = aws_sns_topic.ttl_notifications[0].arn
  protocol  = "email"
  endpoint  = var.notification_email
}

# CloudWatch Alarm for early TTL warning
resource "aws_cloudwatch_metric_alarm" "ttl_warning" {
  count = var.enable_ttl_cleanup && var.ttl_hours > 0 && var.notification_email != "" ? 1 : 0

  alarm_name          = "${local.name_prefix}-ttl-warning"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "1"
  metric_name         = "UpTime"
  namespace           = "AutoMock"
  period              = "3600"
  statistic           = "Average"
  threshold           = var.ttl_hours - 0.5  # Alert 30 minutes before expiry
  alarm_description   = "AutoMock infrastructure approaching TTL expiry"
  alarm_actions       = [aws_sns_topic.ttl_notifications[0].arn]

  tags = merge(local.common_tags, local.ttl_tags)
}