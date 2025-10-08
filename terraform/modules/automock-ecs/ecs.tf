# terraform/modules/automock-ecs/ecs.tf
# ECS Cluster, Service, Task Definition, and Auto-Scaling

# CloudWatch Log Groups
resource "aws_cloudwatch_log_group" "mockserver" {
  name              = "/ecs/automock/${var.project_name}/mockserver"
  retention_in_days = 7

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-mockserver-logs"
  })
}

resource "aws_cloudwatch_log_group" "config_loader" {
  name              = "/ecs/automock/${var.project_name}/config-loader"
  retention_in_days = 7

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-config-loader-logs"
  })
}

# ECS Cluster
resource "aws_ecs_cluster" "main" {
  name = local.name_prefix

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
    capacity_provider = "FARGATE"
    weight            = 1
    base              = 1
  }
}

# ECS Task Definition
resource "aws_ecs_task_definition" "mockserver" {
  family                   = local.name_prefix
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = local.cpu_units
  memory                   = local.memory_units
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([
  # MockServer - NO HEALTH CHECK
  {
    name      = "mockserver"
    image     = "mockserver/mockserver:latest"
    essential = true

    portMappings = [
      {
        containerPort = 1080
        protocol      = "tcp"
        name          = "mockserver-api"
      },
      {
        containerPort = 1090
        protocol      = "tcp"
        name          = "mockserver-dashboard"
      }
    ]

    environment = [
      {
        name  = "MOCKSERVER_LOG_LEVEL"
        value = "INFO"
      },
      {
        name  = "MOCKSERVER_SERVER_PORT"
        value = "1080"
      },
      {
        name  = "MOCKSERVER_CORS_ALLOW_ORIGIN"
        value = "*"
      },
      {
        name  = "MOCKSERVER_CORS_ALLOW_METHODS"
        value = "GET, POST, PUT, DELETE, PATCH, OPTIONS"
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
  },
  
  # Config-Watcher - NON-ESSENTIAL
  {
    name      = "config-watcher"
    image     = "amazon/aws-cli:latest"
    essential = false  # ← CRITICAL!

    dependsOn = [{
      containerName = "mockserver"
      condition     = "START"  # ← Changed back to START
    }]

    entryPoint = ["/bin/bash", "-c"]

    command = [
      <<-EOF
        set -e
        
        yum install -y jq
        
        S3_BUCKET="${local.s3_config.bucket_name}"
        PROJECT_NAME="${var.project_name}"
        CONFIG_PATH="configs/$${PROJECT_NAME}/current.json"
        MOCKSERVER_URL="http://localhost:1080"
        POLL_INTERVAL=30
        
        echo "🔄 Config Watcher Starting"
        
        # Simple wait for MockServer
        echo "⏳ Waiting for MockServer (15 seconds)..."
        sleep 15
        
        echo "📥 Loading configuration..."
        if ! aws s3 cp "s3://$${S3_BUCKET}/$${CONFIG_PATH}" /tmp/current.json 2>/dev/null; then
          echo "⚠️  No config file found"
          LAST_ETAG=""
        else
          EXPECTATIONS=$(cat /tmp/current.json | jq -c '.expectations')
          EXP_COUNT=$(echo "$${EXPECTATIONS}" | jq 'length')
          
          echo "  Loading $${EXP_COUNT} expectations..."
          
          HTTP_CODE=$(curl -X PUT "$${MOCKSERVER_URL}/mockserver/expectation" \
            -H "Content-Type: application/json" \
            -d "$${EXPECTATIONS}" \
            -s -w "%%{http_code}" -o /dev/null)
          
          if [ "$${HTTP_CODE}" = "201" ] || [ "$${HTTP_CODE}" = "200" ]; then
            echo "✅ Loaded $${EXP_COUNT} expectations"
          else
            echo "⚠️  Failed (HTTP $${HTTP_CODE}), but continuing..."
          fi
          
          LAST_ETAG=$(aws s3api head-object \
            --bucket "$${S3_BUCKET}" \
            --key "$${CONFIG_PATH}" \
            --query 'ETag' \
            --output text 2>/dev/null || echo "")
        fi
        
        echo "🔄 Polling for changes..."
        while true; do
          sleep $${POLL_INTERVAL}
          
          CURRENT_ETAG=$(aws s3api head-object \
            --bucket "$${S3_BUCKET}" \
            --key "$${CONFIG_PATH}" \
            --query 'ETag' \
            --output text 2>/dev/null || echo "")
          
          if [ -z "$${CURRENT_ETAG}" ] || [ "$${CURRENT_ETAG}" = "$${LAST_ETAG}" ]; then
            continue
          fi
          
          echo "🔔 [$(date '+%%H:%%M:%%S')] Config changed"
          
          aws s3 cp "s3://$${S3_BUCKET}/$${CONFIG_PATH}" /tmp/current.json
          EXPECTATIONS=$(cat /tmp/current.json | jq -c '.expectations')
          
          HTTP_CODE=$(curl -X PUT "$${MOCKSERVER_URL}/mockserver/expectation" \
            -H "Content-Type: application/json" \
            -d "$${EXPECTATIONS}" \
            -s -w "%%{http_code}" -o /dev/null)
          
          if [ "$${HTTP_CODE}" = "201" ] || [ "$${HTTP_CODE}" = "200" ]; then
            LAST_ETAG="$${CURRENT_ETAG}"
            echo "✅ Updated"
          fi
        done
      EOF
    ]

    environment = [
      {
        name  = "AWS_DEFAULT_REGION"
        value = var.region
      }
    ]

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.config_loader.name
        "awslogs-region"        = var.region
        "awslogs-stream-prefix" = "config-watcher"
      }
    }
  }
])

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-task"
  })
}

# ECS Service
resource "aws_ecs_service" "mockserver" {
  name            = "${local.name_prefix}-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.mockserver.arn
  desired_count   = var.min_tasks
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

  load_balancer {
    target_group_arn = aws_lb_target_group.mockserver_dashboard.arn
    container_name   = "mockserver"
    container_port   = 1090
  }

  deployment_controller {
    type = "ECS"
  }

  deployment_maximum_percent         = 200
  deployment_minimum_healthy_percent = 100

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  enable_execute_command = true

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-service"
  })

  depends_on = [
    aws_lb_listener.https_api,
    aws_lb_listener.https_dashboard,
    aws_iam_role_policy.s3_read_config
  ]
}

# Auto-Scaling Target
resource "aws_appautoscaling_target" "ecs_service" {
  max_capacity       = var.max_tasks
  min_capacity       = var.min_tasks
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.mockserver.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

# CPU-Based Step Scaling Policy
resource "aws_appautoscaling_policy" "cpu_step_scaling" {
  name               = "${local.name_prefix}-cpu-step-scaling"
  policy_type        = "StepScaling"
  resource_id        = aws_appautoscaling_target.ecs_service.resource_id
  scalable_dimension = aws_appautoscaling_target.ecs_service.scalable_dimension
  service_namespace  = aws_appautoscaling_target.ecs_service.service_namespace

  step_scaling_policy_configuration {
    adjustment_type         = "PercentChangeInCapacity"
    cooldown               = 60
    metric_aggregation_type = "Average"

    # 70-80% CPU: Add 50% more tasks
    step_adjustment {
      metric_interval_lower_bound = 0
      metric_interval_upper_bound = 10
      scaling_adjustment          = 50
    }

    # 80-90% CPU: Add 100% more tasks
    step_adjustment {
      metric_interval_lower_bound = 10
      metric_interval_upper_bound = 20
      scaling_adjustment          = 100
    }

    # 90%+ CPU: Add 200% more tasks
    step_adjustment {
      metric_interval_lower_bound = 20
      scaling_adjustment          = 200
    }
  }
}

# CPU High Alarm
resource "aws_cloudwatch_metric_alarm" "cpu_high" {
  alarm_name          = "${local.name_prefix}-cpu-high"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = "2"
  metric_name         = "CPUUtilization"
  namespace           = "AWS/ECS"
  period              = "60"
  statistic           = "Average"
  threshold           = "70"
  alarm_description   = "Triggers step scaling when CPU >= 70%"
  alarm_actions       = [aws_appautoscaling_policy.cpu_step_scaling.arn]

  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.mockserver.name
  }

  tags = merge(local.common_tags, local.ttl_tags)
}

# Memory-Based Step Scaling Policy
resource "aws_appautoscaling_policy" "memory_step_scaling" {
  name               = "${local.name_prefix}-memory-step-scaling"
  policy_type        = "StepScaling"
  resource_id        = aws_appautoscaling_target.ecs_service.resource_id
  scalable_dimension = aws_appautoscaling_target.ecs_service.scalable_dimension
  service_namespace  = aws_appautoscaling_target.ecs_service.service_namespace

  step_scaling_policy_configuration {
    adjustment_type         = "PercentChangeInCapacity"
    cooldown               = 60
    metric_aggregation_type = "Average"

    step_adjustment {
      metric_interval_lower_bound = 0
      metric_interval_upper_bound = 10
      scaling_adjustment          = 50
    }

    step_adjustment {
      metric_interval_lower_bound = 10
      metric_interval_upper_bound = 20
      scaling_adjustment          = 100
    }

    step_adjustment {
      metric_interval_lower_bound = 20
      scaling_adjustment          = 200
    }
  }
}

# Memory High Alarm
resource "aws_cloudwatch_metric_alarm" "memory_high" {
  alarm_name          = "${local.name_prefix}-memory-high"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = "2"
  metric_name         = "MemoryUtilization"
  namespace           = "AWS/ECS"
  period              = "60"
  statistic           = "Average"
  threshold           = "70"
  alarm_description   = "Triggers step scaling when memory >= 70%"
  alarm_actions       = [aws_appautoscaling_policy.memory_step_scaling.arn]

  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.mockserver.name
  }

  tags = merge(local.common_tags, local.ttl_tags)
}

# Request Count Step Scaling Policy
resource "aws_appautoscaling_policy" "request_step_scaling" {
  name               = "${local.name_prefix}-request-step-scaling"
  policy_type        = "StepScaling"
  resource_id        = aws_appautoscaling_target.ecs_service.resource_id
  scalable_dimension = aws_appautoscaling_target.ecs_service.scalable_dimension
  service_namespace  = aws_appautoscaling_target.ecs_service.service_namespace

  step_scaling_policy_configuration {
    adjustment_type         = "PercentChangeInCapacity"
    cooldown               = 60
    metric_aggregation_type = "Average"

    # 500-1000 req/min per task: Add 50%
    step_adjustment {
      metric_interval_lower_bound = 0
      metric_interval_upper_bound = 500
      scaling_adjustment          = 50
    }

    # 1000+ req/min per task: Add 100%
    step_adjustment {
      metric_interval_lower_bound = 500
      scaling_adjustment          = 100
    }
  }
}

# Request Count High Alarm
resource "aws_cloudwatch_metric_alarm" "requests_high" {
  alarm_name          = "${local.name_prefix}-requests-high"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = "2"
  metric_name         = "RequestCountPerTarget"
  namespace           = "AWS/ApplicationELB"
  period              = "60"
  statistic           = "Sum"
  threshold           = "500"
  alarm_description   = "Triggers step scaling when requests >= 500/min per target"
  alarm_actions       = [aws_appautoscaling_policy.request_step_scaling.arn]

  dimensions = {
    TargetGroup  = aws_lb_target_group.mockserver_api.arn_suffix
    LoadBalancer = aws_lb.main.arn_suffix
  }

  tags = merge(local.common_tags, local.ttl_tags)
}

# Scale Down Policy
resource "aws_appautoscaling_policy" "scale_down" {
  name               = "${local.name_prefix}-scale-down"
  policy_type        = "StepScaling"
  resource_id        = aws_appautoscaling_target.ecs_service.resource_id
  scalable_dimension = aws_appautoscaling_target.ecs_service.scalable_dimension
  service_namespace  = aws_appautoscaling_target.ecs_service.service_namespace

  step_scaling_policy_configuration {
    adjustment_type         = "PercentChangeInCapacity"
    cooldown               = 300 # 5 min cooldown
    metric_aggregation_type = "Average"

    # Remove 25% of tasks when low
    step_adjustment {
      metric_interval_upper_bound = 0
      scaling_adjustment          = -25
    }
  }
}

# CPU Low Alarm
resource "aws_cloudwatch_metric_alarm" "cpu_low" {
  alarm_name          = "${local.name_prefix}-cpu-low"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "5"
  metric_name         = "CPUUtilization"
  namespace           = "AWS/ECS"
  period              = "60"
  statistic           = "Average"
  threshold           = "40"
  alarm_description   = "Scale down when CPU < 40% for 5 minutes"
  alarm_actions       = [aws_appautoscaling_policy.scale_down.arn]

  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.mockserver.name
  }

  tags = merge(local.common_tags, local.ttl_tags)
}
