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
auto-mock init --project user-mock
```
- Scans AWS credentials (default or `--profile`)
- Prompts user to describe a mock endpoint
- Generates YAML spec using AI
- Creates a new S3 bucket: `auto-mock-user-mock`
- Deploys infra with 12-hour TTL

### ♻️ Resume Existing Project
```bash
auto-mock resume
```
- Lists available projects (`auto-mock-*` buckets)
- Prompts for stub operations: add, update, delete, view

### ❌ Delete Project
```bash
auto-mock delete --project user-mock
```
- Removes all resources, including bucket and teardown lambda

---

## 🔐 Credential Detection

AutoMock automatically detects which cloud providers you have access to:
- AWS (`~/.aws/credentials`)
- GCP (`GOOGLE_APPLICATION_CREDENTIALS`)
- Azure (`AZURE_CLIENT_ID`, etc.)

If credentials for multiple are found, you're prompted to choose your target platform.

---

## 🛠️ Tech Stack

- Language: **Go** (fast, cross-platform CLI)
- AWS SDK v2, S3, ECS, Lambda
- AI: MCP-based prompt-to-YAML agent
- CLI: `urfave/cli` with interactive UX

---

## 📈 Roadmap

- [x] AWS support for deployment + teardown
- [x] S3-based state persistence
- [x] AI-powered YAML generator
- [ ] Azure and GCP provider support
- [ ] TTL extension / reset
- [ ] CI/CD integration
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
