# AutoMock 🧪⚡

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org/)
[![AWS](https://img.shields.io/badge/AWS-Supported-FF9900?logo=amazon-aws)](https://aws.amazon.com/)

**AutoMock** is an AI-powered, cloud-native CLI tool that generates and deploys production-ready mock API servers from simple descriptions, API collections, or interactive builders. Spin up ephemeral, fully managed mock servers in minutes with auto-scaling, monitoring, and intelligent expectation management.

> Note: Cloud provider support is currently AWS-only. GCP and Azure are planned but not yet available.

---

## 🌟 Highlights

- 🤖 **AI-Generated Mocks** - Describe your API in natural language, get complete MockServer configurations
- ☁️ **Cloud-Native Deployment** - One command deploys ECS Fargate + ALB + Auto-scaling
- 📦 **Multi-Format Import** - Postman, Bruno, Insomnia collections → MockServer expectations
- 🔧 **Interactive Builder** - 7-step guided builder for precise control
- ⚡ **Auto-Scaling** - Configurable min/max based on CPU/Memory/Requests (defaults: 10–200 tasks)
- 💾 **Cloud Storage** - S3-backed, versioned, team-accessible
- 🎭 **Advanced Features** - Progressive delays, GraphQL, response templates, rate limiting
- 🧪 **Load Testing** - Built-in Locust test generation
- 🔐 **Production-Ready** - ALB health checks, CloudWatch monitoring, IAM best practices

---

## �️ Installation

You can install Automock via Homebrew (recommended), download prebuilt binaries from Releases, or build from source with Go.

### Option A — Homebrew (recommended)

macOS or Linux with Homebrew:

```bash
brew tap hemantobora/tap
brew install automock
# Verify
automock --version
```

Tip: You can also directly run `brew install hemantobora/tap/automock` and Homebrew will auto-tap the repo.

### Option B — Download release binaries

1) Go to the Releases page: https://github.com/hemantobora/auto-mock/releases
2) Download the archive for your OS/arch (e.g., `automock_0.0.1_darwin_arm64.tar.gz`)
3) Extract and place the binary on your PATH, e.g.:

```bash
tar -xzf automock_*.tar.gz
sudo mv automock /usr/local/bin/   # or ~/.local/bin on Linux
automock --version
```

### Option C — Go install (from source)

Requires Go 1.22+:

```bash
go install github.com/hemantobora/auto-mock/cmd/auto-mock@latest
# or pin a version, e.g. v0.0.1
go install github.com/hemantobora/auto-mock/cmd/auto-mock@v0.0.1

# Ensure your GOPATH/bin (or GOBIN) is on PATH
automock --version
```

### Upgrading

- Homebrew: `brew upgrade automock`
- Release binary: download the newer version and replace your existing binary
- Go install: `go install github.com/hemantobora/auto-mock/cmd/auto-mock@latest`

---

## �🚀 Quick Start

```bash
# Install
git clone https://github.com/hemantobora/auto-mock.git
cd auto-mock && ./build.sh

# Configure (choose one AI provider)
export ANTHROPIC_API_KEY="sk-ant-..."  # For Claude
export OPENAI_API_KEY="sk-..."         # For GPT-4

# Create your first mock
./automock init --project user-api --provider anthropic

# Deploy to AWS (optional)
./automock deploy --project user-api
```

**Your mock API is now live!** 🎉

---

## 📖 Documentation

| Document | Description |
|----------|-------------|
| [GETTING_STARTED.md](GETTING_STARTED.md) | Complete setup guide, tutorials, examples |
| [terraform/README.md](terraform/README.md) | Infrastructure details, cost estimates |
| `./automock help` | Comprehensive CLI reference |

---

## ✨ Key Features

### 🤖 AI-Powered Generation

Generate complete MockServer configurations from natural language:

```bash
./automock init --project my-api --provider anthropic
```

**Prompt:** *"User management API with registration, login, profile CRUD, password reset, and admin functions"*

**AI generates:**
- ✅ All CRUD endpoints (`GET /users`, `POST /users`, `PUT /users/{id}`, etc.)
- ✅ Authentication flows (login, logout, token refresh)
- ✅ Admin-only endpoints with proper authorization
- ✅ Error responses (400, 401, 403, 404, 500)
- ✅ Realistic test data with proper types
- ✅ Request validation rules
- ✅ Multiple scenarios per endpoint

**Supported AI Providers:**
- **Anthropic** (Claude Sonnet 4.5)
- **OpenAI** (GPT-4)
- **Template** (No AI, fallback mode)

---

### 📦 Collection Import

Import existing API definitions from popular tools:

```bash
./automock init \
  --project api-mock \
  --collection-file api.postman_collection.json \
  --collection-type postman
```

**Supported Formats:**
- **Postman** Collection v2.1 (.json)
- **Bruno** Collection (.json)
- **Insomnia** Workspace (.json) — beta

**Smart Features:**
- 🔄 Sequential API execution with variable resolution
- �️ Interactive matching configuration (guided; no automatic scenario inference)
- �️ Auto-incremented priorities to avoid collisions
- 📝 Pre/post-script processing (Postman-like JS via embedded engine)
- 🔐 Auth mapping to headers when provided in the collection

**Example: Multi-Scenario Detection**
```
Same endpoint GET /api/users/123:
  Priority 100: Anonymous → 401 Unauthorized
  Priority 200: Authenticated → 200 OK (user data)
  Priority 300: Admin → 200 OK (admin view)
  Priority 400: Rate limited → 429 Too Many Requests
```
Note: Scenarios like these are configured via the guided flow; they are not inferred automatically in all cases.

---

### 🔧 Interactive Builder

Precision-controlled, step-by-step expectation creation:

```bash
./automock init --project my-api
# Select: interactive
```

**7-Step Process:**
1. **Basic Info** - Description, priority, tags
2. **Request Matching** - Method, path, query params, headers
3. **Response Configuration** - Status code, headers, body templates
4. **Advanced Features** - Delays, caching, compression
5. **Connection Options** - Socket config, keep-alive
6. **Rate Limiting** - Per-IP, per-endpoint limits
7. **Review & Confirm** - Validate before saving

**Advanced Request Matching:**
- Path parameters: `/users/{id}/orders/{orderId}`
- Regex paths: `/api/.*/status`
- Query string matching: `?status=active&limit=10`
- Header validation: `Authorization: Bearer *`
- Body matching: exact, partial, regex, JSONPath

**Response Features:**
- Template variables: `$!uuid`, `$!now_epoch`, `$!request.headers['X-Request-ID'][0]`
- Progressive delays: 100ms → 150ms → 200ms...
- Multiple response bodies per expectation

---

### ☁️ Cloud Deployment

Deploy production-ready infrastructure with one command:

```bash
./automock deploy --project my-api
```

**What Gets Deployed:**
```
┌─────────────────────────────────────────┐
│  Application Load Balancer (Public)     │
│  http://automock-{project}-{id}.elb...  │
└─────────────┬───────────────────────────┘
              │
    ┌─────────┴─────────┐
    │  Target Groups     │
    │  • API (/)         │
    │  • Dashboard       │
    └─────────┬──────────┘
              │
┌─────────────┴────────────────────────────┐
│  ECS Fargate Cluster                     │
│  • MockServer (port 1080)                │
│  • Config Loader (sidecar)               │
│  • Auto-scaling: configurable (defaults 10–200 tasks) │
└──────────────────────────────────────────┘
              │
    ┌─────────┴──────────┐
    │  S3 Bucket          │
    │  expectations.json  │
    │  (versioned)        │
    └─────────────────────┘
```

**Infrastructure Features:**
- ⚡ **Auto-Scaling** - CPU/Memory/Request-based (10-200 tasks)
- 🔍 **Monitoring** - CloudWatch metrics, logs, alarms
- 🏥 **Health Checks** - ALB target health, /mockserver/status
- 🔒 **Security** - IAM roles, security groups, private subnets
- 💰 **Cost Optimization** - Guidance to minimize spend

**Accessing Your Mock:**
```bash
# API endpoint
curl http://automock-my-api-123.us-east-1.elb.amazonaws.com/api/users

# Dashboard (UI for expectations)
open http://automock-my-api-123.us-east-1.elb.amazonaws.com/mockserver/dashboard
```

---

### 📊 Project Management

Manage expectations throughout their lifecycle:

```bash
# View all expectations
./automock init --project my-api
# → Select: view

# Add new expectations (any generation mode)
./automock init --project my-api
# → Select: add

# Edit specific expectations
./automock init --project my-api
# → Select: edit → Choose endpoint → Modify

# Remove some expectations
./automock init --project my-api
# → Select: remove → Choose endpoints

# Replace all expectations
./automock init --project my-api
# → Select: replace → Generate new set

# Download expectations file
./automock init --project my-api
# → Select: download → Saves to {project}-expectations.json

# Delete project & infrastructure
./automock init --project my-api
# → Select: delete → Confirms & tears down everything
```

---

### 🧪 Load Testing

Generate Locust load testing bundles from collections:

```bash
./automock locust \
  --collection-file api.json \
  --collection-type postman \
  --dir ./load-tests \
  --distributed

cd load-tests
./run_locust_ui.sh

# Browser opens to http://localhost:8089
# Configure: users, spawn rate, target host
# Run load tests & view real-time metrics
```

**Generated Files:**
- `locustfile.py` - Test scenarios
- `requirements.txt` - Dependencies
- `run_locust_ui.sh` - Start with web UI
- `run_locust_headless.sh` - Run without UI
- `run_locust_master.sh` - Distributed master
- `run_locust_worker.sh` - Distributed worker

---

### ☁️ Managed Locust on AWS (beta)
#### Optional: Custom Runtime Image (Locust + Boto3)

By default, no custom image is required. The module uses public images:

- Locust: `locustio/locust:2.31.2`
- Init sidecar: `python:3.11-slim` (installs boto3 at startup and downloads the bundle)

If you prefer faster cold-starts or no runtime installs, you can optionally build a tiny derived image that already contains Locust + boto3 and your bootstrap script:

- Locust CLI
- Python + `boto3` (for S3 downloads)

Build the reference image:

```
# Example: build your own derived image (optional)
docker build -t <your-account>.dkr.ecr.<region>.amazonaws.com/automock-locust:latest <path-to-your-dockerfile>
```

Push to ECR (example):

```
aws ecr create-repository --repository-name automock-locust || true
aws ecr get-login-password --region <region> | docker login --username AWS --password-stdin <your-account>.dkr.ecr.<region>.amazonaws.com
docker push <your-account>.dkr.ecr.<region>.amazonaws.com/automock-locust:latest
```

Then set Terraform variable `locust_container_image` to that ECR URI before deploying (optional):

```
locust_container_image = "<your-account>.dkr.ecr.<region>.amazonaws.com/automock-locust:latest"
```

Sidecar environment variables used:

- `BUNDLE_BUCKET`: S3 bucket name
- `PROJECT_NAME`: Base project ID (without suffix)
- `AWS_REGION`: Region (optional but passed)

When using the default configuration, the init sidecar runs a small inline Python script to:

If you see an error like:

```
Could not find '/workspace/locustfile.py'. Ensure your locustfile ends with '.py' or is a directory with locustfiles.
```

It means no active bundle was downloaded. Common causes:
1. No load test bundle uploaded yet (run `automock locust --upload` for your project).
2. Pointer file `current.json` missing or deleted (re-upload a bundle to recreate it).
3. Bundle directory did not contain `locustfile.py` at upload time (upload validation should catch this).
4. IAM permissions missing for S3 GetObject/ListBucket on the bundle paths.

Recovery steps:
- Upload a new bundle: `automock locust --project <name> --upload --dir ./loadtest`
- Confirm pointer exists: check S3 key `configs/<project>-loadtest/current.json`
- Verify bundle objects under `configs/<project>-loadtest/bundles/<bundle_id>/`
- Re-deploy after fixing.

The master task will start even if the bundle is missing; Locust falls back to the error above. This is intentional to keep deployments responsive. Add a health check to force restart if desired:

```
healthCheck = {
  command = ["CMD-SHELL", "test -f /workspace/locustfile.py || exit 1"],
  interval = 30,
  timeout  = 5,
  retries  = 3,
  startPeriod = 20
}
```
1. Fetch `configs/<project>-loadtest/current.json`
2. Read `bundle_id` (or `BundleID` fallback)
3. Download all files under `bundles/<project>-loadtest/<bundle_id>/` into `/workspace`
4. Exit 0 even if missing (so main Locust container still starts)

IAM policy requirements for the task role:
```
Action: ["s3:GetObject", "s3:ListBucket"]
Resource:
  arn:aws:s3:::<bucket>
  arn:aws:s3:::<bucket>/configs/*
  arn:aws:s3:::<bucket>/bundles/*
```

Health check recommendation (add to container definition):
```
healthCheck = {
  command = ["CMD-SHELL", "test -f /workspace/locustfile.py || exit 1"],
  interval = 30,
  timeout  = 5,
  retries  = 3,
  startPeriod = 15
}
```


Provision a dedicated, production-ready Locust cluster on AWS using Terraform under the hood. This deploys an ALB (HTTP/HTTPS with a self-signed cert), an ECS Fargate cluster, and optional workers for distributed tests.

Deploy via the interactive REPL:

```bash
./automock repl
# In the menu:
# → Deploy Locust infrastructure
#   • Choose a project name (e.g., perf-demo)
#   • Confirm AWS region and sizing
# → Show Locust deployment details
```

Scale workers up or down (Terraform-based, drift-free):

```bash
# From REPL menu:
# → Scale Locust workers
#   • Enter new desired worker count (e.g., 5)
```

Tear down when done:

```bash
# From REPL menu:
# → Destroy Locust infrastructure
```

What you get:
- Public ALB with HTTP/HTTPS access to the Locust master UI
- Private Cloud Map namespace for service discovery within ECS
- ECS task definitions for master and workers
- CloudWatch log groups with configurable retention
- Security groups with least-privileged rules

Outputs shown by the REPL include:
- ALB DNS name (UI URL)
- Cloud Map FQDN for Locust master
- ECS cluster and service names

Note on TLS: the stack uses a self-signed certificate imported into ACM for HTTPS. For production, replace with a proper ACM certificate and Route53-managed domain.

## 📂 Project Structure

```
auto-mock/
├── cmd/auto-mock/           # CLI entrypoint (main.go)
├── internal/
│   ├── cloud/               # Cloud provider abstraction
│   │   ├── aws/             # AWS implementation (S3, ECS, IAM)
│   │   ├── factory.go       # Provider detection & initialization
│   │   └── manager.go       # Orchestration & workflows
│   ├── mcp/                 # AI provider integration (Anthropic, OpenAI)
│   ├── builders/            # Interactive expectation builders
│   ├── collections/         # Collection parsers (Postman, Bruno, Insomnia)
│   ├── expectations/        # Expectation CRUD operations
│   ├── repl/                # Interactive CLI flows
│   ├── terraform/           # Infrastructure deployment
│   └── models/              # Data structures
├── terraform/               # Terraform modules
│   ├── main.tf              # Root configuration
│   ├── variables.tf         # Input variables
│   └── outputs.tf           # Output values
├── go.mod                   # Go dependencies
├── build.sh                 # Build script
├── README.md                # This file
├── GETTING_STARTED.md       # Detailed guide
└── LICENSE                  # MIT License
```

---

## 🎯 Use Cases

### 1. Frontend Development
Mock backend APIs before they exist:
```bash
./automock init --project frontend-mock --provider anthropic
# Describe: "REST API for blog app: posts, comments, users"
./automock deploy --project frontend-mock

# Frontend team develops against:
# http://automock-frontend-mock-123.elb.amazonaws.com
```

### 2. Integration Testing
Consistent, controlled test environments:
```bash
# CI/CD pipeline
./automock deploy --project test-api --skip-confirmation
npm run test:integration -- --api-url http://automock-test-api-123.elb.amazonaws.com
./automock destroy --project test-api --force
```

### 3. Third-Party API Simulation
Test against external APIs without rate limits or costs:
```bash
./automock init \
  --project stripe-mock \
  --collection-file stripe-api.postman_collection.json \
  --collection-type postman
```

### 4. Performance Testing
Validate system behavior under load:
```bash
./automock locust \
  --collection-file prod-api.json \
  --collection-type postman \
  --dir ./load-tests

# Run distributed load test
cd load-tests
./run_locust_master.sh &
./run_locust_worker.sh &
./run_locust_worker.sh &
```

### 5. Demo & Prototyping
Quick API mocks for presentations:
```bash
./automock init --project demo-api --provider anthropic
# Describe API in seconds
./automock deploy --project demo-api
# Share URL with stakeholders
```

---

## 💰 Cost Estimates

### AWS Infrastructure (10 tasks, 24/7)

Note: 10 tasks reflects the default `min_tasks`; both `min_tasks` and `max_tasks` are configurable during setup.

| Component | Monthly Cost |
|-----------|--------------|
| ECS Fargate (0.25 vCPU, 0.5 GB) | ~$35 |
| Application Load Balancer | ~$16 |
| NAT Gateways (2x) | ~$64 |
| Data Transfer | ~$9 |
| CloudWatch Logs | ~$0.50 |
| S3 Storage | ~$0.30 |
| **Total** | **~$125** |

> Note: These are rough, region-dependent estimates and will vary with traffic, data transfer, and log volume. Please validate with the AWS Pricing Calculator for your account and region.

### Hourly Rate (rough)
- 10 tasks: **~$0.17/hour**

<!-- TTL auto-teardown is not currently implemented; TTL-based cost examples removed. -->

### AI Generation Costs
| Provider | Cost per API Generation |
|----------|-------------------------|
| Claude Sonnet 4.5 | $0.05 - $0.20 |
| GPT-4 | $0.10 - $0.30 |

**Cost Optimization Tips:**
- Destroy when not in use: `./automock destroy --project name`
- Reduce task count for smaller APIs
- Use spot instances (future feature)

---

## 🏗️ Infrastructure Details

### Auto-Scaling Policies

**Scale Up (Aggressive):**
- CPU 70-80% → +50% tasks (10 → 15)
- CPU 80-90% → +100% tasks (10 → 20)
- CPU 90%+ → +200% tasks (10 → 30)
- Memory thresholds follow same pattern
- Requests/min: 500-1000 → +50%, 1000+ → +100%

**Scale Down (Conservative):**
- CPU < 40% for 5 minutes → -25% tasks
- Cooldown: 5 minutes between scale events

**Limits:**
- Minimum: 10 tasks
- Maximum: 200 tasks

### Monitoring & Alerts

**CloudWatch Metrics:**
- ECS: CPU utilization, memory utilization, task count
- ALB: Request count, response time, 4xx/5xx errors
- Custom: Expectation reloads, config changes

**Alarms:**
- Unhealthy host count > 0
- 5XX errors > 10/minute
- CPU > 70% for 10 minutes
- Memory > 80% for 10 minutes

### Security

**IAM:**
- Least privilege access
- Separate task execution and task roles
- No hardcoded credentials

**Networking:**
- Private subnets for ECS tasks
- NAT Gateways for outbound only
- Security groups restrict traffic to ALB
- ALB in public subnets

**Data:**
- S3 server-side encryption (AES-256)
- S3 versioning enabled
- CloudWatch Logs retention: 30 days

---

## 🔧 Advanced Features

### Progressive Response Delays
Simulate degrading performance:
```json
{
  "progressive": {
    "base": 100,    // Start at 100ms
    "step": 50,     // Increase by 50ms per request
    "cap": 500      // Max 500ms
  }
}
```

**Result:**
- Request 1: 100ms delay
- Request 2: 150ms delay
- Request 3: 200ms delay
- ...
- Request N: 500ms delay (stays at cap)

### Response Templates
Dynamic values in responses:
```json
{
  "id": "$!uuid",
  "timestamp": "$!now_epoch",
  "requestId": "$!request.headers['x-request-id'][0]",
  "userId": "$!request.pathParameters['userId'][0]",
  "randomScore": "$!rand_int_100"
}
```

**Available Variables:**
- `$!uuid` - Random UUID
- `$!now_epoch` - Current timestamp (epoch seconds)
- `$!rand_int_100` - Random integer (0-100)
- `$!rand_bytes_64` - Random 64 bytes (base64)
- `$!request.path` - Request path
- `$!request.method` - Request method
- `$!request.headers['X-Header'][0]` - Header value
- `$!request.pathParameters['param'][0]` - Path parameter
- `$!request.queryStringParameters['query'][0]` - Query parameter

### GraphQL Support
Basic GraphQL request matching (no schema validation):
```json
{
  "httpRequest": {
    "method": "POST",
    "path": "/graphql",
    "body": {
      "query": {"contains": "query GetUser"},
      "variables": {"userId": "123"}
    }
  },
  "httpResponse": {
    "body": {
      "data": {
        "user": {"id": "123", "name": "John"}
      }
    }
  }
}
```

Supported matching:
- Query string contains
- Operation name extraction/matching
- Optional variables matching (exact)

---

---

## 🛠️ Development

### Build from Source
```bash
git clone https://github.com/hemantobora/auto-mock.git
cd auto-mock
go mod download
go build -o automock ./cmd/auto-mock
```

### Run Tests
```bash
go test ./...
```

### Local Development
```bash
# Build
./build.sh

# Run with verbose logging
./automock init --project test --log-level debug
```

---

## 🤝 Contributing

We welcome contributions! Here's how:

1. **Fork the repository**
2. **Create a feature branch** (`git checkout -b feature/amazing-feature`)
3. **Commit your changes** (`git commit -m 'Add amazing feature'`)
4. **Push to the branch** (`git push origin feature/amazing-feature`)
5. **Open a Pull Request**

### Areas We'd Love Help With
- [ ] Azure and GCP provider support
- [ ] Swagger/OpenAPI import
- [ ] Bruno .bru file format support
- [ ] Web UI for expectation management
- [ ] Terraform modules for other clouds
- [ ] Enhanced monitoring dashboards
- [ ] Performance optimizations

---

## 📊 Roadmap

- [x] AWS support (S3, ECS, ALB)
- [x] AI-powered mock generation (Claude, GPT-4)
- [x] Collection import (Postman, Bruno, Insomnia)
- [x] Interactive builder
- [x] Auto-scaling infrastructure
- [x] CloudWatch monitoring
- [x] Locust load testing
- [ ] Azure provider support
- [ ] GCP provider support
- [ ] Swagger/OpenAPI import
- [ ] Bruno .bru file format
- [ ] Web UI for expectation management
- [ ] Prometheus metrics export
- [ ] Custom domain support (Route53)
- [ ] Multiple region deployment
- [ ] Docker Compose local deployment
- [ ] Kubernetes deployment option

---

## 📄 License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

**Attribution Required:** If you use AutoMock in your project, please include attribution to:
```
AutoMock by Hemanto Bora
https://github.com/hemantobora/auto-mock
```

---

## 🙏 Acknowledgments

- **MockServer** - Powerful HTTP mocking server
- **Anthropic** - Claude AI for intelligent mock generation
- **OpenAI** - GPT-4 for intelligent mock generation
- **AWS** - Cloud infrastructure platform
- **Go** - Excellent tooling and performance
- **Terraform** - Infrastructure as Code

---

## 📞 Support

- **Documentation**: [GETTING_STARTED.md](GETTING_STARTED.md), `./automock help`
- **GitHub Issues**: [Create an issue](https://github.com/hemantobora/auto-mock/issues)
- **Email**: hemantobora@gmail.com

---

## ⭐ Star History

If you find AutoMock useful, please consider starring the repository!

---

<div align="center">

**Built with ❤️ by Hemanto Bora**

[GitHub](https://github.com/hemantobora/auto-mock) • [Issues](https://github.com/hemantobora/auto-mock/issues)

</div>
