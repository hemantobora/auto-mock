# terraform/modules/automock-ecs/main.tf
# AutoMock ECS Fargate + ALB + S3 Infrastructure Module

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
  description = "Infrastructure TTL in hours (0 = no TTL)"
  type        = number
  default     = 4
}

variable "custom_domain" {
  description = "Custom domain for the API (optional)"
  type        = string
  default     = ""
}

variable "hosted_zone_id" {
  description = "Route53 hosted zone ID for custom domain"
  type        = string
  default     = ""
}

variable "instance_size" {
  description = "ECS task size (small, medium, large, xlarge)"
  type        = string
  default     = "small"
}

variable "enable_ttl_cleanup" {
  description = "Enable automatic infrastructure cleanup"
  type        = bool
  default     = true
}

variable "notification_email" {
  description = "Email for TTL notifications"
  type        = string
  default     = ""
}

# Local values
locals {
  name_prefix = "automock-${var.project_name}-${var.environment}"
  
  # ECS task sizing
  task_config = {
    small  = { cpu = 256,  memory = 512 }
    medium = { cpu = 512,  memory = 1024 }
    large  = { cpu = 1024, memory = 2048 }
    xlarge = { cpu = 2048, memory = 4096 }
  }
  
  cpu_units    = local.task_config[var.instance_size].cpu
  memory_units = local.task_config[var.instance_size].memory
  
  common_tags = {
    Project     = "AutoMock"
    ProjectName = var.project_name
    Environment = var.environment
    ManagedBy   = "Terraform"
    CreatedAt   = timestamp()
    Region      = var.region
  }
  
  ttl_tags = var.ttl_hours > 0 && var.enable_ttl_cleanup ? {
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
data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

# Route53 hosted zone (if custom domain is used)
data "aws_route53_zone" "domain" {
  count   = var.custom_domain != "" ? 1 : 0
  zone_id = var.hosted_zone_id
}

# S3 Configuration from external S3 module
locals {
  # Use external S3 bucket if provided, otherwise create a fallback name
  config_bucket_name = var.config_bucket_name != "" ? var.config_bucket_name : "${local.name_prefix}-config-${random_id.suffix.hex}"
  config_bucket_arn  = var.config_bucket_arn != "" ? var.config_bucket_arn : "arn:aws:s3:::${local.config_bucket_name}"
  
  # S3 configuration for ECS tasks
  s3_config = var.s3_bucket_configuration != null ? var.s3_bucket_configuration : {
    bucket_name       = local.config_bucket_name
    expectations_path = "expectations.json"
    metadata_path     = "project-metadata.json"
    versions_prefix   = "versions/"
  }
}

# VPC and Networking
resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-vpc"
  })
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-igw"
  })
}

# Public subnets for ALB
resource "aws_subnet" "public" {
  count = 2

  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.${count.index + 1}.0/24"
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-public-${count.index + 1}"
    Type = "Public"
  })
}

# Private subnets for ECS tasks
resource "aws_subnet" "private" {
  count = 2

  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.${count.index + 10}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-private-${count.index + 1}"
    Type = "Private"
  })
}

# NAT Gateway for private subnet internet access
resource "aws_eip" "nat" {
  count = 2

  domain = "vpc"
  depends_on = [aws_internet_gateway.main]

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-nat-eip-${count.index + 1}"
  })
}

resource "aws_nat_gateway" "main" {
  count = 2

  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public[count.index].id
  depends_on    = [aws_internet_gateway.main]

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-nat-${count.index + 1}"
  })
}

# Route tables
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-public-rt"
  })
}

resource "aws_route_table" "private" {
  count = 2

  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main[count.index].id
  }

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-private-rt-${count.index + 1}"
  })
}

# Route table associations
resource "aws_route_table_association" "public" {
  count = 2

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table_association" "private" {
  count = 2

  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private[count.index].id
}

# Security Groups
resource "aws_security_group" "alb" {
  name_prefix = "${local.name_prefix}-alb-"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-alb-sg"
  })

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_security_group" "ecs_tasks" {
  name_prefix = "${local.name_prefix}-ecs-"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "MockServer API"
    from_port       = 1080
    to_port         = 1080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  ingress {
    description     = "MockServer Dashboard"
    from_port       = 1090
    to_port         = 1090
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-ecs-sg"
  })

  lifecycle {
    create_before_destroy = true
  }
}

# Application Load Balancer
resource "aws_lb" "main" {
  name               = "${local.name_prefix}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  enable_deletion_protection = false

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-alb"
  })
}

# ALB Target Groups
resource "aws_lb_target_group" "mockserver_api" {
  name        = "${local.name_prefix}-api-tg"
  port        = 1080
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"

  health_check {
    enabled             = true
    healthy_threshold   = 2
    unhealthy_threshold = 2
    timeout             = 10
    interval            = 30
    path                = "/mockserver/status"
    matcher             = "200"
    port                = "traffic-port"
    protocol            = "HTTP"
  }

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-api-tg"
  })

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_lb_target_group" "mockserver_dashboard" {
  name        = "${local.name_prefix}-dash-tg"
  port        = 1090
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"

  health_check {
    enabled             = true
    healthy_threshold   = 2
    unhealthy_threshold = 2
    timeout             = 10
    interval            = 30
    path                = "/"
    matcher             = "200"
    port                = "traffic-port"
    protocol            = "HTTP"
  }

  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-dashboard-tg"
  })

  lifecycle {
    create_before_destroy = true
  }
}