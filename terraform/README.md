# AutoMock Terraform Infrastructure

Complete Infrastructure-as-Code for deploying AutoMock mock API servers on AWS.

## Overview

This directory contains modular Terraform configurations for deploying production-ready mock API infrastructure on AWS ECS Fargate with auto-scaling, TTL-based cleanup, and optional custom domains.

## Architecture

```
terraform/
├── main.tf              # Root configuration
├── variables.tf         # Input variables
├── outputs.tf           # Output values
└── modules/
    ├── state-backend/   # S3 + DynamoDB for Terraform state
    ├── automock-s3/     # Configuration storage bucket
    ├── networking/      # VPC, subnets, security groups
    ├── iam/             # Roles and policies
    └── automock-ecs/    # Complete ECS + ALB + Auto-Scaling + TTL
```

## Quick Start

### Prerequisites

- Terraform >= 1.0
- AWS CLI configured
- Valid AWS credentials

### Deploy Infrastructure

```bash
# Initialize Terraform
terraform init

# Review changes
terraform plan

# Deploy
terraform apply

# Get outputs
terraform output
```

### Using from Go CLI

```go
import "github.com/hemantobora/auto-mock/internal/terraform"

manager := terraform.NewManager("user-api", "default")
outputs, err := manager.Deploy(&terraform.DeploymentOptions{
    InstanceSize: "small",
    TTLHours:     4,
})
```

## Modules

### 1. State Backend (`modules/state-backend/`)

Creates centralized S3 bucket and DynamoDB table for Terraform state management.

**Resources:**
- S3 bucket (versioned, encrypted)
- DynamoDB table for state locking

**Cost:** ~$0.50/month

### 2. S3 Configuration (`modules/automock-s3/`)

Manages S3 bucket for storing MockServer expectations and metadata.

**Resources:**
- S3 bucket with versioning
- Bucket policies
- Lifecycle rules

**Cost:** ~$0.05/month

### 3. Networking (`modules/networking/`)

VPC and networking infrastructure.

**Resources:**
- VPC (10.0.0.0/16)
- 2 Public subnets
- 2 Private subnets
- Internet Gateway
- 2 NAT Gateways
- Route tables
- Security groups

**Cost:** ~$65/month (NAT Gateways)

### 4. IAM (`modules/iam/`)

IAM roles and policies with least privilege access.

**Resources:**
- ECS Task Execution Role
- ECS Task Role
- Lambda Cleanup Role
- Auto-Scaling Role

**Cost:** Free

### 5. ECS Infrastructure (`modules/automock-ecs/`)

Complete deployment including ECS, ALB, auto-scaling, and TTL cleanup.

**Resources:**
- ECS Cluster
- ECS Service (Fargate)
- Task Definition (MockServer + Config Loader)
- Application Load Balancer
- Target Groups (API + Dashboard)
- Auto-Scaling policies
- CloudWatch alarms
- EventBridge rule
- Lambda function (TTL cleanup)
- SNS topic (notifications)

**Cost:** ~$60/month base + scaling

## Configuration

### Input Variables

```hcl
# Required
variable "project_name" {
  type = string
}

# Optional with defaults
variable "environment" {
  type    = string
  default = "dev"
}

variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "instance_size" {
  type    = string
  default = "small"  # small, medium, large, xlarge
}

variable "min_tasks" {
  type    = number
  default = 10
}

variable "max_tasks" {
  type    = number
  default = 200
}

variable "ttl_hours" {
  type    = number
  default = 4
}

variable "enable_ttl_cleanup" {
  type    = bool
  default = true
}

variable "custom_domain" {
  type    = string
  default = ""
}

variable "hosted_zone_id" {
  type    = string
  default = ""
}

variable "notification_email" {
  type    = string
  default = ""
}
```

### Outputs

```hcl
output "mockserver_url" {
  value = "http://alb-dns-name" or "https://custom-domain"
}

output "dashboard_url" {
  value = "http://alb-dns-name:8080/mockserver/dashboard"
}

output "config_bucket" {
  value = "automock-project-config-abc123"
}

output "infrastructure_summary" {
  value = {
    # Complete summary object
  }
}
```

## Auto-Scaling Configuration

### Scaling Policies

| Metric | Threshold | Action | Result |
|--------|-----------|--------|--------|
| CPU | 70-80% | +50% | 10 → 15 tasks |
| CPU | 80-90% | +100% | 10 → 20 tasks |
| CPU | 90%+ | +200% | 10 → 30 tasks |
| Memory | 70-80% | +50% | 10 → 15 tasks |
| Memory | 80-90% | +100% | 10 → 20 tasks |
| Memory | 90%+ | +200% | 10 → 30 tasks |
| Requests | 500-1000/min | +50% | 10 → 15 tasks |
| Requests | 1000+/min | +100% | 10 → 20 tasks |

### Scale Down

- Threshold: CPU < 40% for 5 minutes
- Action: Remove 25% of tasks
- Cooldown: 5 minutes

## TTL Cleanup

### How It Works

1. **EventBridge Rule**: Triggers Lambda hourly
2. **Lambda Function**: Checks TTL expiration
3. **Cleanup Sequence**:
   - Scale ECS to 0
   - Delete ECS resources
   - Delete ALB + Target Groups
   - Delete VPC resources
   - Delete S3 bucket
   - Delete logs
   - Self-destruct

### Notifications

Optional SNS notifications for:
- TTL warning (1 hour before expiry)
- Cleanup started
- Cleanup complete
- Cleanup errors

## Cost Estimates

### Base Cost (10 tasks, 24/7)

| Component | Monthly Cost |
|-----------|--------------|
| ECS Fargate | ~$35 |
| ALB | ~$16 |
| NAT Gateways | ~$64 |
| Data Transfer | ~$9 |
| CloudWatch | ~$0.50 |
| S3 + DynamoDB | ~$0.30 |
| **Total** | **~$125** |

### With TTL (Cost Savings)

| TTL Duration | Actual Cost |
|--------------|-------------|
| 4 hours | ~$0.68 |
| 8 hours | ~$1.37 |
| 24 hours | ~$4.11 |
| 1 week | ~$28.77 |

## Security

### IAM Policies
- Least privilege access
- Separate task and execution roles
- No hardcoded credentials

### Networking
- Private subnets for ECS
- Security groups restrict traffic
- ALB terminates SSL
- NAT for outbound only

### Data
- S3 encryption at rest
- Versioning enabled
- Lifecycle policies
- No sensitive data

## Monitoring

### CloudWatch Metrics
- ECS: CPU, Memory, Task Count
- ALB: Response Time, Requests, Errors
- Custom: TTL warnings, Config reloads

### Logs
- `/ecs/automock/{project}/mockserver`
- `/ecs/automock/{project}/config-loader`
- `/aws/lambda/automock-{project}-ttl-cleanup`

### Alarms
- UnhealthyHostCount > 0
- 5XX errors > 10
- CPU > 70% for 10 min
- TTL expiration warning

## Troubleshooting

### Tasks Not Starting

```bash
aws ecs describe-services \
  --cluster automock-{project} \
  --services automock-{project}-service

aws logs tail /ecs/automock/{project}/mockserver --follow
```

### Health Checks Failing

```bash
curl http://{alb-dns}/mockserver/status

aws elbv2 describe-target-health \
  --target-group-arn {arn}
```

### Configuration Issues

```bash
aws s3 ls s3://automock-{project}-config-{suffix}/
aws s3 cp s3://automock-{project}-config-{suffix}/expectations.json -
```

## Development

### Local Testing

```bash
# Validate
terraform validate

# Format
terraform fmt -recursive

# Plan
terraform plan

# Package Lambda
cd modules/automock-ecs/scripts
./package_lambda.sh
```

### Module Testing

Each module can be tested independently:

```bash
cd modules/networking
terraform init
terraform plan -var="name_prefix=test" -var="region=us-east-1"
```

## Contributing

When adding or modifying modules:

1. Follow Terraform best practices
2. Use variables for configurable values
3. Document all outputs
4. Add examples
5. Test thoroughly
6. Update this README

## License

MIT License - See LICENSE file for details

## Support

For issues or questions:
- GitHub Issues: https://github.com/hemantobora/auto-mock/issues
- Documentation: See INFRASTRUCTURE.md
