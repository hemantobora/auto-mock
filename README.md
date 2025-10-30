# AutoMock 🧪⚡

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org/)
[![AWS](https://img.shields.io/badge/AWS-Supported-FF9900?logo=amazon-aws)](https://aws.amazon.com/)

**AutoMock** is an AI-powered, cloud-native CLI tool that generates and deploys production-ready mock API servers from simple descriptions, API collections, or interactive builders. Spin up ephemeral, fully managed mock servers in minutes with auto-scaling, monitoring, and intelligent expectation management.

---

## 🌟 Highlights

- 🤖 **AI-Generated Mocks** - Describe your API in natural language, get complete MockServer configurations
- ☁️ **Cloud-Native Deployment** - One command deploys ECS Fargate + ALB + Auto-scaling
- 📦 **Multi-Format Import** - Postman, Bruno, Insomnia collections → MockServer expectations
- 🎯 **Smart Scenario Detection** - Automatically handles auth variations, error states, edge cases
- 🔧 **Interactive Builder** - 7-step guided builder for precise control
- ⚡ **Auto-Scaling** - 10-200 tasks based on CPU/Memory/Requests
- 💾 **Cloud Storage** - S3-backed, versioned, team-accessible
- 🎭 **Advanced Features** - Progressive delays, GraphQL, response templates, rate limiting
- 🧪 **Load Testing** - Built-in Locust test generation
- 🔐 **Production-Ready** - ALB health checks, CloudWatch monitoring, IAM best practices

---

## 🚀 Quick Start

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
- **Anthropic** (Claude Sonnet 4.5) - Recommended
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
- **Insomnia** Workspace (.json)

**Smart Features:**
- 🔄 Sequential API execution with variable resolution
- 🎯 Automatic scenario detection (auth variations, error states)
- 🏆 Intelligent priority assignment (100, 200, 300...)
- 📝 Pre/post-script processing
- 🔐 Auth header injection

**Example: Multi-Scenario Detection**
```
Same endpoint GET /api/users/123:
  Priority 100: Anonymous → 401 Unauthorized
  Priority 200: Authenticated → 200 OK (user data)
  Priority 300: Admin → 200 OK (admin view)
  Priority 400: Rate limited → 429 Too Many Requests
```

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
- Conditional responses based on request data
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
│  • Auto-scaling: 10-200 tasks            │
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
- 💰 **Cost Optimization** - Optional TTL cleanup, auto-teardown

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

| Component | Monthly Cost |
|-----------|--------------|
| ECS Fargate (0.25 vCPU, 0.5 GB) | ~$35 |
| Application Load Balancer | ~$16 |
| NAT Gateways (2x) | ~$64 |
| Data Transfer | ~$9 |
| CloudWatch Logs | ~$0.50 |
| S3 Storage | ~$0.30 |
| **Total** | **~$125** |

### Hourly Rate
- 10 tasks: **~$0.17/hour**
- With TTL cleanup: **Pennies per test run**

### TTL-Based Costs
| Duration | Cost |
|----------|------|
| 4 hours | ~$0.68 |
| 8 hours | ~$1.37 |
| 24 hours | ~$4.11 |
| 1 week | ~$28.77 |

### AI Generation Costs
| Provider | Cost per API Generation |
|----------|-------------------------|
| Claude Sonnet 4.5 | $0.05 - $0.20 |
| GPT-4 | $0.10 - $0.30 |

**Cost Optimization Tips:**
- Use TTL cleanup to auto-destroy infrastructure
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
Create expectations for GraphQL APIs:
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

### Conditional Responses
Different responses based on request data:
```json
[
  {
    "priority": 100,
    "httpRequest": {
      "headers": {"X-User-Type": ["premium"]}
    },
    "httpResponse": {
      "body": {"features": ["all"]}
    }
  },
  {
    "priority": 200,
    "httpRequest": {},
    "httpResponse": {
      "body": {"features": ["basic"]}
    }
  }
]
```

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
