# AutoMock — Getting Started Guide

Welcome to AutoMock! This guide will help you set up and start using AutoMock to generate and deploy mock API servers.

## 📋 Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Quick Start](#quick-start)
4. [Core Concepts](#core-concepts)
5. [Generation Modes](#generation-modes)
6. [Working with Projects](#working-with-projects)
7. [Infrastructure Deployment](#infrastructure-deployment)
8. [Load Testing](#load-testing)
9. [Advanced Features](#advanced-features)
10. [Monitoring & Debugging](#monitoring--debugging)
11. [Cost Management](#cost-management)
12. [Troubleshooting](#troubleshooting)
13. [Best Practices](#best-practices)

---

## Prerequisites

> Cloud provider support: AutoMock currently supports AWS only. GCP and Azure integrations are planned but not available yet.

### Required
- **AWS Account** with configured credentials
- **AWS CLI** installed and configured

### Optional (for AI generation)
- **Anthropic API Key** (Claude)
- **OpenAI API Key** (GPT-4)

### AWS Permissions Required
Your AWS credentials need the following permissions:
- S3: bucket operations, object read/write
- ECS: cluster, service, task management
- EC2: VPC, subnet, security group operations
- IAM: role creation and management
- CloudWatch: logs and metrics
- Application Load Balancer: creation and management
- ACM: certificate creation and validation (if using custom domain)
- Route53: hosted zone and record management (if using custom domain)

---

## Installation

### Option A — Homebrew (macOS / Linux)

```bash
brew tap hemantobora/tap
brew install automock
automock --version
```

Or directly: `brew install hemantobora/tap/automock`

### Option B — Scoop (Windows)

```bash
scoop bucket add hemantobora https://github.com/hemantobora/scoop-bucket
scoop install automock
automock --version
```

### Option C — Download release binary

1. Go to the [Releases page](https://github.com/hemantobora/auto-mock/releases)
2. Download the archive for your OS/arch (e.g., `automock_darwin_arm64.tar.gz`)
3. Extract and place the binary on your PATH:

```bash
tar -xzf automock_*.tar.gz
sudo mv automock /usr/local/bin/
automock --version
```

### Option D — Build from source

Requires Go 1.22+:

```bash
git clone https://github.com/hemantobora/auto-mock.git
cd auto-mock
go build -o automock ./cmd/auto-mock
```

### Configure AWS Credentials

```bash
# Option 1: AWS CLI
aws configure

# Option 2: Environment variables
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"

# Option 3: AWS profiles (use --profile flag)
automock deploy --project my-api --profile production
```

### Configure AI Provider (Optional)

```bash
# For Claude (Anthropic)
export ANTHROPIC_API_KEY="sk-ant-..."

# OR for GPT-4 (OpenAI)
export OPENAI_API_KEY="sk-..."
```

Use `--provider <anthropic|openai|template>` to preselect the AI provider. The tool verifies the relevant API key is set and prompts for it if missing.

---

## Quick Start

```bash
# Step 1: Initialize a project
automock init

# Step 2: Follow the prompts
# - Create new project or select existing
# - Choose generation mode (describe, interactive, collection, upload)
# - Generate your mock expectations
# - Save to cloud storage

# Step 3: Deploy infrastructure
automock deploy --project your-project-name

# Step 4: Access your mock API
# The deployment outputs the ALB URL, e.g.:
# https://automock-your-project-1234567890.us-east-1.elb.amazonaws.com
```

### Example: AI-Powered Generation

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
automock init --project user-api --provider anthropic

# When prompted, describe your API:
# "User management system with registration, login, profile CRUD,
#  password reset, and admin functions. Include authentication."
```

### Example: Import from Postman

```bash
automock init \
  --project api-mock \
  --collection-file ./my-api.postman_collection.json \
  --collection-type postman

automock deploy --project api-mock
```

---

## Core Concepts

### Projects
A project is a named collection of mock expectations and infrastructure:
- Stored in S3 (bucket: `automock-{project-name}-config-{suffix}`)
- Contains expectations (MockServer configuration)
- Can have deployed infrastructure (ECS, ALB, etc.)
- Managed independently per project

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
Optional cloud deployment (AWS) consisting of:
- **ECS Fargate Cluster** — Runs MockServer containers
- **Application Load Balancer (ALB)** — Public access point (HTTPS)
- **Auto Scaling** — 10–200 tasks based on load (configurable)
- **CloudWatch** — Monitoring and logging
- **S3** — Configuration storage

---

## Generation Modes

### 1. 🤖 AI-Powered (describe)

Generate expectations from natural language descriptions.

**Best for:** Quick prototyping, comprehensive APIs

```bash
automock init --project user-api --provider anthropic
```

**Prompt examples:**
- "REST API for a blog: posts, comments, users, authentication"
- "E-commerce API: products, cart, checkout, orders, payments"
- "Banking API with accounts, transactions, transfers"

**AI generates:** Complete endpoint definitions, realistic response data, error handling, multiple scenarios per endpoint, request validation rules.

### 2. 🔧 Interactive Builder (interactive)

Step-by-step guided builder for manual creation.

**Best for:** Precise control, learning MockServer

```bash
automock init --project my-api
# Select: interactive
```

**7-Step Process:**
1. **Basic Info** — Description, priority, tags
2. **Request Matching** — Method, path, query params, headers
3. **Response Configuration** — Status code, headers, body templates
4. **Advanced Features** — Delays, caching, compression
5. **Connection Options** — Socket config, keep-alive
6. **Response Limits** — Serve unlimited times or expire after N requests
7. **Review & Confirm** — Validate before saving

**Supported matching:** path parameters (`/users/{id}`), wildcards, query strings, header validation, request body (exact, partial, regex, JSONPath).

### 3. 📦 Collection Import (collection)

Import from existing API collections.

**Best for:** Converting existing tests to mocks

**Supported formats:**
- Postman Collection v2.1
- Bruno Collection
- Insomnia Workspace

```bash
automock init \
  --project my-api \
  --collection-file ./api.postman_collection.json \
  --collection-type postman
```

**Features:** sequential API execution with variable resolution, pre/post-script processing, interactive matching configuration (guided; no automatic scenario inference), auth variation handling, auto-incremented priorities.

### 4. 📤 Upload Mode (upload)

Upload pre-built MockServer JSON files directly.

**Best for:** Migrating from existing MockServer setups, team-shared configs

```bash
automock init --project my-api
# Select: upload
# Paste or upload your expectations.json
```

---

## Working with Projects

```bash
# All project actions are available via automock init --project <name>
# and selecting the action from the menu:

automock init --project my-api
```

| Action | Description |
|--------|-------------|
| `view` | List all expectations |
| `add` | Add new expectations (any generation mode) |
| `edit` | Edit specific expectations |
| `remove` | Remove selected expectations |
| `replace` | Replace all expectations with a new set |
| `download` | Save to `{project-name}-expectations.json` |
| `delete` | Tear down project and all infrastructure |

---

## Infrastructure Deployment

### Deploy

```bash
# Deploy (prompts for infrastructure options)
automock deploy --project my-api

# Skip confirmation
automock deploy --project my-api --skip-confirmation

# Use a specific AWS profile
automock deploy --project my-api --profile production
```

The deploy flow prompts for:
- AWS region and task sizing
- BYO networking (optional): existing VPC, subnets, IGW, NAT, IAM roles, security groups
- Custom domain (optional): ACM certificate + Route53 hosted zone
- Private ALB (optional): internal load balancer for VPC-internal clients

### Check Status

```bash
automock status --project my-api
automock status --project my-api --detailed
```

### Access Your Mock API

After deployment you'll receive:
- **API Endpoint**: `https://automock-{project}-{id}.{region}.elb.amazonaws.com`
- **Dashboard**: `https://automock-{project}-{id}.{region}.elb.amazonaws.com/mockserver/dashboard`

```bash
curl https://your-alb-url/api/users
curl -H "Authorization: Bearer token123" https://your-alb-url/api/users/1
```

### Destroy Infrastructure

```bash
# Interactive confirmation (prompts for project name + yes/no)
automock destroy --project my-api

# Skip confirmation
automock destroy --project my-api --force
```

Destroy removes all cloud infrastructure. The S3 bucket and expectations are preserved unless you also run the `delete` action via `automock init`.

---

## Load Testing

### Generate a Bundle Locally

```bash
automock load \
  --collection-file api.json \
  --collection-type postman \
  --dir ./load-tests

# Optional flags:
#   --headless       Skip interactive prompts, use defaults
#   --distributed    Generate master/worker helper scripts
```

**Generated files:**
- `locustfile.py` — Test scenarios
- `requirements.txt` — Python dependencies
- `run_locust_ui.sh` / `.ps1` — Start with web UI (http://localhost:8089)
- `run_locust_headless.sh` / `.ps1` — Run without UI
- `run_locust_master.sh` / `.ps1` — Distributed master
- `run_locust_worker.sh` / `.ps1` — Distributed worker

### Upload / Manage Bundles

```bash
# Upload bundle to S3 (required before managed AWS deploy)
automock load --project my-api --upload --dir ./load-tests

# Download active bundle for editing
automock load --project my-api --download --dir ./work

# Roll back and remove the active bundle pointer
automock load --project my-api --delete-pointer

# Delete all load test artifacts for a project
automock load --project my-api --purge-all
```

### Deploy Managed Locust on AWS

After uploading a bundle, deploy the Locust cluster:

```bash
automock deploy --project my-api
# → If a load test bundle exists and is not yet deployed, prompts to deploy Locust infrastructure
# → Provisions ECS Fargate master + workers, public ALB for Locust UI, Cloud Map service discovery
```

**Scale workers:**
```bash
automock deploy --project my-api
# → When already deployed, prompts for new worker count
```

**Destroy Locust infrastructure:**
```bash
automock destroy --project my-api
# → Select: mocks, loadtest, or both
```

**Variable substitution in locustfile:**
- `${env.VAR}` — Expanded at load-time
- `${data.<field>}` / `${user.id|index}` — Expanded at runtime in path, headers, params, body

---

## Advanced Features

### Progressive Delays

Simulate degrading performance:
```
Request 1: 100ms → Request 2: 150ms → … → capped at 500ms
```
Configure during interactive build: base delay, step increment, cap.

### Response Templates

Dynamic values in response bodies:
```json
{
  "id": "$!uuid",
  "timestamp": "$!now_epoch",
  "requestId": "$!request.headers['x-request-id'][0]",
  "path": "$!request.path",
  "randomValue": "$!rand_int_100"
}
```

Available variables: `$!uuid`, `$!now_epoch`, `$!rand_int_100`, `$!rand_bytes_64`, `$!request.path`, `$!request.method`, `$!request.headers[...]`, `$!request.pathParameters[...]`, `$!request.queryStringParameters[...]`

### GraphQL Support

Match GraphQL requests by query string content, operation name, and optional variables (exact match). No schema validation.

---

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

---

## Cost Management

### Estimated Costs (10 tasks, 24/7)

Both `min_tasks` and `max_tasks` are configurable at deploy time. Using BYO networking with an existing NAT eliminates the NAT Gateway cost.

| Component | Monthly Cost |
|-----------|--------------|
| ECS Fargate (0.25 vCPU, 0.5 GB) | ~$35 |
| Application Load Balancer | ~$16 |
| NAT Gateway | ~$32 |
| Data Transfer | ~$9 |
| CloudWatch | ~$0.50 |
| S3 | ~$0.30 |
| **Total** | **~$93** |

> Rough estimates; varies by region, traffic, and log volume.

### Cost Tips
- Destroy when not in use: `automock destroy --project <name>`
- Reduce task count for smaller APIs
- Use BYO networking to share existing NAT Gateways

---

## Troubleshooting

### "No AI provider configured"
```bash
export ANTHROPIC_API_KEY="sk-ant-..."
# OR
export OPENAI_API_KEY="sk-..."
```

### "AWS credentials not found"
```bash
aws configure
# OR
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"
```

### "Failed to create S3 bucket"
- Bucket names must be globally unique
- Try using `--profile` for a different account
- Verify S3 permissions in your IAM policy

### "ECS tasks not starting"
1. Check CloudWatch logs
2. Verify expectations exist in S3
3. Check task definition CPU/memory allocation
4. Review IAM role permissions

### "Health checks failing"
1. Check MockServer started — review logs
2. Test health endpoint: `/mockserver/status`
3. Check ALB target group configuration
4. Verify security group rules

### Locust can't reach mock server endpoint
If Locust is deployed in a private subnet without a NAT route to the public internet, it cannot reach the mock server's public ALB. Deploy the mock with `enable_private_alb = true` (prompted during deploy) and configure Locust to target the private ALB endpoint instead.

---

## Common Workflows

**Development:**
```bash
automock init --project dev-api          # generate expectations
automock deploy --project dev-api        # deploy to AWS
curl https://your-alb-url/api/test       # test
automock init --project dev-api          # iterate (add/edit/remove)
automock destroy --project dev-api       # clean up
```

**Team Collaboration:**
```bash
# Member 1
automock init --project shared-api
automock deploy --project shared-api

# Member 2 (project auto-detected from S3)
automock init --project shared-api
```

**CI/CD:**
```bash
export AWS_PROFILE=ci
export ANTHROPIC_API_KEY=$CLAUDE_API_KEY

automock deploy --project test-api --skip-confirmation
npm run test:integration
automock destroy --project test-api --force
```

---

## Best Practices

1. **Project Naming** — Use descriptive names: `user-service-mock`, `payment-api-dev`
2. **Priorities** — Use 100, 200, 300… for main scenarios; 10–90 for edge cases
3. **Error Responses** — Always include 400, 401, 404, 500 responses
4. **Response Templates** — Use variables for dynamic data
5. **Destroy Unused Infrastructure** — Avoid unnecessary costs
6. **Version Control** — Export and commit expectations to git
7. **Descriptions** — Add descriptions to expectations for team readability

---

Ready to create your first mock API? Run `automock init` and follow the interactive prompts.

For a full command reference, run: `automock help`
