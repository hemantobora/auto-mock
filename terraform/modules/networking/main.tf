# terraform/modules/networking/main.tf
# VPC and Networking Resources for AutoMock

terraform {
  required_version = ">= 1.0"
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  vpc_cidr = var.vpc_cidr
  az_count = var.az_count
  
  common_tags = merge(
    var.tags,
    {
      Module    = "networking"
      ManagedBy = "Terraform"
    }
  )
}

# VPC
resource "aws_vpc" "main" {
  cidr_block           = local.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(
    local.common_tags,
    {
      Name = "${var.name_prefix}-vpc"
    }
  )
}

# Internet Gateway
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = merge(
    local.common_tags,
    {
      Name = "${var.name_prefix}-igw"
    }
  )
}

# Public Subnets (Multi-AZ for high availability)
resource "aws_subnet" "public" {
  count = local.az_count

  vpc_id                  = aws_vpc.main.id
  cidr_block              = cidrsubnet(local.vpc_cidr, 8, count.index)
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true

  tags = merge(
    local.common_tags,
    {
      Name = "${var.name_prefix}-public-${count.index + 1}"
      Type = "Public"
      Tier = "public"
    }
  )
}

# Route Table (Public)
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = merge(
    local.common_tags,
    {
      Name = "${var.name_prefix}-public-rt"
    }
  )
}

# Route Table Associations
resource "aws_route_table_association" "public" {
  count = local.az_count

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# Security Group - ALB
resource "aws_security_group" "alb" {
  name_prefix = "${var.name_prefix}-alb-"
  description = "Security group for AutoMock ALB"
  vpc_id      = aws_vpc.main.id

  tags = merge(
    local.common_tags,
    {
      Name = "${var.name_prefix}-alb-sg"
    }
  )

  lifecycle {
    create_before_destroy = true
  }
}

# ALB Security Group Rules - HTTPS
resource "aws_security_group_rule" "alb_https_ingress" {
  type              = "ingress"
  description       = "HTTPS from anywhere"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.alb.id
}

# ALB Security Group Rules - HTTP (conditional)
resource "aws_security_group_rule" "alb_http_ingress" {
  count = var.allow_http ? 1 : 0

  type              = "ingress"
  description       = "HTTP from anywhere"
  from_port         = 80
  to_port           = 80
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.alb.id
}

# ALB Security Group Rules - Egress
resource "aws_security_group_rule" "alb_egress" {
  type              = "egress"
  description       = "All outbound traffic"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.alb.id
}

# Security Group - ECS Tasks
resource "aws_security_group" "ecs_tasks" {
  name_prefix = "${var.name_prefix}-ecs-"
  description = "Security group for AutoMock ECS tasks"
  vpc_id      = aws_vpc.main.id

  tags = merge(
    local.common_tags,
    {
      Name = "${var.name_prefix}-ecs-sg"
    }
  )

  lifecycle {
    create_before_destroy = true
  }
}

# ECS Security Group Rules - MockServer port from ALB
resource "aws_security_group_rule" "ecs_mockserver_ingress" {
  type                     = "ingress"
  description              = "MockServer traffic from ALB"
  from_port                = 1080
  to_port                  = 1080
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.alb.id
  security_group_id        = aws_security_group.ecs_tasks.id
}

# ECS Security Group Rules - Egress
resource "aws_security_group_rule" "ecs_egress" {
  type              = "egress"
  description       = "All outbound traffic"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.ecs_tasks.id
}

# VPC Endpoint for S3 (optional, for better performance and no data transfer costs)
resource "aws_vpc_endpoint" "s3" {
  count = var.enable_s3_endpoint ? 1 : 0

  vpc_id            = aws_vpc.main.id
  service_name      = "com.amazonaws.${var.region}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = [aws_route_table.public.id]

  tags = merge(
    local.common_tags,
    {
      Name = "${var.name_prefix}-s3-endpoint"
    }
  )
}
