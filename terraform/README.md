# AutoMock Terraform Infrastructure

Complete Infrastructure-as-Code for deploying AutoMock mock API servers on AWS with ECS Fargate, Application Load Balancer, auto-scaling, and monitoring.

## 📋 Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Quick Start](#quick-start)
4. [Modules](#modules)
5. [Configuration](#configuration)
6. [Auto-Scaling](#auto-scaling)
7. [Monitoring](#monitoring)
8. [Cost Estimates](#cost-estimates)
9. [Security](#security)
10. [Troubleshooting](#troubleshooting)

## Overview

This directory contains modular Terraform configurations for deploying production-ready mock API infrastructure on AWS. The infrastructure includes:

- **ECS Fargate** - Containerized MockServer with config loader sidecar
- **Application Load Balancer** - Public HTTP/HTTPS access
- **Auto-Scaling** - CPU, memory, and request-based scaling (10-200 tasks)
- **S3 Storage** - Versioned configuration storage
- **CloudWatch** - Comprehensive logging and monitoring
- **Optional TTL Cleanup** - Automatic infrastructure teardown

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Public Internet                           │
└────────────────────┬─────────────────────────────────────────┘
                     │
                     ▼
        ┌────────────────────────────┐
        │  Application Load Balancer │
        │  (Public Subnets)          │
        │  • Port 80                 │
        │  • Health checks           │
        └────────┬──────────┬────────┘
                 │          │
       ┌─────────┴───┐  ┌──┴──────────┐
       │ Target Group│  │ Target Group │
       │ /           │  │ /dashboard  │
       └─────────┬───┘  └──┬──────────┘
                 │          │
                 └────┬─────┘
                      ▼
        ┌──────────────────────────────┐
        │  ECS Fargate Service         │
        │  (Private Subnets)           │
        │  ┌────────────────────────┐  │
        │  │ Task 1                 │  │
        │  │ • MockServer (1080)    │  │
        │  │ • Config Loader        │  │
        │  └────────────────────────┘  │
        │  ┌────────────────────────┐  │
        │  │ Task 2..N              │  │
        │  └────────────────────────┘  │
        │  Auto-scaling: 10-200 tasks  │
        └───────────┬──────────────────┘
                    │
                    ▼
        ┌──────────────────────────────┐
        │  S3 Bucket                   │
        │  • expectations.json         │
        │  • metadata.json             │
        │  • Versioning enabled        │
        └──────────────────────────────┘
```

### Network Architecture

```
VPC (10.0.0.0/16)
├── Public Subnets (10.0.1.0/24, 10.0.2.0/24)
│   ├── Application Load Balancer
│   └── NAT Gateways
└── Private Subnets (10.0.3.0/24, 10.0.4.0/24)
    └── ECS Fargate Tasks
```

## Quick Start

### Prerequisites

- Terraform >= 1.0
- AWS CLI configured
- Valid AWS credentials with required permissions
- AutoMock project with expectations already created

### Deploy from AutoMock CLI (Recommended)

```bash
# Using AutoMock CLI (handles everything)
./automock deploy --project my-api
```

### Deploy Directly with Terraform

```bash
# Initialize Terraform
cd terraform
terraform init

# Plan deployment
terraform plan \
  -var="project_name=my-api" \
  -var="environment=dev"

# Deploy
terraform apply \
  -var="project_name=my-api" \
  -var="environment=dev"

# Get outputs
terraform output
```

### Deploy with Custom Variables

```bash
terraform apply \
  -var="project_name=my-api" \
  -var="instance_size=medium" \
  -var="min_tasks=20" \
  -var="max_tasks=100" \
  -var="ttl_hours=8"
```

## Modules

### Root Configuration

```
terraform/
├── main.tf              # Main configuration
├── variables.tf         # Input variables
└── outputs.tf           # Output values
```

The root configuration orchestrates all infrastructure components:
- ECS Cluster and Service
- Application Load Balancer
- Auto-Scaling policies
- CloudWatch monitoring
- IAM roles and policies
- S3 configuration bucket

## Configuration

### Input Variables

#### Required Variables

```hcl
variable "project_name" {
  description = "Name of the AutoMock project"
  type        = string
}
```

#### Optional Variables with Defaults

```hcl
variable "aws_region" {
  description = "AWS region for deployment"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "instance_size" {
  description = "Task size: small, medium, large, xlarge"
  type        = string
  default     = "small"
  # small:  0.25 vCPU, 0.5 GB
  # medium: 0.5 vCPU,  1 GB
  # large:  1 vCPU,    2 GB
  # xlarge: 2 vCPU,    4 GB
}

variable "min_tasks" {
  description = "Minimum number of ECS tasks"
  type        = number
  default     = 10
}

variable "max_tasks" {
  description = "Maximum number of ECS tasks"
  type        = number
  default     = 200
}

variable "ttl_hours" {
  description = "Hours before auto-cleanup (0 = disabled)"
  type        = number
  default     = 4
}

variable "enable_ttl_cleanup" {
  description = "Enable automatic TTL-based cleanup"
  type        = bool
  default     = true
}

variable "custom_domain" {
  description = "Custom domain name (optional)"
  type        = string
  default     = ""
}

variable "hosted_zone_id" {
  description = "Route53 hosted zone ID (required with custom_domain)"
  type        = string
  default     = ""
}

variable "notification_email" {
  description = "Email for infrastructure notifications"
  type        = string
  default     = ""
}
```

### Task Sizing

| Size | vCPU | Memory | Use Case |
|------|------|--------|----------|
| small | 0.25 | 0.5 GB | Development, light APIs |
| medium | 0.5 | 1 GB | Standard APIs |
| large | 1 | 2 GB | Heavy processing |
| xlarge | 2 | 4 GB | High-throughput APIs |

### Outputs

```hcl
output "alb_dns_name" {
  description = "ALB DNS name for API access"
  value       = "automock-{project}-{id}.{region}.elb.amazonaws.com"
}

output "api_url" {
  description = "Full API URL"
  value       = "http://automock-{project}-{id}.{region}.elb.amazonaws.com"
}

output "dashboard_url" {
  description = "MockServer dashboard URL"
  value       = "http://...{elb}.../mockserver/dashboard"
}

output "config_bucket_name" {
  description = "S3 bucket containing expectations"
  value       = "automock-{project}-config-{suffix}"
}

output "ecs_cluster_name" {
  description = "ECS cluster name"
  value       = "automock-{project}"
}

output "ecs_service_name" {
  description = "ECS service name"
  value       = "automock-{project}-service"
}

output "cloudwatch_log_group" {
  description = "CloudWatch log group name"
  value       = "/ecs/automock/{project}/mockserver"
}
```

## Auto-Scaling

### Scaling Policies

AutoMock uses multiple scaling dimensions for optimal performance:

#### CPU-Based Scaling

| CPU Usage | Action | Example |
|-----------|--------|---------|
| 70-80% | +50% tasks | 10 → 15 |
| 80-90% | +100% tasks | 10 → 20 |
| 90%+ | +200% tasks | 10 → 30 |
| < 40% (5 min) | -25% tasks | 20 → 15 |

**Configuration:**
```hcl
# High CPU alarm triggers aggressive scale-up
resource "aws_cloudwatch_metric_alarm" "cpu_high" {
  metric_name         = "CPUUtilization"
  threshold           = 90
  evaluation_periods  = 2
  period              = 60
}
```

#### Memory-Based Scaling

| Memory Usage | Action | Example |
|-------------|--------|---------|
| 70-80% | +50% tasks | 10 → 15 |
| 80-90% | +100% tasks | 10 → 20 |
| 90%+ | +200% tasks | 10 → 30 |
| < 40% (5 min) | -25% tasks | 20 → 15 |

#### Request-Based Scaling

| Requests/min | Action | Example |
|-------------|--------|---------|
| 500-1000 | +50% tasks | 10 → 15 |
| 1000+ | +100% tasks | 10 → 20 |

**Target Tracking:**
```hcl
resource "aws_appautoscaling_policy" "ecs_target_tracking" {
  policy_type = "TargetTrackingScaling"
  
  target_tracking_scaling_policy_configuration {
    target_value = 70.0
    
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
  }
}
```

### Scale-Down Protection

- **Cooldown**: 5 minutes between scale events
- **Conservative**: Only removes 25% of tasks at a time
- **Threshold**: Must be < 40% CPU for 5 minutes
- **Minimum**: Never scales below `min_tasks` setting

## Monitoring

### CloudWatch Metrics

**ECS Service:**
- `CPUUtilization` - Task CPU usage
- `MemoryUtilization` - Task memory usage
- `DesiredTaskCount` - Target task count
- `RunningTaskCount` - Actual running tasks

**Application Load Balancer:**
- `RequestCount` - Total requests
- `TargetResponseTime` - Average response time
- `HTTPCode_Target_2XX_Count` - Success responses
- `HTTPCode_Target_4XX_Count` - Client errors
- `HTTPCode_Target_5XX_Count` - Server errors
- `UnHealthyHostCount` - Unhealthy targets

**Custom Metrics:**
- `ConfigReloadCount` - Expectation updates
- `ExpectationCount` - Total expectations
- `MockServerErrors` - MockServer errors

### CloudWatch Alarms

```hcl
# Unhealthy targets
resource "aws_cloudwatch_metric_alarm" "unhealthy_host_count" {
  alarm_name          = "automock-${var.project_name}-unhealthy-hosts"
  comparison_operator = "GreaterThanThreshold"
  threshold           = "0"
  evaluation_periods  = "2"
  metric_name         = "UnHealthyHostCount"
}

# High error rate
resource "aws_cloudwatch_metric_alarm" "high_5xx_errors" {
  alarm_name          = "automock-${var.project_name}-high-5xx-errors"
  comparison_operator = "GreaterThanThreshold"
  threshold           = "10"
  evaluation_periods  = "2"
  metric_name         = "HTTPCode_Target_5XX_Count"
}
```

### Logs

**Log Groups:**
- `/ecs/automock/{project}/mockserver` - MockServer logs
- `/ecs/automock/{project}/config-loader` - Config loader logs
- `/aws/lambda/automock-{project}-ttl-cleanup` - Cleanup logs

**Retention:** 30 days (configurable)

**Viewing Logs:**
```bash
# MockServer logs
aws logs tail /ecs/automock/my-api/mockserver --follow

# Config loader logs
aws logs tail /ecs/automock/my-api/config-loader --follow
```

## Cost Estimates

### Base Infrastructure (10 tasks, 24/7)

| Component | Hourly | Monthly |
|-----------|--------|---------|
| ECS Fargate (small) | $0.0486 | ~$35 |
| Application Load Balancer | $0.0225 | ~$16 |
| NAT Gateways (2x) | $0.090 | ~$64 |
| Data Transfer (10 GB) | - | ~$9 |
| CloudWatch Logs | - | ~$0.50 |
| S3 Storage (1 GB) | - | ~$0.30 |
| **Total** | **~$0.17/hr** | **~$125/mo** |

### With Auto-Scaling (10-50 avg)

| Component | Monthly |
|-----------|---------|
| Base (10 tasks) | ~$125 |
| Additional (40 tasks) | ~$140 |
| **Total** | **~$265** |

### Cost by Task Size

| Size | Hourly (10 tasks) | Monthly (10 tasks, 24/7) |
|------|-------------------|--------------------------|
| small (0.25 vCPU, 0.5 GB) | $0.0486 | ~$35 |
| medium (0.5 vCPU, 1 GB) | $0.0972 | ~$70 |
| large (1 vCPU, 2 GB) | $0.1944 | ~$140 |
| xlarge (2 vCPU, 4 GB) | $0.3888 | ~$280 |

### TTL-Based Costs (small, 10 tasks)

| Duration | Cost |
|----------|------|
| 1 hour | ~$0.17 |
| 4 hours | ~$0.68 |
| 8 hours | ~$1.37 |
| 24 hours | ~$4.11 |
| 1 week | ~$28.77 |
| 1 month | ~$125 |

### Cost Optimization Strategies

1. **Use TTL Cleanup**
   ```bash
   automock deploy --project my-api
   # Infrastructure auto-destroys after 4 hours (default)
   ```

2. **Reduce Min Tasks**
   ```hcl
   variable "min_tasks" {
     default = 5  # Instead of 10
   }
   ```

3. **Use Smaller Instance Size**
   ```bash
   terraform apply -var="instance_size=small"
   ```

4. **Destroy When Not in Use**
   ```bash
   automock destroy --project my-api
   ```

5. **Single NAT Gateway** (Not Recommended for Prod)
   ```hcl
   # Modify networking to use 1 NAT instead of 2
   # Saves ~$32/month but removes HA
   ```

## Security

### IAM Roles

**ECS Task Execution Role:**
- ECR image pulls
- CloudWatch log writes
- SSM parameter reads

**ECS Task Role:**
- S3 read access (expectations bucket)
- CloudWatch metrics writes

**Lambda Cleanup Role:**
- ECS describe/update/delete
- ALB describe/delete
- VPC describe/delete
- S3 read/delete
- CloudWatch logs

### Policies

```hcl
# Task Execution Role Policy
resource "aws_iam_role_policy_attachment" "ecs_task_execution_role_policy" {
  role       = aws_iam_role.ecs_task_execution_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# Task Role Policy (S3 read)
resource "aws_iam_role_policy" "ecs_task_policy" {
  role = aws_iam_role.ecs_task_role.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:ListBucket"
        ]
        Resource = [
          aws_s3_bucket.config.arn,
          "${aws_s3_bucket.config.arn}/*"
        ]
      }
    ]
  })
}
```

### Network Security

**Security Groups:**
- ALB: Ingress 80/443, Egress to ECS
- ECS: Ingress 1080 from ALB, Egress 443 (S3)

**Network ACLs:**
- Default VPC NACLs (allow all)

**Encryption:**
- S3: Server-side encryption (AES-256)
- ALB: Can terminate SSL/TLS (with custom domain)
- ECS: No encryption (HTTP within VPC)

## Troubleshooting

### Tasks Not Starting

**Check ECS Service:**
```bash
aws ecs describe-services \
  --cluster automock-my-api \
  --services automock-my-api-service
```

**Check Task Definition:**
```bash
aws ecs describe-task-definition \
  --task-definition automock-my-api
```

**View Logs:**
```bash
aws logs tail /ecs/automock/my-api/mockserver --follow
```

**Common Issues:**
- IAM role permissions
- S3 bucket not found
- Invalid expectations JSON
- CPU/memory insufficient

### Health Checks Failing

**Test MockServer Health:**
```bash
curl http://alb-dns-name/mockserver/status
```

**Check Target Health:**
```bash
aws elbv2 describe-target-health \
  --target-group-arn arn:aws:elasticloadbalancing:...
```

**Common Issues:**
- MockServer not starting
- Health check path incorrect
- Security group blocking traffic
- Tasks in private subnet without NAT

### Configuration Not Loading

**Check S3 Bucket:**
```bash
aws s3 ls s3://automock-my-api-config-abc123/
aws s3 cp s3://automock-my-api-config-abc123/expectations.json -
```

**Validate JSON:**
```bash
cat expectations.json | jq .
```

**Check Config Loader Logs:**
```bash
aws logs tail /ecs/automock/my-api/config-loader --follow
```

### High Costs

**Check Running Tasks:**
```bash
aws ecs describe-services \
  --cluster automock-my-api \
  --services automock-my-api-service \
  --query 'services[0].runningCount'
```

**Review Auto-Scaling History:**
```bash
aws application-autoscaling describe-scaling-activities \
  --service-namespace ecs \
  --resource-id service/automock-my-api/automock-my-api-service
```

**Destroy Infrastructure:**
```bash
automock destroy --project my-api --force
```

## Development

### Local Testing

**Validate Configuration:**
```bash
terraform validate
```

**Format Code:**
```bash
terraform fmt -recursive
```

**Plan Without Applying:**
```bash
terraform plan -out=tfplan
terraform show tfplan
```

### Testing Changes

1. Create a test project
2. Apply changes
3. Verify functionality
4. Destroy test infrastructure

```bash
# Test deployment
terraform apply -var="project_name=test-$(date +%s)"

# Test functionality
curl http://alb-dns-name/mockserver/status

# Cleanup
terraform destroy -var="project_name=test-..."
```

## Support

### Documentation
- **Root README**: [../README.md](../README.md)
- **Getting Started**: [../GETTING_STARTED.md](../GETTING_STARTED.md)
- **CLI Help**: `./automock help`

### Issues
- GitHub Issues: https://github.com/hemantobora/auto-mock/issues

### Best Practices

1. **Always use TTL** for ephemeral test environments
2. **Use tags** to track resources
3. **Monitor costs** with AWS Cost Explorer
4. **Set up billing alerts** in AWS Console
5. **Use smallest instance size** that meets requirements
6. **Destroy infrastructure** when not in use
7. **Version expectations** in git
8. **Test locally first** with Docker Compose (future feature)

---

For more information, see:
- [AutoMock CLI Documentation](../GETTING_STARTED.md)
- [AWS ECS Best Practices](https://docs.aws.amazon.com/AmazonECS/latest/bestpracticesguide/)
- [Terraform AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
