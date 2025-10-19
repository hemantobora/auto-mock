# terraform/modules/automock-ecs/ttl.tf
# TTL-Based Auto-Cleanup using EventBridge and Lambda

# Only create TTL resources if TTL cleanup is enabled
locals {
  create_ttl = var.ttl_hours > 0 && var.enable_ttl_cleanup
}

# Archive Lambda source code
data "archive_file" "ttl_cleanup" {
  count = local.create_ttl ? 1 : 0

  type        = "zip"
  source_file = "${path.module}/scripts/ttl_cleanup.py"
  output_path = "${path.module}/scripts/ttl_cleanup.zip"
}

# Lambda function for TTL cleanup
resource "aws_lambda_function" "ttl_cleanup" {
  count = var.ttl_hours > 0 && var.enable_ttl_cleanup ? 1 : 0

  filename         = data.archive_file.ttl_cleanup[0].output_path
  source_code_hash = data.archive_file.ttl_cleanup[0].output_base64sha256
  function_name    = "${local.name_prefix}-ttl-cleanup"
  role             = aws_iam_role.lambda_ttl_cleanup[0].arn
  handler          = "ttl_cleanup.lambda_handler"
  runtime          = "python3.11"
  timeout          = 900 # 15 minutes
  memory_size      = 256

  environment {
    variables = {
      PROJECT_NAME            = var.project_name
      CLUSTER_NAME            = aws_ecs_cluster.main.name
      DESTROY_TASK_DEFINITION = aws_ecs_task_definition.terraform_destroy[0].family
      SUBNETS                 = join(",", aws_subnet.private[*].id)
      SECURITY_GROUP          = aws_security_group.ecs_tasks.id
      CONFIG_BUCKET           = local.config_bucket_name
    }
  }

  # Ensure task definition exists before Lambda
  depends_on = [
    aws_ecs_task_definition.terraform_destroy
  ]

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-ttl-cleanup"
  })
}

# Lambda needs permission to run ECS tasks
resource "aws_iam_role_policy" "lambda_run_ecs_task" {
  name = "run-ecs-task"
  role = aws_iam_role.lambda_ttl_cleanup[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecs:RunTask",
          "ecs:DescribeTasks",
          "ecs:TagResource"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = "iam:PassRole"
        Resource = [
          aws_iam_role.ecs_task_execution.arn,
          aws_iam_role.terraform_destroy[0].arn
        ]
      }
    ]
  })
}

# IAM Role for Lambda
resource "aws_iam_role" "lambda_ttl_cleanup" {
  count = var.ttl_hours > 0 && var.enable_ttl_cleanup && var.cleanup_role_arn == "" ? 1 : 0

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


# ECS Task Definition for Terraform Destroy
resource "aws_ecs_task_definition" "terraform_destroy" {
  count = var.enable_ttl_cleanup ? 1 : 0
  family                   = "${local.name_prefix}-terraform-destroy"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = 512   # Increased for Terraform
  memory                   = 1024  # Increased for Terraform
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.terraform_destroy[0].arn

  container_definitions = jsonencode([{
    name      = "terraform-destroy"
    image     = "${aws_ecr_repository.terraform_destroy[0].repository_url}:latest"  # ← Uses ECR image
    essential = true

    environment = [
      {
        name  = "PROJECT_NAME"
        value = var.project_name
      },
      {
        name  = "AWS_REGION"
        value = var.region
      },
      {
        name  = "S3_BUCKET"
        value = local.config_bucket_name
      }
    ]

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.terraform_destroy[0].name
        "awslogs-region"        = var.region
        "awslogs-stream-prefix" = "terraform-destroy"
      }
    }
  }])

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-terraform-destroy-task"
  })
  
  # Ensure image is built before task definition
  depends_on = [
    docker_registry_image.terraform_destroy
  ]
}

# CloudWatch Log Group
resource "aws_cloudwatch_log_group" "terraform_destroy" {
  count = var.enable_ttl_cleanup ? 1 : 0
  name              = "/ecs/automock/${var.project_name}/terraform-destroy"
  retention_in_days = 7

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-terraform-destroy-logs"
  })
}

# IAM Role for Terraform Destroy Task
resource "aws_iam_role" "terraform_destroy" {
  count = var.enable_ttl_cleanup ? 1 : 0
  name = "${local.name_prefix}-terraform-destroy-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ecs-tasks.amazonaws.com"
      }
    }]
  })

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-terraform-destroy-role"
  })
}

# IAM Policy - Full permissions to destroy everything
resource "aws_iam_role_policy" "terraform_destroy_permissions" {
  count = var.enable_ttl_cleanup ? 1 : 0
  name = "terraform-destroy-permissions"
  role = aws_iam_role.terraform_destroy[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecs:*",
          "ec2:*",
          "elasticloadbalancing:*",
          "s3:*",
          "logs:*",
          "iam:*",
          "lambda:*",
          "events:*",
          "cloudwatch:*",
          "ecr:*"  # ← Added: Can delete ECR repo
        ]
        Resource = "*"
      }
    ]
  })
}