# AutoMock - Getting Started Guide

Welcome to AutoMock! This guide will help you set up and start using AutoMock to generate and deploy mock API servers.

## 📋 Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Quick Start](#quick-start)
4. [Core Concepts](#core-concepts)
5. [Generation Modes](#generation-modes)
6. [Working with Projects](#working-with-projects)
7. [Infrastructure Deployment](#infrastructure-deployment)
8. [Next Steps](#next-steps)

## Prerequisites

> Cloud provider support: AutoMock currently supports AWS only. GCP and Azure
> integrations are planned but not available yet.

### Required
- **Go 1.22+** (for building from source)
- **AWS Account** with configured credentials (currently the only supported cloud)
- **AWS CLI** installed and configured

### Optional (for AI generation)
- **Anthropic API Key** (Claude) - for AI-powered generation
- **OpenAI API Key** (GPT-4) - alternative AI provider

### AWS Permissions Required
Your AWS credentials need the following permissions:
- S3: Bucket operations, object read/write
- ECS: Cluster, service, task management
- EC2: VPC, subnet, security group operations
- IAM: Role creation and management
- CloudWatch: Logs and metrics
- Application Load Balancer: Creation and management

## Installation

### 1. Clone the Repository
```bash
git clone https://github.com/hemantobora/auto-mock.git
cd auto-mock
```

### 2. Build AutoMock
```bash
# Make the build script executable
chmod +x build.sh

# Build the binary
./build.sh
```

This creates an `automock` binary in the current directory.

### 3. Configure AWS Credentials (AWS-only for now)
```bash
# Option 1: Use AWS CLI
aws configure

# Option 2: Set environment variables
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"

# Option 3: Use AWS profiles
# Edit ~/.aws/credentials and add profiles
```

### 4. Configure AI Provider (Optional)
```bash
# For Claude (Anthropic)
export ANTHROPIC_API_KEY="sk-ant-..."

# OR for GPT-4 (OpenAI)
export OPENAI_API_KEY="sk-..."
```

Note: the CLI also accepts a provider override when running `automock init`.
Use `--provider <anthropic|openai|template>` to preselect the AI provider and
skip the interactive provider-selection prompt. The tool will still verify that
an appropriate API key environment variable is set (and will prompt for it if
missing).

## Quick Start

### Your First Mock API in 3 Minutes

```bash
# Step 1: Initialize a project (interactive mode)
./automock init

# Step 2: Follow the prompts
# - Create new project or select existing
# - Choose generation mode (describe, interactive, collection, upload)
# - Generate your mock expectations
# - Save to cloud storage

# Step 3: Deploy infrastructure (optional)
./automock deploy --project your-project-name

# Step 4: Access your mock API
# The deployment will output the ALB URL
# Example: http://automock-your-project-1234567890.us-east-1.elb.amazonaws.com
```

### Example: AI-Powered Generation
```bash
# Export your API key
export ANTHROPIC_API_KEY="sk-ant-..."

# Create project with AI generation
./automock init --project user-api --provider anthropic

# When prompted, describe your API:
# "User management system with registration, login, profile CRUD,
#  password reset, and admin functions. Include authentication."

# AI generates complete expectations with:
# - All CRUD endpoints
# - Authentication flows
# - Error responses (400, 401, 403, 404, 500)
# - Realistic test data
```

### Example: Import from Postman
```bash
# Import Postman collection
./automock init \
  --project api-mock \
  --collection-file ./my-api.postman_collection.json \
  --collection-type postman

# Deploy immediately
./automock deploy --project api-mock
```

## Core Concepts

### Projects
A project is a named collection of mock expectations and infrastructure:
- Stored in S3 (bucket name: `automock-{project-name}-config-{suffix}`)
- Contains expectations (MockServer configuration)
- Can have deployed infrastructure (ECS, ALB, etc.)
- Managed independently

### Expectations
Expectations define how MockServer responds to requests:
```json
{
  "httpRequest": {
    "method": "GET",
    "path": "/api/users/{id}"
  },
  "httpResponse": {
    "statusCode": 200,
    "headers": {
      "Content-Type": ["application/json"]
    },
    "body": {
      "id": "{{pathParameters.id.0}}",
      "name": "John Doe",
      "email": "john@example.com"
    }
  },
  "priority": 100
}
```

### Infrastructure
Optional cloud deployment (AWS-only) consisting of:
- **AWS ECS Fargate Cluster** - Runs MockServer containers
- **AWS Application Load Balancer (ALB)** - Public access point
- **AWS Auto Scaling** - 10-200 tasks based on load
- **Amazon CloudWatch** - Monitoring and logging
- **Amazon S3** - Configuration storage

## Generation Modes

### 1. 🤖 AI-Powered (describe)
Generate expectations from natural language descriptions.

**Best for:** Quick prototyping, comprehensive APIs

**Example:**
```bash
./automock init --project user-api --provider anthropic
```

Note: when you pass `--provider anthropic` (or `openai` / `template`) the init
flow will preselect that provider for AI generation and avoid asking you to
choose a provider interactively. It will still ensure the provider's API key
is set (e.g., `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`) and will prompt you to
enter it if necessary.

**Prompt examples:**
- "REST API for a blog: posts, comments, users, authentication"
- "E-commerce API: products, cart, checkout, orders, payments"
- "Banking API with accounts, transactions, transfers"

**AI generates:**
- Complete endpoint definitions
- Realistic response data
- Error handling
- Multiple scenarios per endpoint
- Request validation rules

### 2. 🔧 Interactive Builder (interactive)
Step-by-step guided builder for manual creation.

**Best for:** Precise control, learning MockServer

**7-Step Process:**
1. **Basic Info** - Description, priority
2. **Request Match** - Method, path, query params, headers
3. **Response Config** - Status code, headers, body
4. **Features** - Delays, caching, compression
5. **Connection** - Socket options, keep-alive
6. **Limits** - Rate limiting, request count
7. **Review** - Validate and confirm

**Features:**
- Path parameters & wildcards (`/users/{id}`)
- Query string matching
- Header validation
- Request body matching (exact, partial, regex)
- Response templates with variables
- Progressive response delays
- Custom error responses

### 3. 📦 Collection Import (collection)
Import from existing API collections.

**Supported formats:**
- Postman Collection v2.1
- Bruno Collection
- Insomnia Workspace (beta)

**Best for:** Converting existing tests to mocks

**Features:**
- Executes APIs sequentially
- Variable resolution
- Pre/post-script processing
- Interactive matching configuration (guided; no automatic scenario inference)
- Auth variation handling (via separate expectations)
- Auto-incremented priorities to avoid collisions

**Example:**
```bash
./automock init \
  --project my-api \
  --collection-file ./api.postman_collection.json \
  --collection-type postman
```

### 4. 📤 Upload Mode (upload)
Upload pre-built MockServer JSON files.

**Best for:** Migrating from existing MockServer, team-shared configs

**Example:**
```bash
./automock init --project my-api
# Select "upload" mode
# Paste or upload your expectations.json
```

## Working with Projects

### Create a New Project
```bash
# Interactive mode
./automock init

# CLI mode
./automock init --project my-new-api
```

### View Expectations
```bash
./automock init --project my-api
# Select action: "view"
```

### Add New Expectations
```bash
./automock init --project my-api
# Select action: "add"
# Choose generation mode
```

### Edit Existing Expectations
```bash
./automock init --project my-api
# Select action: "edit"
# Select endpoint to modify
# Update configuration
```

### Remove Expectations
```bash
./automock init --project my-api
# Select action: "remove"
# Select expectations to remove (or "all")
```

### Replace All Expectations
```bash
./automock init --project my-api
# Select action: "replace"
# Generate new expectations
```

### Delete Project
```bash
# Via init command (interactive)
./automock init --project my-api
# Select action: "delete"

# OR via destroy command (infrastructure only)
./automock destroy --project my-api
```

### Download Expectations
```bash
./automock init --project my-api
# Select action: "download"
# File saved to: {project-name}-expectations.json
```

## Infrastructure Deployment

### Deploy Infrastructure
```bash
# Deploy for existing project
./automock deploy --project my-api

# Skip confirmation prompt
./automock deploy --project my-api --skip-confirmation

# Use specific AWS profile
./automock deploy --project my-api --profile production
```

### Check Status
```bash
# Basic status
./automock status --project my-api

# Detailed status (with metrics)
./automock status --project my-api --detailed
```

### Access Your Mock API
After deployment (on AWS), you'll get URLs for:
- **API Endpoint**: `http://automock-{project}-{id}.{region}.elb.amazonaws.com`
- **Dashboard**: `http://automock-{project}-{id}.{region}.elb.amazonaws.com/mockserver/dashboard`

### Example API Call
```bash
# Assuming you have a GET /api/users endpoint
curl http://your-alb-url/api/users

# With authentication header
curl -H "Authorization: Bearer token123" \
     http://your-alb-url/api/users/1
```

### Destroy Infrastructure
```bash
# Interactive confirmation
./automock destroy --project my-api

# Force destroy (skip prompts)
./automock destroy --project my-api --force
```

**Note:** This destroys infrastructure but preserves the S3 bucket and expectations by default. To delete everything, use the "delete" action in `automock init`.

## Advanced Features

### Progressive Responses
Simulate degrading or improving performance:
```
Request 1: 100ms delay
Request 2: 150ms delay  (+50ms)
Request 3: 200ms delay  (+50ms)
...
Request N: 500ms delay  (cap reached)
```

Configure during interactive build:
- Base: Starting delay (e.g., 100ms)
- Step: Increment per request (e.g., 50ms)
- Cap: Maximum delay (e.g., 500ms)

### Response Templates
Use variables in response bodies:
```json
{
  "id": "$!uuid",
  "timestamp": "$!now_epoch",
  "requestId": "$!request.headers['x-request-id'][0]",
  "path": "$!request.path",
  "randomValue": "$!rand_int_100"
}
```

Available variables:
- `$!uuid` - Random UUID
- `$!now_epoch` - Current timestamp
- `$!rand_int_100` - Random integer (0-100)
- `$!rand_bytes_64` - Random 64 bytes (base64)
- `$!request.*` - Request properties

### GraphQL Support
Create expectations for GraphQL endpoints with:
- Query matching
- Operation name matching
- Optional variables matching (exact)

### Load Testing with Locust
Generate load testing scripts from collections:
```bash
./automock load \
  --collection-file api.json \
  --collection-type postman \
  --dir ./load-tests

cd load-tests
./run_locust_ui.sh
# Open http://localhost:8089
```

## Monitoring & Debugging

### CloudWatch Logs
```bash
# MockServer logs
aws logs tail /ecs/automock/{project}/mockserver --follow

# Config loader logs
aws logs tail /ecs/automock/{project}/config-loader --follow
```

### Check ECS Service
```bash
aws ecs describe-services \
  --cluster automock-{project} \
  --services automock-{project}-service
```

### Check ALB Health
```bash
aws elbv2 describe-target-health \
  --target-group-arn {arn-from-outputs}
```

### Verify S3 Expectations
```bash
aws s3 ls s3://automock-{project}-config-{suffix}/
aws s3 cp s3://automock-{project}-config-{suffix}/expectations.json -
```

## Cost Management

### Estimated Costs (10 tasks, 24/7)
- ECS Fargate: ~$35/month
- ALB: ~$16/month
- NAT Gateways: ~$64/month
- Data Transfer: ~$9/month
- CloudWatch: ~$0.50/month
- S3: ~$0.30/month
- **Total**: ~$125/month

> Note: These are rough, region-dependent estimates and will vary with traffic, data transfer, and log volume. Please validate with the AWS Pricing Calculator for your account and region.

### Cost Optimization
1. **Destroy when idle** - `./automock destroy --project name`
2. **Adjust task count** - Modify min/max in Terraform
3. **Use smaller regions** - Some regions have lower costs

## Troubleshooting

### "No AI provider configured"
```bash
# Export your API key
export ANTHROPIC_API_KEY="sk-ant-..."
# OR
export OPENAI_API_KEY="sk-..."
```

### "AWS credentials not found"
```bash
# Configure AWS
aws configure

# OR use environment variables
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"
```

### "Failed to create S3 bucket"
- Bucket names must be globally unique
- Try using `--profile` flag for different account
- Verify S3 permissions in IAM policy

### "ECS tasks not starting"
1. Check CloudWatch logs
2. Verify expectations in S3
3. Check task definition CPU/memory
4. Review IAM role permissions

### "Health checks failing"
1. Verify MockServer started: Check logs
2. Test health endpoint: `/mockserver/status`
3. Check ALB target group configuration
4. Verify security group rules

## Next Steps

### Learn More
- [README.md](README.md) - Full project overview
- [terraform/README.md](terraform/README.md) - Infrastructure details
- Run `./automock help` - Comprehensive CLI reference

### Common Workflows

**Development Workflow:**
```bash
# 1. Create & test locally
./automock init --project dev-api
# (generate expectations, select "local" to test)

# 2. Deploy to cloud
./automock deploy --project dev-api

# 3. Test in cloud
curl http://your-alb-url/api/test

# 4. Iterate (add/edit/remove expectations)
./automock init --project dev-api

# 5. Clean up when done
./automock destroy --project dev-api
```

**Team Collaboration:**
```bash
# Team member 1: Create & deploy
./automock init --project shared-api
./automock deploy --project shared-api

# Team member 2: View & modify
./automock init --project shared-api
# (project auto-detected from S3)
```

**CI/CD Integration:**
```bash
# In your CI pipeline
export AWS_PROFILE=ci
export ANTHROPIC_API_KEY=$CLAUDE_API_KEY

# Deploy mock for testing
./automock deploy --project test-api --skip-confirmation

# Run integration tests
npm run test:integration

# Tear down
./automock destroy --project test-api --force
```

### Get Help
- **GitHub Issues**: https://github.com/hemantobora/auto-mock/issues
- **Documentation**: Check README.md and terraform/README.md
- **Detailed Help**: Run `./automock help`

## Best Practices

1. **Project Naming** - Use descriptive names: `user-service-mock`, `payment-api-dev`
2. **Priorities** - Use 100, 200, 300... for main scenarios, 10-90 for edge cases
3. **Error Responses** - Always include 400, 401, 404, 500 responses
4. **Response Templates** - Use variables for dynamic data
5. **Destroy Unused Infrastructure** - Avoid unnecessary costs
6. **Version Control** - Export and commit expectations to git
7. **Documentation** - Add descriptions to expectations
8. **Testing** - Test with `automock status` before full integration

---

Ready to create your first mock API? Run `./automock init` and follow the interactive prompts!

For comprehensive command reference, run: `./automock help`
