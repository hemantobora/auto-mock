# AutoMock - Project Context & Architecture

> **Purpose**: This document provides complete project context for AI assistants to understand the architecture, implementation, and capabilities without reading every file. Last updated: 2025-01-04

---

## 🎯 Project Overview

**AutoMock** is an AI-powered CLI tool that generates and deploys mock API servers in the cloud. It bridges the gap between API design and testing by creating intelligent mock servers with minimal manual configuration.

### Core Value Proposition
- **AI-Generated Mocks**: Use natural language or API collections to generate MockServer configurations
- **Multi-Cloud Ready**: Supports AWS (production), GCP and Azure (planned)
- **Collection Import**: Import from Postman, Bruno, or Insomnia collections
- **Smart Scenario Detection**: Automatically identifies and configures multiple API scenarios
- **Cloud-Native**: Deploy to S3 + ECS Fargate with auto-scaling and TTL-based teardown

---

## 📐 Architecture Overview

### High-Level Architecture
```
┌─────────────────────────────────────────────────────────────┐
│                      CLI Interface                          │
│                 (cmd/auto-mock/main.go)                     │
└────────────────────┬────────────────────────────────────────┘
                     │
         ┌───────────┴───────────┐
         │                       │
┌────────▼─────────┐   ┌────────▼──────────┐
│  Cloud Manager   │   │  REPL Interface   │
│  (orchestrator)  │   │  (interactive)    │
└────────┬─────────┘   └────────┬──────────┘
         │                       │
         ├───────────┬───────────┴──────────┬──────────────┐
         │           │                      │              │
┌────────▼────┐ ┌───▼────────┐ ┌──────────▼───┐ ┌───────▼────────┐
│  Provider   │ │    MCP     │ │ Collections  │ │ Expectations   │
│   Layer     │ │  (AI Gen)  │ │  Processor   │ │    Manager     │
└────────┬────┘ └───┬────────┘ └──────────┬───┘ └───────┬────────┘
         │          │                      │              │
         └──────────┴──────────────────────┴──────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │    State Management      │
                    │   (S3 Store Factory)     │
                    └──────────────────────────┘
```

### Component Responsibilities

#### 1. **CLI Layer** (`cmd/auto-mock/main.go`)
- **Entry point** for all user interactions
- Command definitions: `init`, `upgrade`, `status`, `help`
- Flag parsing and context building
- **Key Commands**:
  - `init`: Main command for project initialization
  - `upgrade`: Upgrade project infrastructure
  - `status`: Show infrastructure status
  - `help`: Detailed help with supported features

#### 2. **Cloud Manager** (`internal/cloud/manager.go`)
- **Central orchestrator** for the entire workflow
- Manages two operation modes:
  - **Interactive Mode** (default): REPL-driven user experience
  - **Collection Mode**: CLI-driven collection import
- **Workflow**:
  1. Validate cloud provider credentials
  2. Resolve project (CLI or interactive)
  3. Initialize cloud infrastructure
  4. Generate mock configuration (AI or collection)

#### 3. **Provider Layer** (`internal/cloud/`)
- **Abstraction** for multi-cloud support
- Currently implements AWS provider
- **AWS Provider** (`internal/cloud/aws/`):
  - S3 bucket management
  - Credential validation
  - Resource creation/deletion

#### 4. **MCP (AI Generation)** (`internal/mcp/mcp.go`)
- **AI-powered mock generation** engine
- Supports multiple AI providers:
  - **Anthropic** (Claude) - requires `ANTHROPIC_API_KEY`
  - **OpenAI** (GPT-4) - requires `OPENAI_API_KEY`
  - **Template** - Free, always available
- **Capabilities**:
  - Generate from natural language descriptions
  - Convert collections to MockServer JSON
  - Security sanitization (credential removal)
  - Provider selection and fallback

#### 5. **Collections Processor** (`internal/collections/processor.go`)
- **Import and process** API collections
- Supports formats:
  - **Postman** Collection v2.1 (.json)
  - **Bruno** Collection (.json or .bru)
  - **Insomnia** Workspace (.json)
- **Advanced Features**:
  - Sequential API execution with variable resolution
  - Pre/post-script processing
  - **Scenario Detection**: Identifies multiple variants of same endpoint
  - **Smart Matching**: Configures priorities and matching rules
  - GraphQL-aware processing

#### 6. **Expectations Manager** (`internal/expectations/manager.go`)
- **CRUD operations** on MockServer expectations
- Operations:
  - `view`: Display expectations
  - `download`: Export to file
  - `edit`: Modify specific expectations
  - `remove`: Delete specific expectations
  - `replace`: Replace all expectations
  - `delete`: Delete entire project
- Integrates with S3 storage

#### 7. **REPL Interface** (`internal/repl/repl.go`)
- **Interactive experience** for mock generation
- Generation methods:
  - **describe**: Natural language → AI generation
  - **interactive**: 7-step builder
  - **collection**: Import from file
  - **template**: Quick templates
  - **upload**: Direct expectation file upload
- **Result handling**:
  - Save to S3
  - Deploy infrastructure
  - Start local MockServer
  - Exit without saving

#### 8. **State Management** (`internal/state/`)
- **S3-based persistence** for configurations
- **Store Factory Pattern**:
  - `StoreForProject()`: Creates store for project
  - Handles bucket naming and initialization
- **Data Models**:
  - `MockConfiguration`: Complete config with metadata
  - `MockExpectation`: Individual API expectation
  - `ConfigMetadata`: Version, timestamps, provider info

#### 9. **Builders** (`internal/builders/`)
- **MockServer configuration builders**
- Components:
  - `mock_configurator.go`: Interactive step-by-step builder
  - `rest.go`: REST API expectation builder
  - `graphql.go`: GraphQL expectation builder
  - `common.go`: Shared builder utilities

#### 10. **Utilities** (`internal/utils/`)
- `base62.go`: Random suffix generation
- `naming.go`: Bucket and project naming conventions

---

## 🔄 Core Workflows

### Workflow 1: Initialize New Project (Interactive)
```
$ automock init

1. CLI detects cloud credentials (AWS)
2. REPL presents project selection:
   - Create new project
   - Resume existing project
3. User creates "user-api"
4. S3 bucket created: "auto-mock-user-api-ABC123"
5. REPL asks for generation method:
   - describe / interactive / collection / template / upload
6. User chooses "describe"
7. MCP engine generates MockServer JSON using Claude/GPT-4
8. User reviews and saves to S3
9. Optional: Deploy to ECS Fargate
```

### Workflow 2: Import Postman Collection
```
$ automock init --collection-file api.json --collection-type postman

1. Cloud Manager validates credentials
2. Resolves/creates project
3. Collection Processor reads Postman file
4. Shows security disclaimer
5. Parses collection structure
6. Executes APIs sequentially:
   - Resolves variables dynamically
   - Runs pre/post-scripts
   - Records responses
7. Detects scenarios (e.g., auth/no-auth variants)
8. Configures matching criteria with priorities
9. User reviews expectations
10. Saves to S3 with metadata
```

### Workflow 3: Manage Existing Project
```
$ automock init  # Select existing project

1. REPL shows available projects
2. User selects "user-api-ABC123"
3. Checks if expectations exist
4. Shows action menu:
   - view / download / edit / remove / replace / delete
5. User chooses action
6. Expectation Manager handles operation
7. Changes saved to S3
```

---

## 📊 Data Flow

### Mock Configuration Storage
```
S3 Bucket: auto-mock-{project}-{suffix}
│
├── expectations.json          # MockServer format
├── metadata.json              # Project metadata
└── versions/
    ├── v{timestamp}.json
    └── v{timestamp}.json
```

### MockConfiguration Structure
```json
{
  "metadata": {
    "project_id": "user-api",
    "version": "v1704369600",
    "created_at": "2025-01-04T12:00:00Z",
    "updated_at": "2025-01-04T12:00:00Z",
    "description": "User management API",
    "provider": "anthropic-claude"
  },
  "expectations": [
    {
      "id": "exp_12345",
      "priority": 1,
      "httpRequest": {
        "method": "POST",
        "path": "/api/users",
        "headers": { "Content-Type": "application/json" }
      },
      "httpResponse": {
        "statusCode": 201,
        "body": { "id": "123", "name": "John" }
      }
    }
  ],
  "settings": {
    "source": "ai-generated",
    "import_method": "describe"
  }
}
```

---

## 🎭 Advanced Features

### 1. Scenario Detection & Matching
The Collections Processor automatically detects multiple scenarios for the same endpoint:

**Example**: Same `/api/users` endpoint with different behaviors
- **Scenario 1** (Priority 1): Success case (200 OK)
- **Scenario 2** (Priority 2): Missing auth (401 Unauthorized)
- **Scenario 3** (Priority 3): Invalid auth (403 Forbidden)
- **Scenario 4** (Priority 4): Validation error (400 Bad Request)

**Matching Configuration**:
- Scenarios prioritized by specificity
- Auto-configures header matching (regex, exact, not-present)
- Handles GraphQL variants (different queries/variables)
- Supports status code differentiation

### 2. Variable Resolution in Collections
Sequential execution with dynamic variable extraction:

```javascript
// Pre-script (Postman/Bruno/Insomnia)
pm.environment.set("userId", pm.response.json().id);

// Post-script
const token = pm.response.json().token;
pm.environment.set("authToken", token);
```

**Collection Processor**:
1. Executes API 1 → extracts `userId`
2. Uses `userId` in API 2 request
3. Executes API 2 → extracts `authToken`
4. Uses `authToken` in API 3 headers

### 3. AI Provider Selection
**Priority Order**:
1. User-specified provider (`--provider anthropic`)
2. Available API keys (Anthropic > OpenAI)
3. Template provider (always available)

**Provider Info**:
```go
type ProviderInfo struct {
    Name      string // "Anthropic Claude"
    Available bool   // Has API key?
    Cost      string // "Paid" or "Free"
    IsFree    bool   // true for Template provider
}
```

### 4. GraphQL Support
- Detects GraphQL requests (path, content-type, body structure)
- Groups by operation name/query
- Scenario variants: different queries, variables, auth
- Proper body matching for GraphQL requests

### 5. Security Features
**Credential Sanitization** (`internal/mcp/security/sanitizer.go`):
- Removes API keys, tokens, passwords
- Redacts sensitive headers (Authorization, Cookie)
- Sanitizes URLs (removes auth params)
- Provides warnings to user

---

## 🗂️ File Structure & Purpose

### Core Files
```
auto-mock/
├── cmd/
│   └── auto-mock/
│       └── main.go                    # CLI entrypoint, commands
│
├── internal/
│   ├── cloud/
│   │   ├── manager.go                 # Central orchestrator
│   │   ├── provider.go                # Provider interface
│   │   ├── providers.go               # Provider discovery
│   │   └── aws/
│   │       ├── provider.go            # AWS implementation
│   │       └── delete.go              # Resource cleanup
│   │
│   ├── mcp/
│   │   ├── mcp.go                     # AI generation engine
│   │   ├── providers/
│   │   │   ├── anthropic.go           # Claude provider
│   │   │   └── providers.go           # Provider manager
│   │   └── security/
│   │       └── sanitizer.go           # Credential removal
│   │
│   ├── collections/
│   │   ├── processor.go               # Collection import
│   │   ├── script_engine.go           # Pre/post-script execution
│   │   └── variable_extractor.go      # Variable resolution
│   │
│   ├── expectations/
│   │   └── manager.go                 # Expectation CRUD
│   │
│   ├── repl/
│   │   ├── repl.go                    # Interactive interface
│   │   └── interactive.go             # 7-step builder
│   │
│   ├── state/
│   │   ├── store.go                   # Storage interface
│   │   ├── s3_store.go                # S3 implementation
│   │   └── store_factory.go           # Factory pattern
│   │
│   ├── builders/
│   │   ├── mock_configurator.go       # Interactive builder
│   │   ├── rest.go                    # REST builder
│   │   └── graphql.go                 # GraphQL builder
│   │
│   └── utils/
│       ├── base62.go                  # Random ID generation
│       └── naming.go                  # Naming conventions
│
├── terraform/                         # Infrastructure as Code
│   └── modules/
│       ├── automock-ecs/              # ECS Fargate module
│       └── automock-s3/               # S3 module
│
├── go.mod                             # Go dependencies
├── README.md                          # User documentation
├── GETTING_STARTED.md                 # Quick start guide
└── PROJECT_CONTEXT.md                 # This file
```

### Key Dependencies
```go
require (
    github.com/aws/aws-sdk-go-v2          // AWS SDK
    github.com/urfave/cli/v2              // CLI framework
    github.com/AlecAivazis/survey/v2      // Interactive prompts
    github.com/dop251/goja                // JavaScript execution
)
```

---

## 🔧 Configuration & Environment

### Required Environment Variables
```bash
# AI Providers (at least one recommended)
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."

# AWS Credentials (via standard methods)
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_PROFILE="dev"
```

### CLI Flags
```bash
--profile         # AWS profile name
--project         # Project name (skip selection)
--provider        # AI provider (anthropic/openai/template)
--include-auth    # Include auth endpoints
--include-errors  # Include error responses (default: true)
--collection-file # Path to collection file
--collection-type # Collection format (postman/bruno/insomnia)
```

---

## 🎯 Use Cases

### Use Case 1: Frontend Development
**Scenario**: Frontend team needs mock API before backend is ready

**Solution**:
```bash
$ automock init --project checkout-api
# Describe API in natural language
# AI generates complete mock with realistic data
# Deploy to cloud, share URL with team
```

### Use Case 2: API Testing
**Scenario**: QA team needs to test different error scenarios

**Solution**:
```bash
$ automock init --collection-file api-tests.json --collection-type postman
# Processor detects scenarios: success, 400, 401, 403, 500
# Configures priorities and matching automatically
# Save to S3, deploy to ECS
```

### Use Case 3: CI/CD Integration
**Scenario**: Automated tests need mock server in pipeline

**Solution**:
```bash
$ automock init --project e2e-tests
# Generate mocks from API spec
# Save to S3
# Use mock URL in test suite
# TTL auto-teardown after tests complete
```

### Use Case 4: API Design Validation
**Scenario**: Product team wants to validate API design before implementation

**Solution**:
```bash
$ automock init --project design-review
# Upload expectation file with proposed API
# Share dashboard URL for review
# Iterate on design based on feedback
```

---

## 🚀 Deployment Options

### Option 1: Basic (S3 Only) - Current Default
- S3 bucket for expectations
- Fast initialization
- Cost-effective
- Manual MockServer setup required

### Option 2: Complete (ECS + ALB) - Future
- ECS Fargate service
- Application Load Balancer
- Auto-scaling
- CloudWatch monitoring
- TTL-based auto-teardown
- Custom domain + SSL

---

## 🔮 Future Enhancements

### Planned Features
1. **GCP & Azure Support**: Multi-cloud provider implementation
2. **TTL Extension**: Reset/extend infrastructure lifetime
3. **CI/CD Integration**: GitHub Actions, GitLab CI templates
4. **Load Testing**: Auto-generate `locustfile.py` from expectations
5. **Observability**: Metrics, traces, analytics dashboard
6. **Collaborative Features**: Team management, shared projects
7. **Advanced Matching**: Content negotiation, cookies, SSL verification

### Potential Improvements
- **Performance**: Parallel API execution in collection processor
- **UX**: Progress bars, better error messages
- **Storage**: Alternative backends (DynamoDB, Redis)
- **Security**: Vault integration, secret rotation
- **Testing**: Comprehensive unit and integration tests

---

## 📝 Development Guidelines

### Adding New Features

#### 1. New AI Provider
```go
// internal/mcp/providers/new_provider.go
type NewProvider struct {
    apiKey string
}

func (p *NewProvider) GenerateMockConfig(ctx context.Context, 
    input string, options *GenerationOptions) (*GenerationResult, error) {
    // Implementation
}

// Register in providers.go
func NewProviderManager() *ProviderManager {
    pm := &ProviderManager{}
    pm.providers = append(pm.providers, NewNewProvider())
    return pm
}
```

#### 2. New Collection Format
```go
// internal/collections/processor.go
func (cp *CollectionProcessor) parseNewFormat(data []byte) ([]APIRequest, error) {
    // Parse collection structure
    // Extract requests, headers, body, scripts
    // Return APIRequest slice
}

// Add to parseCollectionFile switch
case "newformat":
    return cp.parseNewFormat(data)
```

#### 3. New Cloud Provider
```go
// internal/cloud/newcloud/provider.go
type NewCloudProvider struct {
    profile string
    projectName string
}

func (p *NewCloudProvider) InitProject() error {
    // Create resources
}

func (p *NewCloudProvider) DeleteProject() error {
    // Cleanup resources
}

// Register in providers.go
```

### Code Style
- **Error handling**: Always return errors, never panic
- **Logging**: Use fmt.Printf for user messages
- **Context**: Pass context.Context for cancellation
- **Naming**: Clear, descriptive variable names
- **Comments**: Document public functions and complex logic

---

## 🐛 Troubleshooting

### Common Issues

#### "No valid cloud provider credentials found"
**Solution**: Configure AWS credentials
```bash
aws configure --profile dev
# or
export AWS_ACCESS_KEY_ID="..." AWS_SECRET_ACCESS_KEY="..."
```

#### "Failed to save to S3"
**Solution**: Check S3 permissions
```bash
# Required IAM permissions:
s3:CreateBucket, s3:PutObject, s3:GetObject, s3:DeleteObject
```

#### "AI generation failed"
**Solution**: Verify API key
```bash
echo $ANTHROPIC_API_KEY  # Should not be empty
# or use template provider (no key required)
automock init --provider template
```

#### Collection import fails on variable resolution
**Solution**: Ensure APIs are in correct order, provide missing environment variables
```bash
export USER_ID="12345" AUTH_TOKEN="abc123"
automock init --collection-file api.json --collection-type postman
```

---

## 📚 Key Concepts

### MockServer Expectations
AutoMock generates **MockServer expectations** - JSON configurations that define:
- **Request matching**: Method, path, headers, body
- **Response**: Status code, headers, body
- **Priority**: Order of evaluation (higher = more specific)
- **Times**: How many times to match (unlimited, once, etc.)

### Collection Processor Workflow
1. **Parse**: Read collection file structure
2. **DAG Build**: Determine execution order
3. **Execute**: Run APIs sequentially
4. **Extract**: Get variables from responses
5. **Detect**: Identify scenarios
6. **Configure**: Set matching criteria
7. **Save**: Store to S3

### AI Generation Modes
- **describe**: Natural language → MockServer JSON
- **template**: Pre-built templates (user/auth/product/etc.)
- **collection**: Import → AI enhancement → MockServer JSON

---

## 🎓 Learning Resources

### MockServer Documentation
- [MockServer.org](https://www.mock-server.com/)
- Expectation format reference
- Request matching rules
- Response templates

### Collection Formats
- **Postman**: [Schema v2.1](https://schema.postman.com/json/collection/v2.1.0/collection.json)
- **Bruno**: [Docs](https://docs.usebruno.com/)
- **Insomnia**: [Docs](https://docs.insomnia.rest/)

### AWS Resources
- S3 documentation
- ECS Fargate guide
- Application Load Balancer setup

---

## 📞 Support & Contribution

### Getting Help
- Check `README.md` for user documentation
- Review `GETTING_STARTED.md` for quick start
- Use `automock help` for CLI reference

### Contributing
- Open issues for bugs/feature requests
- Follow code style guidelines
- Add tests for new features
- Update this context document for major changes

---

## 🏁 Quick Reference

### Most Common Commands
```bash
# Interactive mode (recommended)
automock init

# Quick start with project name
automock init --project my-api

# Import Postman collection
automock init --collection-file api.json --collection-type postman

# Use specific AI provider
automock init --provider anthropic

# Check project status
automock status --project my-api
```

### Architecture Decisions
1. **S3 for state**: Cloud-native, versioned, team-accessible
2. **Factory pattern**: Easy to add new storage backends
3. **Provider interface**: Multi-cloud abstraction
4. **AI-first**: Leverage LLMs for configuration generation
5. **Collection support**: Bridge existing API tools

---

**Last Updated**: 2025-01-04  
**Version**: 1.0  
**Author**: Hemanto Bora

---

## Appendix: Implementation Status

### ✅ Completed
- AWS provider implementation
- S3 state management
- MCP AI generation (Anthropic, OpenAI, Template)
- Collection import (Postman, Bruno, Insomnia)
- Scenario detection and matching
- Expectations management (CRUD)
- REPL interactive interface
- 7-step interactive builder

### 🚧 In Progress
- ECS Fargate deployment
- TTL-based auto-teardown
- Custom domain + SSL

### 📋 Planned
- GCP and Azure providers
- CI/CD integration
- Load testing generation
- Team collaboration features
- Advanced observability

---

*This document serves as the primary context for AI assistants working with the AutoMock codebase. It should be kept up-to-date with major architectural changes.*
