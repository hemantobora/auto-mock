# AutoMock 🧪⚡

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org/)
[![AWS](https://img.shields.io/badge/AWS-Supported-FF9900?logo=amazon-aws)](https://aws.amazon.com/)

**AutoMock** is a cloud-native CLI tool that generates and deploys production-ready mock API servers from natural language descriptions, API collections, or an interactive builder. Spin up fully managed mock servers on AWS (ECS Fargate) with auto-scaling, monitoring, and built-in load testing support.

> Cloud provider support is currently AWS-only. GCP and Azure are planned but not yet available.

---

## 🌟 Highlights

- 🤖 **AI-Generated Mocks** — Describe your API in natural language, get complete MockServer configurations
- ☁️ **AWS Deployment** — ECS Fargate + ALB — fully managed, auto-scaling
- 📦 **Multi-Format Import** — Postman, Bruno, Insomnia, OpenCollection → MockServer expectations and Locust bundles
- 🔧 **Interactive Builder** — 7-step guided builder for precise control
- ⚡ **Auto-Scaling** — CPU/Memory/Request-based (10–200 tasks by default)
- 💾 **S3 Storage** — Versioned, team-accessible expectations
- 🎭 **Advanced Features** — Progressive delays, GraphQL, response templates, response limits
- 🧪 **Load Testing** — Locust bundle generation and managed cloud deployment (AWS ECS)
- 🔐 **Production-Ready** — Health checks, monitoring, IAM best practices

---

## 🗺️ Installation

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

### Upgrading

- Homebrew: `brew upgrade automock`
- Scoop: `scoop update automock`
- Binary: download the newer version and replace the existing binary

---

## 🚀 Quick Start

```bash
# Set your AI provider key (choose one)
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."

# Configure AWS credentials (or use existing profile)
aws configure

# Generate and initialise a project
automock init --project user-api --provider anthropic

# Deploy to AWS
automock deploy --project user-api
```

Your mock API is now live at the endpoint shown after deploy.

---

## 📖 Documentation

| Document | Description |
|----------|-------------|
| [GETTING_STARTED.md](GETTING_STARTED.md) | Complete setup guide, tutorials, examples |
| `automock help` | CLI reference |

---

## ✨ Key Features

### 🤖 AI-Powered Generation

Generate complete MockServer configurations from natural language:

```bash
automock init --project my-api --provider anthropic
```

**Prompt:** *"User management API with registration, login, profile CRUD, password reset, and admin functions"*

**AI generates:**
- All CRUD endpoints (`GET /users`, `POST /users`, `PUT /users/{id}`, etc.)
- Authentication flows (login, logout, token refresh)
- Admin-only endpoints with authorization
- Error responses (400, 401, 403, 404, 500)
- Realistic test data with proper types
- Request validation rules
- Multiple scenarios per endpoint

**Supported AI Providers:**
- **Anthropic** (Claude Sonnet 4.5)
- **OpenAI** (GPT-4)
- **Template** (no AI, fallback mode)

---

### 📦 Collection Import

Import existing API definitions from popular tools — for both mock generation and load test bundles:

```bash
# Postman
automock init \
  --project api-mock \
  --collection-file api.postman_collection.json \
  --collection-type postman

# Bruno OpenCollection v3 (bundled YAML)
automock init \
  --project api-mock \
  --collection-file open.yml \
  --collection-type opencollection
```

**Supported Formats:**

| Format | Flag | Notes |
|--------|------|-------|
| Postman Collection v2.1 | `--collection-type postman` | .json export |
| Bruno JSON export | `--collection-type bruno` | .json export |
| Bruno OpenCollection v3 | `--collection-type opencollection` | bundled .yml |
| Insomnia Workspace | `--collection-type insomnia` | .json export |

All four formats are supported in both `automock init` (mock generation) and `automock load` (Locust bundle generation).

**Features:**
- Sequential API execution with variable resolution
- Interactive matching configuration (guided; no automatic scenario inference)
- Auto-incremented priorities to avoid collisions
- Pre/post-script processing — Postman (`pm.environment.set`, `pm.response.json()`) and Bruno (`bru.setEnvVar`, `res.body`) script APIs via embedded JS engine
- Auth mapping to headers when provided in the collection
- Template variable detection (`{{variable}}`) — warns and skips exact-match offer for body/query params containing placeholders

**Example: Multi-Scenario Configuration**
```
GET /api/users/123:
  Priority 100: Anonymous → 401 Unauthorized
  Priority 200: Authenticated → 200 OK (user data)
  Priority 300: Admin → 200 OK (admin view)
  Priority 400: Rate limited → 429 Too Many Requests
```

---

### 🔧 Interactive Builder

Precision-controlled, step-by-step expectation creation:

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

**Advanced Request Matching:**
- Path parameters: `/users/{id}/orders/{orderId}`
- Regex paths: `/api/.*/status`
- Query string matching: `?status=active&limit=10`
- Header validation: `Authorization: Bearer *`
- Body matching: exact (STRING), partial (JSON ONLY_MATCHING_FIELDS), JSON Schema, regex, parameters

**Response Features:**
- Template variables: `$!uuid`, `$!now_epoch`, `$!request.headers['X-Request-ID'][0]`
- Progressive delays: 100ms → 150ms → 200ms…
- Multiple response bodies per expectation

---

### ☁️ Cloud Deployment

Deploy production-ready infrastructure with one command:

```bash
automock deploy --project my-api
```

#### AWS — ECS Fargate + ALB

```
┌─────────────────────────────────────────┐
│  Application Load Balancer (Public)     │
│  https://automock-{project}-{id}.elb…  │
└─────────────┬───────────────────────────┘
              │
    ┌─────────┴──────────┐
    │  Target Groups      │
    │  • API (/)          │
    │  • Dashboard        │
    └─────────┬───────────┘
              │
┌─────────────┴────────────────────────────┐
│  ECS Fargate Cluster                     │
│  • MockServer (port 1080)                │
│  • Config Loader (sidecar)               │
│  • Auto-scaling: configurable            │
│    (defaults: 10–200 tasks)              │
└──────────────────────────────────────────┘
              │
    ┌─────────┴──────────┐
    │  S3 Bucket          │
    │  expectations.json  │
    │  (versioned)        │
    └─────────────────────┘
```

**Infrastructure features:**
- Auto-Scaling — CPU/Memory/Request-based (10–200 tasks by default)
- Monitoring — CloudWatch metrics, logs, alarms
- Health Checks — ALB target health, `/mockserver/status`
- Security — IAM roles, security groups, private subnets
- Custom Domain — ACM certificate + Route53 (optional, prompted at deploy time)
- Private ALB — Optional internal ALB for VPC-internal clients (e.g., Locust)
- BYO Networking — Use existing VPC, subnets, IGW, NAT, IAM roles, and security groups

**Authenticating:**
```bash
aws configure                  # or use an existing named profile
```

---

### 📊 Project Management

Manage expectations throughout their lifecycle:

```bash
# View / add / edit / remove / replace / download / delete
automock init --project my-api
# → Select the desired action from the menu
```

| Action | Description |
|--------|-------------|
| `view` | List all expectations |
| `add` | Add new expectations (any generation mode) |
| `edit` | Edit specific expectations |
| `remove` | Remove selected expectations |
| `replace` | Replace all expectations |
| `download` | Save to `{project}-expectations.json` |
| `delete` | Tear down project and all infrastructure |

---

### 🧪 Load Testing — Local

Generate a ready-to-run Locust bundle from any supported collection format:

```bash
automock load \
  --collection-file api.json \
  --collection-type postman \   # postman | insomnia | bruno | opencollection
  --dir ./load-tests \
  --distributed

cd load-tests
./run_locust_ui.sh
# Browser opens at http://localhost:8089
```

**Generated files:**
- `locustfile.py` — Test scenarios (run via Locust, not Python directly)
- `locust_endpoints.json` — Endpoint config — edit to tune behaviour
- `user_data.yaml` — Per-user test data rows
- `requirements.txt` — Python dependencies
- `run_locust_ui.sh` / `.ps1` — Start with web UI
- `run_locust_headless.sh` / `.ps1` — Run without UI
- `run_locust_master.sh` / `.ps1` — Distributed master
- `run_locust_worker.sh` / `.ps1` — Distributed worker

**Running the load test:**

```bash
cd load-tests

# UI mode — opens http://localhost:8089 to set users/rate interactively
export AM_HOST="http://your-target-host"
./run_locust_ui.sh

# Headless mode — set params via env vars
export AM_HOST="http://your-target-host"
export AM_USERS=20
export AM_SPAWN_RATE=5
export AM_DURATION=5m
./run_locust_headless.sh
```

> **Note:** Run via the shell scripts or `locust -f locustfile.py`. Running `python3 locustfile.py` directly only loads configuration and prints the data row count — no test starts.

**Variable substitution:**
- `${env.VAR}` — Expanded at load-time from environment / `.env` file
- `${data.<field>}` and `${user.id|index}` — Expanded at runtime in path, headers, params, and body
- In `auth.mode: shared`, only `${env.*}` expands; in `auth.mode: per_user`, `${data.*}` and `${user.*}` also expand in the login path/headers/body

---

### ☁️ Managed Locust — AWS

Deploy a Locust cluster to AWS via the same `deploy` command. AutoMock provisions an ECS Fargate cluster (master + workers), an ALB for the Locust UI, and Cloud Map service discovery.

```bash
# 1. Upload your load test bundle
automock load --project my-api --upload --dir ./load-tests

# 2. Deploy infrastructure (prompts for sizing)
automock deploy --project my-api

# 3. Scale workers
automock deploy --project my-api
# → prompts for new worker count when already deployed

# 4. Tear down
automock destroy --project my-api
# → select: mocks, loadtest, or both
```

**What you get:**
- Public ALB with HTTP/HTTPS access to the Locust master UI
- ECS task definitions for master and workers (configurable CPU/memory)
- Cloud Map private namespace for master–worker discovery
- CloudWatch log groups

---

## 🎯 Use Cases

### Frontend Development
Mock backend APIs before they exist:
```bash
automock init --project frontend-mock --provider anthropic
# Describe: "REST API for blog app: posts, comments, users"
automock deploy --project frontend-mock
```

### Integration Testing
Consistent, controlled test environments in CI/CD:
```bash
automock deploy --project test-api --skip-confirmation
npm run test:integration -- --api-url https://automock-test-api-123.elb.amazonaws.com
automock destroy --project test-api --force
```

### Third-Party API Simulation
Test against external APIs without rate limits or costs:
```bash
automock init \
  --project stripe-mock \
  --collection-file stripe-api.postman_collection.json \
  --collection-type postman
```

### Performance Testing
Generate and deploy load tests against your mock:
```bash
automock load \
  --collection-file prod-api.json \
  --collection-type postman \
  --dir ./load-tests
automock load --project prod-api --upload --dir ./load-tests
automock deploy --project prod-api
```

---

## 💰 Cost Estimates

### AWS (default: 10 tasks, 24/7)

`min_tasks` and `max_tasks` are configurable at deploy time. Using BYO networking (existing VPC, subnets, NAT) eliminates the NAT Gateway cost.

| Component | Monthly Cost |
|-----------|--------------|
| ECS Fargate (0.25 vCPU, 0.5 GB) | ~$35 |
| Application Load Balancer | ~$16 |
| NAT Gateway | ~$32 |
| Data Transfer | ~$9 |
| CloudWatch Logs | ~$0.50 |
| S3 Storage | ~$0.30 |
| **Total** | **~$93** |

> Rough East US estimates; varies by region and traffic. Validate with the [AWS Pricing Calculator](https://calculator.aws).

**Hourly rate (10 tasks):** ~$0.13/hour

### AI Generation Costs

| Provider | Cost per Generation |
|----------|---------------------|
| Claude Sonnet 4.5 | $0.05 – $0.20 |
| GPT-4 | $0.10 – $0.30 |

**Cost tips:** Destroy when not in use (`automock destroy --project <name>`), reduce task count for smaller APIs, or use BYO networking to share existing NAT Gateways.

---

## 🏗️ Infrastructure Details

### Auto-Scaling

**Scale Up (Aggressive):**
- CPU 70–80% → +50% tasks
- CPU 80–90% → +100% tasks
- CPU 90%+ → +200% tasks
- Memory thresholds follow the same pattern
- Requests/min: 500–1000 → +50%, 1000+ → +100%

**Scale Down (Conservative):**
- CPU < 40% for 5 minutes → −25% tasks
- Cooldown: 5 minutes between scale events

**Limits:** minimum 10 tasks, maximum 200 tasks (both configurable)

### Monitoring & Alerts

**CloudWatch Metrics:** ECS CPU/memory/task count, ALB request count/response time/4xx/5xx errors

**Alarms:** unhealthy host count > 0, 5XX errors > 10/min, CPU > 70% for 10 min, Memory > 80% for 10 min

### Security

- **IAM** — Least-privilege, separate task execution and task roles, no hardcoded credentials; optional permissions boundary and custom role path
- **Networking** — ECS tasks in private subnets behind ALB
- **Data** — S3 server-side encryption (AES-256) with versioning enabled

---

## 🔧 Advanced Features

### Progressive Response Delays

Simulate degrading performance:
```json
{
  "progressive": {
    "base": 100,
    "step": 50,
    "cap": 500
  }
}
```
Request 1: 100ms → Request 2: 150ms → … → capped at 500ms

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

**Available variables:** `$!uuid`, `$!now_epoch`, `$!rand_int_100`, `$!rand_bytes_64`, `$!request.path`, `$!request.method`, `$!request.headers['X-Header'][0]`, `$!request.pathParameters['param'][0]`, `$!request.queryStringParameters['query'][0]`

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
    "body": {"data": {"user": {"id": "123", "name": "John"}}}
  }
}
```

Supported: query string contains, operation name matching, optional variables matching (exact).

---

## 📂 Project Structure

```
auto-mock/
├── cmd/auto-mock/           # CLI entrypoint
├── internal/
│   ├── cloud/               # Cloud provider abstraction
│   │   ├── aws/             # AWS implementation (S3, ECS, IAM)
│   │   ├── factory.go       # Provider detection & initialization
│   │   └── manager.go       # Orchestration
│   ├── mcp/                 # AI provider integration (Anthropic, OpenAI)
│   ├── builders/            # Interactive expectation builders
│   ├── collections/         # Collection parsers (Postman, Bruno, Insomnia, OpenCollection)
│   ├── expectations/        # Expectation CRUD operations
│   ├── repl/                # Interactive CLI flows
│   ├── terraform/           # Embedded infrastructure modules
│   │   └── infra/
│   │       ├── mock/aws/    # ECS Fargate MockServer
│   │       └── loadtest/aws/   # ECS Fargate Locust
│   └── models/              # Data structures
├── go.mod
├── build.sh
├── README.md
└── LICENSE
```

---

## 📊 Roadmap

- [x] AWS support (S3, ECS, ALB)
- [x] AI-powered mock generation (Claude, GPT-4)
- [x] Collection import (Postman, Bruno, Insomnia, OpenCollection)
- [x] OpenCollection support in load test bundle generation
- [x] Interactive builder
- [x] Auto-scaling infrastructure
- [x] CloudWatch monitoring
- [x] Locust load testing (local + managed AWS)
- [x] Custom domain support (ACM + Route53)
- [x] Private ALB for VPC-internal clients
- [x] BYO networking (VPC, subnets, IAM, security groups)
- [ ] Azure provider support
- [ ] GCP provider support
- [ ] Swagger/OpenAPI import
- [ ] Bruno directory-based (.bru files) format
- [ ] Web UI for expectation management
- [ ] Prometheus metrics export
- [ ] Multiple region deployment
- [ ] Docker Compose local deployment

---

## 🛠️ Development

```bash
git clone https://github.com/hemantobora/auto-mock.git
cd auto-mock
go mod download
go build -o automock ./cmd/auto-mock

# Run tests
go test ./...
```

---

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

**Areas we'd love help with:** Azure/GCP provider support, Swagger/OpenAPI import, Bruno directory-based (.bru) format, Web UI for expectation management, Prometheus metrics export.

---

## 📄 License

This project is licensed under the **MIT License** — see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- **MockServer** — Powerful HTTP mocking server
- **Anthropic** — Claude AI for intelligent mock generation
- **OpenAI** — GPT-4 for intelligent mock generation
- **AWS** — Cloud infrastructure platform
- **Terraform** — Infrastructure as Code
- **Locust** — Load testing framework

---

## 📞 Support

- **Documentation**: [GETTING_STARTED.md](GETTING_STARTED.md), `automock help`
- **GitHub Issues**: [Create an issue](https://github.com/hemantobora/auto-mock/issues)
- **Email**: hemantobora@gmail.com

---

<div align="center">

**Built with ❤️ by Hemanto Bora**

[GitHub](https://github.com/hemantobora/auto-mock) • [Issues](https://github.com/hemantobora/auto-mock/issues)

</div>
