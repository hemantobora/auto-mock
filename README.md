# AutoMock 🧪⚡

AutoMock is an AI-powered, multi-cloud-ready CLI tool that generates and deploys mock API servers based on simple request/response definitions. It enables developers and testers to spin up ephemeral, cloud-hosted mock servers — fully managed, stateful, and automatically torn down.

---

## 🚀 Features

- 🤖 **AI-generated mock specs** from natural language prompts and sample responses
- ☁️ **Cloud-native infrastructure deployment** (AWS now, Azure/GCP extensible)
- 🧠 **Agentic CLI** with interactive flows, progress indicators, and state management
- 💾 **Persistent state** via S3 — infra can be recreated with saved stubs
- ⏱️ **Auto teardown** using TTL-aware Lambda triggers
- 🔁 **Stub operations**: create, update, delete, read
- 📡 **One-command deployment** with minimal setup
- 🎭 **Advanced scenario detection** - automatically handles multiple API variants
- 📂 **Collection import support** - Postman, Bruno, Insomnia collections
- 🎯 **Smart matching configuration** - intelligent MockServer expectation setup

---

## 📦 Project Structure

```
auto-mock/
├── cmd/auto-mock/           # CLI entrypoint (main.go)
├── internal/
│   ├── cloud/               # Cloud abstraction layer
│   │   ├── aws/
│   │   ├── gcp/
│   │   ├── azure/
│   │   └── manager.go       # Cloud provider detection logic
│   ├── generator/           # AI YAML generator (MCP-based)
│   ├── deployer/            # Infra deploy/teardown logic
│   ├── state/               # Stub storage (e.g., S3 interface)
│   └── utils/               # CLI UI, error handling
├── go.mod
└── README.md
```

---

## 🧪 Example Usage

### 🆕 Initialize a New Project
```bash
# Interactive mode (AI-guided mock generation)
automock init --project user-mock

# Collection import mode (Postman/Bruno/Insomnia)
automock init --project user-mock --collection-file api.json --collection-type postman
```
**Interactive Mode:**
- Scans AWS credentials (default or `--profile`)
- Prompts user to describe a mock endpoint
- Generates YAML spec using AI
- Creates a new S3 bucket: `auto-mock-user-mock`
- Deploys infra with 12-hour TTL

**Collection Import Mode:**
- Parses Postman/Bruno/Insomnia collections
- Executes APIs sequentially with variable resolution
- Automatically detects multiple scenarios for same endpoint
- Creates intelligent MockServer expectations with priorities
- Handles auth variations, error responses, and edge cases

### ♻️ Resume Existing Project
```bash
automock init  # Interactive project selection
```
- Lists available projects (`auto-mock-*` buckets)
- Prompts for operations: add, view, download, edit, replace, remove, delete
- Supports both AI-guided generation and collection import for existing projects

### ❌ Delete Project
```bash
automock init  # Select project, then choose 'delete' action
```
- Removes all resources, including bucket and teardown lambda
- Available through interactive project management

---

## 🔐 Credential Detection & Collection Support

**Cloud Providers:**
AutoMock automatically detects which cloud providers you have access to:
- AWS (`~/.aws/credentials`) - Primary support
- GCP (`GOOGLE_APPLICATION_CREDENTIALS`) - Planned
- Azure (`AZURE_CLIENT_ID`, etc.) - Planned

**Collection Formats:**
- **Postman** - Collection v2.1 (.json)
- **Bruno** - Collection format (.json)
- **Insomnia** - Workspace format (.json)

**Advanced Features:**
- Sequential API execution with variable resolution
- Pre/post-script processing
- Automatic scenario detection (auth, errors, variants)
- Intelligent MockServer expectation generation

---

## 🛠️ Tech Stack

- Language: **Go** (fast, cross-platform CLI)
- AWS SDK v2, S3, ECS, Lambda
- AI: MCP-based prompt-to-YAML agent  
- CLI: `urfave/cli` with interactive UX
- MockServer: Advanced expectation management
- Collection Processing: Postman/Bruno/Insomnia parsers

---

## 📈 Roadmap

- [x] AWS support for deployment + teardown
- [x] S3-based state persistence
- [x] AI-powered YAML generator
- [x] Advanced scenario detection and handling
- [x] Collection import (Postman/Bruno/Insomnia)
- [x] Smart MockServer expectation configuration
- [x] Interactive vs CLI-driven workflows
- [ ] Azure and GCP provider support
- [ ] TTL extension / reset
- [ ] CI/CD integration
- [ ] Bruno .bru file format support
- [ ] Post-MVP: Auto-generate `locustfile.py` for load testing

---

## 🤝 Contributing

Pull requests are welcome! Please open an issue first to discuss changes.

If you use AutoMock in your project, please include attribution to:
**Hemanto Bora — AutoMock (https://github.com/hemantobora/auto-mock)**

---

## 📄 License

This project is licensed under the **MIT License** — see the `LICENSE` file for details.
Attribution is required for public or commercial reuse.
