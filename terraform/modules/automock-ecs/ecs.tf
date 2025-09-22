# terraform/modules/automock-ecs/ecs.tf
# ECS Fargate Configuration

# ECS Cluster
resource "aws_ecs_cluster" "main" {
  name = "${local.name_prefix}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-cluster"
  })
}

# ECS Cluster Capacity Providers
resource "aws_ecs_cluster_capacity_providers" "main" {
  cluster_name = aws_ecs_cluster.main.name

  capacity_providers = ["FARGATE", "FARGATE_SPOT"]

  default_capacity_provider_strategy {
    base              = 1
    weight            = 100
    capacity_provider = "FARGATE"
  }
}

# CloudWatch Log Group
resource "aws_cloudwatch_log_group" "mockserver" {
  name              = "/ecs/${local.name_prefix}/mockserver"
  retention_in_days = var.ttl_hours > 0 ? min(max(var.ttl_hours / 24, 1), 14) : 14

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-mockserver-logs"
  })
}

# IAM Role for ECS Task Execution
resource "aws_iam_role" "ecs_task_execution" {
  name = "${local.name_prefix}-ecs-exec-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
      }
    ]
  })

  tags = merge(local.common_tags, local.ttl_tags)
}

resource "aws_iam_role_policy_attachment" "ecs_task_execution" {
  role       = aws_iam_role.ecs_task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# IAM Role for ECS Task (Runtime)
resource "aws_iam_role" "ecs_task" {
  name = "${local.name_prefix}-ecs-task-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
      }
    ]
  })

  tags = merge(local.common_tags, local.ttl_tags)
}

# IAM Policy for S3 access
resource "aws_iam_role_policy" "ecs_task_s3" {
  name = "${local.name_prefix}-ecs-s3-policy"
  role = aws_iam_role.ecs_task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject"
        ]
        Resource = [
          "${local.config_bucket_arn}/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "s3:ListBucket"
        ]
        Resource = [
          local.config_bucket_arn
        ]
      }
    ]
  })
}

# ECS Task Definition
resource "aws_ecs_task_definition" "mockserver" {
  family                   = "${local.name_prefix}-mockserver"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = local.cpu_units
  memory                   = local.memory_units
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([
    {
      name = "mockserver"
      image = "mockserver/mockserver:5.15.0"
      essential = true
      
      portMappings = [
        {
          containerPort = 1080
          protocol      = "tcp"
          name          = "mockserver-api"
        }
      ]
      
      environment = [
        {
          name  = "MOCKSERVER_SERVER_PORT"
          value = "1080"
        },
        {
          name  = "MOCKSERVER_LOG_LEVEL"
          value = var.environment == "prod" ? "WARN" : "INFO"
        },
        {
          name  = "MOCKSERVER_INITIALIZATION_JSON_PATH"
          value = "/config/expectations.json"
        },
        {
          name  = "MOCKSERVER_PROPERTY_FILE"
          value = "/config/mockserver.properties"
        },
        {
          name  = "CONFIG_BUCKET"
          value = local.config_bucket_name
        },
        {
          name  = "AUTOMOCK_CONFIG_BUCKET"
          value = local.config_bucket_name
        },
        {
          name  = "AUTOMOCK_CONFIG_KEY"
          value = local.s3_config.expectations_path
        },
        {
          name  = "AUTOMOCK_METADATA_KEY"
          value = local.s3_config.metadata_path
        },
        {
          name  = "AUTOMOCK_PROJECT_NAME"
          value = var.project_name
        },
        {
          name  = "AUTOMOCK_ENVIRONMENT"
          value = var.environment
        },
        {
          name  = "AWS_DEFAULT_REGION"
          value = var.region
        }
      ]
      
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.mockserver.name
          "awslogs-region"        = var.region
          "awslogs-stream-prefix" = "mockserver"
        }
      }
      
      healthCheck = {
        command = [
          "CMD-SHELL",
          "curl -f http://localhost:1080/mockserver/status || exit 1"
        ]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 60
      }
      
      # Resource limits
      memory = local.memory_units
      cpu    = local.cpu_units
    }
  ])

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-mockserver-task"
  })
}

# ECS Service
resource "aws_ecs_service" "mockserver" {
  name            = "${local.name_prefix}-mockserver"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.mockserver.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.mockserver_api.arn
    container_name   = "mockserver"
    container_port   = 1080
  }

  depends_on = [
    aws_lb_listener.http,
    aws_iam_role_policy.ecs_task_s3
  ]

  enable_execute_command = true

  deployment_configuration {
    maximum_percent         = 200
    minimum_healthy_percent = 100
  }

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-mockserver-service"
  })

  lifecycle {
    ignore_changes = [desired_count]
  }
}

# Auto Scaling Target
resource "aws_appautoscaling_target" "ecs_target" {
  max_capacity       = var.environment == "prod" ? 10 : 3
  min_capacity       = 1
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.mockserver.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"

  tags = merge(local.common_tags, local.ttl_tags)
}

# Auto Scaling Policy - CPU
resource "aws_appautoscaling_policy" "ecs_cpu" {
  name               = "${local.name_prefix}-cpu-scaling"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.ecs_target.resource_id
  scalable_dimension = aws_appautoscaling_target.ecs_target.scalable_dimension
  service_namespace  = aws_appautoscaling_target.ecs_target.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value       = 70.0
    scale_in_cooldown  = 300
    scale_out_cooldown = 300
  }
}

# Auto Scaling Policy - Memory
resource "aws_appautoscaling_policy" "ecs_memory" {
  name               = "${local.name_prefix}-memory-scaling"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.ecs_target.resource_id
  scalable_dimension = aws_appautoscaling_target.ecs_target.scalable_dimension
  service_namespace  = aws_appautoscaling_target.ecs_target.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageMemoryUtilization"
    }
    target_value       = 80.0
    scale_in_cooldown  = 300
    scale_out_cooldown = 300
  }
}