# Get ECR authorization token
data "aws_ecr_authorization_token" "token" {
  registry_id = data.aws_caller_identity.current.account_id
}

# Configure Docker provider
provider "docker" {
  registry_auth {
    address  = format("%v.dkr.ecr.%v.amazonaws.com", data.aws_caller_identity.current.account_id, var.region)
    username = "AWS"
    password = data.aws_ecr_authorization_token.token.password
  }
}

# ECR Repository for destroy image
resource "aws_ecr_repository" "terraform_destroy" {
  count                = var.enable_ttl_cleanup ? 1 : 0
  name                 = "${local.name_prefix}-terraform-destroy"
  image_tag_mutability = "MUTABLE"
  
  image_scanning_configuration {
    scan_on_push = true
  }
  
  encryption_configuration {
    encryption_type = "AES256"
  }
  
  force_delete = true  # ← IMPORTANT: Allows deletion even with images
  
  tags = merge(local.common_tags, local.ttl_tags, {
    Name = "${local.name_prefix}-terraform-destroy-repo"
  })
}

# ECR Lifecycle policy (optional - keep only latest)
resource "aws_ecr_lifecycle_policy" "terraform_destroy" {
  count      = var.enable_ttl_cleanup ? 1 : 0
  repository = aws_ecr_repository.terraform_destroy[0].name
  
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep only latest image"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 1
      }
      action = {
        type = "expire"
      }
    }]
  })
}

# Build Docker image with Terraform code embedded
resource "docker_image" "terraform_destroy" {
  count = var.enable_ttl_cleanup ? 1 : 0
  name  = "${aws_ecr_repository.terraform_destroy[0].repository_url}:latest"
  
  build {
    context    = path.root  # Project root
    dockerfile = "docker/Dockerfile.terraform-destroy"
    
    tag = ["${aws_ecr_repository.terraform_destroy[0].repository_url}:latest"]
    
    # Build arguments (if needed)
    build_args = {
      TERRAFORM_VERSION = "1.5"
    }
  }
  
  # Force rebuild when Terraform files change
  triggers = {
    terraform_files = sha1(join("", [
      for f in fileset("${path.module}", "*.tf") : 
      filesha1("${path.module}/${f}")
    ]))
  }
}

# Push image to ECR (happens automatically)
resource "docker_registry_image" "terraform_destroy" {
  count         = var.enable_ttl_cleanup ? 1 : 0
  name          = docker_image.terraform_destroy[0].name
  keep_remotely = false  # ← IMPORTANT: Delete from ECR when destroyed
  
  triggers = {
    image_id = docker_image.terraform_destroy[0].image_id
  }
}