# AutoMock ECS Fargate Infrastructure

Complete **ECS Fargate + ALB + S3** infrastructure for deploying MockServer with automatic TTL cleanup, custom domains, and Route53 DNS support.

## 🏗️ **Architecture Overview**

This Terraform module deploys a production-ready MockServer infrastructure on AWS:

- **ECS Fargate** - Serverless container platform
- **Application Load Balancer** - HTTPS/TLS termination
- **VPC + Networking** - Isolated network with public/private subnets
- **S3** - Configuration storage with versioning
- **Route53 + ACM** - Custom domains with SSL certificates
- **EventBridge + Lambda** - Automatic TTL cleanup
- **CloudWatch** - Logging and monitoring
- **Auto Scaling** - CPU/Memory based scaling

## 🚀 **Quick Start**

### 1. **Deploy with AutoMock CLI** (Recommended)

```bash
cd /Users/hemantobora/Desktop/Projects/auto-mock
./automock init
# Select "deploy" when prompted
```

### 2. **Deploy with Terraform Directly**

```bash
cd terraform

# Initialize Terraform
terraform init

# Deploy infrastructure
terraform apply -var="project_name=my-mock-api" \
                -var="environment=dev" \
                -var="ttl_hours=4" \
                -var="notification_email=you@company.com"
```

## 📋 **Configuration Options**

### **Basic Configuration**
```hcl
module "automock_ecs" {
  source = "./modules/automock-ecs"
  
  project_name     = "my-api"           # Required: Your project name
  environment      = "dev"              # dev, staging, prod
  region           = "us-east-1"        # AWS region
  instance_size    = "small"            # small, medium, large, xlarge
}
```

### **TTL Auto-Cleanup**
```hcl
ttl_hours          = 4                  # Auto-delete after 4 hours
enable_ttl_cleanup = true               # Enable automatic cleanup
notification_email = "you@company.com"  # Get notified before cleanup
```

### **Custom Domain**
```hcl
custom_domain   = "api.mycompany.com"   # Your domain
hosted_zone_id  = "Z1234567890ABC"      # Route53 hosted zone ID
```

## 🌐 **Domain Options**

### **Auto-Generated Domain** (Default)
- **Format**: `project-env-random.region.elb.amazonaws.com`
- **TLS**: Automatic AWS ALB certificate
- **Cost**: Free
- **Setup**: Instant

### **Custom Domain** 
- **Requirements**: Domain ownership + Route53 hosted zone
- **TLS**: Automatic ACM certificate
- **DNS**: Automatic Route53 record creation
- **Validation**: Automatic DNS validation

## ⏰ **TTL Auto-Cleanup**

Prevents unexpected AWS costs by automatically destroying infrastructure:

```hcl
ttl_hours = 4    # Options: 2, 4, 8, 12, 24, 48, 72 hours
```

**Cleanup Process:**
1. **15 min before**: Email notification (if configured)
2. **At TTL expiry**: Automatic resource deletion
3. **After cleanup**: Completion notification

**Resources Cleaned:**
- ECS Service & Cluster
- Application Load Balancer  
- VPC & Networking (subnets, NAT gateways, etc.)
- S3 Configuration Bucket
- SSL Certificates
- Route53 DNS Records

## 💰 **Cost Optimization**

### **Development** (~$20/month)
```hcl
instance_size = "small"    # 256 CPU, 512MB RAM
environment   = "dev"      # Minimal resources
ttl_hours     = 4          # Auto-cleanup after 4 hours
```

### **Production** (~$80/month)
```hcl
instance_size = "large"    # 1024 CPU, 2GB RAM  
environment   = "prod"     # High availability
ttl_hours     = 0          # No auto-cleanup
```

## 📊 **Outputs**

After deployment, Terraform provides:

```hcl
# Access URLs
mockserver_url = "https://my-api-dev-abc123.us-east-1.elb.amazonaws.com"
dashboard_url  = "https://my-api-dev-abc123.us-east-1.elb.amazonaws.com/mockserver/dashboard"

# Infrastructure Details
config_bucket  = "automock-my-api-dev-1a2b3c4d"
ecs_cluster    = "automock-my-api-dev-cluster"
vpc_id         = "vpc-1234567890abcdef0"
```

## 🛠️ **Management Commands**

### **View Logs**
```bash
aws logs tail /ecs/automock-my-api-dev/mockserver --follow
```

### **Scale Service**
```bash
# Scale up
aws ecs update-service --cluster automock-my-api-dev-cluster \
                       --service automock-my-api-dev-mockserver \
                       --desired-count 3

# Scale down  
aws ecs update-service --cluster automock-my-api-dev-cluster \
                       --service automock-my-api-dev-mockserver \
                       --desired-count 1
```

### **Update Configuration**
```bash
# Upload new expectations to S3
aws s3 cp new-expectations.json s3://automock-my-api-dev-1a2b3c4d/expectations.json

# Restart ECS service to reload
aws ecs update-service --cluster automock-my-api-dev-cluster \
                       --service automock-my-api-dev-mockserver \
                       --force-new-deployment
```

### **Extend TTL**
```bash
# Extend by 2 hours (modify EventBridge rule)
aws events put-rule --name automock-my-api-dev-ttl-check \
                    --schedule-expression "rate(2 hours)"
```

### **Manual Cleanup**
```bash
terraform destroy -auto-approve
```

## 🔧 **Advanced Configuration**

### **Auto Scaling**
```hcl
auto_scaling_min_capacity     = 1     # Minimum tasks
auto_scaling_max_capacity     = 10    # Maximum tasks  
cpu_utilization_threshold     = 70    # CPU scale trigger
memory_utilization_threshold  = 80    # Memory scale trigger
```

### **Networking**
```hcl
vpc_cidr           = "10.0.0.0/16"    # VPC CIDR block
availability_zones = 2                # Number of AZs
enable_nat_gateway = true             # NAT for private subnets
```

### **Security**
```hcl
enable_waf                   = true   # AWS WAF protection
ssl_policy                   = "ELBSecurityPolicy-TLS-1-2-2017-01"
enable_deletion_protection   = false  # ALB deletion protection
```

### **Monitoring**
```hcl
enable_container_insights = true      # CloudWatch Container Insights
log_retention_days       = 7          # Log retention period
```

## 📁 **Module Structure**

```
terraform/
├── main.tf                           # Main configuration
├── modules/automock-ecs/
│   ├── main.tf                       # Core infrastructure
│   ├── ecs.tf                        # ECS Fargate configuration
│   ├── ssl.tf                        # SSL/TLS and Route53
│   ├── ttl.tf                        # Auto-cleanup with EventBridge
│   ├── variables.tf                  # Input variables
│   ├── outputs.tf                    # Module outputs
│   └── scripts/
│       └── ttl_cleanup.py           # Python cleanup Lambda
```

## 🔐 **Security Features**

- **TLS Encryption**: All traffic encrypted in transit
- **Private Networking**: ECS tasks in private subnets
- **IAM Least Privilege**: Minimal required permissions
- **VPC Isolation**: Dedicated network per deployment
- **Security Groups**: Restrictive inbound/outbound rules
- **S3 Encryption**: Configuration encrypted at rest

## 🌍 **Multi-Region Support**

```hcl
# Deploy to different regions
module "us_east_1" {
  source = "./modules/automock-ecs"
  region = "us-east-1"
  project_name = "my-api-east"
}

module "eu_west_1" {
  source = "./modules/automock-ecs" 
  region = "eu-west-1"
  project_name = "my-api-eu"
}
```

## 🚨 **Troubleshooting**

### **Common Issues**

**ECS Service Won't Start**
```bash
# Check service events
aws ecs describe-services --cluster CLUSTER_NAME --services SERVICE_NAME

# Check task logs
aws logs tail /ecs/automock-PROJECT-ENV/mockserver --follow
```

**Domain Not Resolving**
```bash
# Check certificate status
aws acm describe-certificate --certificate-arn CERT_ARN

# Check Route53 records
aws route53 list-resource-record-sets --hosted-zone-id ZONE_ID
```

**TTL Cleanup Failed**
```bash
# Check Lambda logs
aws logs tail /aws/lambda/automock-PROJECT-ENV-ttl-cleanup --follow

# Manual cleanup
terraform destroy -auto-approve
```

## 📚 **Next Steps**

1. **Test Your API**: Use the provided URLs to test your MockServer
2. **Upload Expectations**: Use the S3 bucket to store mock configurations
3. **Monitor**: Check CloudWatch logs and metrics
4. **Scale**: Adjust ECS service capacity based on load
5. **Extend**: Modify TTL or disable auto-cleanup for production

## 🤝 **Support**

- **Documentation**: Check `/GETTING_STARTED.md` in your AutoMock project
- **Issues**: File issues in your AutoMock repository
- **AWS Costs**: Monitor AWS billing dashboard for usage
- **Security**: Review AWS security best practices

---

**Built with ❤️ using Terraform + AWS ECS Fargate + MockServer**