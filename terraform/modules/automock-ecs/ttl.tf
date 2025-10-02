# terraform/modules/automock-ecs/ttl.tf
# TTL-Based Auto-Cleanup using EventBridge and Lambda

# Only create TTL resources if TTL cleanup is enabled
locals {
  create_ttl = var.ttl_hours > 0 && var.enable_ttl_cleanup
}

# Lambda function for TTL cleanup
resource "aws_lambda_function" "ttl_cleanup" {
  count = local.create_ttl ? 1 : 0

  filename      = "${path.module}/scripts/ttl_cleanup.zip"
  function_name = "${local.name_prefix}-ttl-cleanup"
  role          = aws_iam_role.lambda_ttl_cleanup[0].arn
  handler       = "ttl_cleanup.lambda_handler"
  runtime       = "python3.11"
  timeout       = 900 # 15 minutes
  memory_size   = 256

  environment {
    variables = {
      PROJECT_NAME         = var.project_name
      ENVIRONMENT          = var.environment
      CLUSTER_NAME         = aws_ecs_cluster.main.name
      SERVICE_NAME         = aws_ecs_service.mockserver.name
      ALB_ARN              = aws_lb.main.arn
      TARGET_GROUP_API_ARN = aws_lb_target_group.mockserver_api.arn
      TARGET_GROUP_DASH_ARN = aws_lb_target_group.mockserver_dashboard.arn
      VPC_ID               = aws_vpc.main.id
      CONFIG_BUCKET        = local.s3_config.bucket_name
      REGION               = var.region
      TTL_HOURS            = var.ttl_hours
      NOTIFICATION_EMAIL   = var.notification_email
    }
  }

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-ttl-cleanup"
  })
}

# IAM Role for Lambda
resource "aws_iam_role" "lambda_ttl_cleanup" {
  count = local.create_ttl ? 1 : 0

  name_prefix = "${local.name_prefix}-lambda-"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "lambda.amazonaws.com"
      }
    }]
  })

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-lambda-role"
  })
}

# Lambda Basic Execution Policy
resource "aws_iam_role_policy_attachment" "lambda_basic_execution" {
  count = local.create_ttl ? 1 : 0

  role       = aws_iam_role.lambda_ttl_cleanup[0].name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# Lambda Cleanup Policy
resource "aws_iam_role_policy" "lambda_cleanup_policy" {
  count = local.create_ttl ? 1 : 0

  name_prefix = "cleanup-policy-"
  role        = aws_iam_role.lambda_ttl_cleanup[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecs:DeleteService",
          "ecs:DeleteCluster",
          "ecs:UpdateService",
          "ecs:DescribeServices",
          "ecs:DescribeClusters",
          "ecs:ListServices",
          "ecs:ListTasks",
          "ecs:StopTask"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "elasticloadbalancing:DeleteLoadBalancer",
          "elasticloadbalancing:DeleteTargetGroup",
          "elasticloadbalancing:DeleteListener",
          "elasticloadbalancing:DescribeLoadBalancers",
          "elasticloadbalancing:DescribeTargetGroups",
          "elasticloadbalancing:DescribeListeners",
          "elasticloadbalancing:DescribeTags"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "ec2:DeleteSecurityGroup",
          "ec2:DeleteSubnet",
          "ec2:DeleteVpc",
          "ec2:DeleteInternetGateway",
          "ec2:DeleteRouteTable",
          "ec2:DeleteNatGateway",
          "ec2:ReleaseAddress",
          "ec2:DetachInternetGateway",
          "ec2:DisassociateRouteTable",
          "ec2:DescribeSecurityGroups",
          "ec2:DescribeSubnets",
          "ec2:DescribeVpcs",
          "ec2:DescribeInternetGateways",
          "ec2:DescribeRouteTables",
          "ec2:DescribeNetworkInterfaces",
          "ec2:DescribeNatGateways",
          "ec2:DescribeAddresses"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "s3:DeleteBucket",
          "s3:DeleteObject",
          "s3:DeleteObjectVersion",
          "s3:GetObject",
          "s3:GetObjectVersion",
          "s3:GetObjectAttributes",
          "s3:ListBucket",
          "s3:ListBucketVersions"
        ]
        Resource = [
          local.config_bucket_arn,
          "${local.config_bucket_arn}/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "logs:DeleteLogGroup"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "events:RemoveTargets",
          "events:DeleteRule"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "lambda:DeleteFunction",
          "lambda:RemovePermission"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "sns:Publish"
        ]
        Resource = var.notification_email != "" ? aws_sns_topic.ttl_notifications[0].arn : "*"
      }
    ]
  })
}

# EventBridge Rule for TTL Check
resource "aws_cloudwatch_event_rule" "ttl_check" {
  count = local.create_ttl ? 1 : 0

  name                = "${local.name_prefix}-ttl-check"
  description         = "Hourly check for TTL expiration"
  schedule_expression = "rate(1 hour)"

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-ttl-rule"
  })
}

# EventBridge Target
resource "aws_cloudwatch_event_target" "lambda" {
  count = local.create_ttl ? 1 : 0

  rule      = aws_cloudwatch_event_rule.ttl_check[0].name
  target_id = "TTLCleanupLambda"
  arn       = aws_lambda_function.ttl_cleanup[0].arn
}

# Lambda Permission for EventBridge
resource "aws_lambda_permission" "allow_eventbridge" {
  count = local.create_ttl ? 1 : 0

  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.ttl_cleanup[0].function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.ttl_check[0].arn
}

# SNS Topic for TTL Notifications
resource "aws_sns_topic" "ttl_notifications" {
  count = local.create_ttl && var.notification_email != "" ? 1 : 0

  name_prefix = "${local.name_prefix}-ttl-"

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-ttl-notifications"
  })
}

# SNS Subscription
resource "aws_sns_topic_subscription" "email" {
  count = local.create_ttl && var.notification_email != "" ? 1 : 0

  topic_arn = aws_sns_topic.ttl_notifications[0].arn
  protocol  = "email"
  endpoint  = var.notification_email
}

# CloudWatch Metric Alarm for TTL Warning (1 hour before expiry)
resource "aws_cloudwatch_metric_alarm" "ttl_warning" {
  count = local.create_ttl && var.notification_email != "" ? 1 : 0

  alarm_name          = "${local.name_prefix}-ttl-warning"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "1"
  metric_name         = "TTLWarning"
  namespace           = "AutoMock/TTL"
  period              = "3600"
  statistic           = "Sum"
  threshold           = "0"
  alarm_description   = "TTL expiration warning (1 hour remaining)"
  alarm_actions       = [aws_sns_topic.ttl_notifications[0].arn]

  tags = merge(local.common_tags, local.ttl_tags)
}
