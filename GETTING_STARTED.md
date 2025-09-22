# AutoMock - Getting Started Guide

## Quick Setup

1. **Set your AI provider API key:**
```bash
export ANTHROPIC_API_KEY="your-key-here"
# OR
export OPENAI_API_KEY="your-key-here"
```

2. **Build AutoMock:**
```bash
chmod +x build.sh
./build.sh
```

3. **Run AutoMock:**
```bash
./automock init --project my-api
```

## How It Works

AutoMock now properly leverages AI to generate MockServer configurations. Here's the improved flow:

### 🎯 Three Generation Modes

#### 1. Quick Mode - Natural Language
Simply describe your API in plain English:
- "User management API with CRUD operations"
- "E-commerce API with products, cart, and checkout"
- "Banking API with accounts, transactions, and authentication"

The AI will generate comprehensive MockServer configuration with:
- All necessary endpoints
- Request matching rules
- Success responses with realistic data
- Error responses (400, 401, 404, 500)
- Proper headers and CORS

#### 2. Detailed Mode - Request/Response Examples
Provide specific examples of requests and responses:
- Define exact paths and methods
- Paste sample request bodies
- Provide example response JSONs
- AI fills in the gaps and ensures consistency

#### 3. Import Mode (Coming Soon)
Import from:
- OpenAPI/Swagger specs
- Postman collections
- Insomnia workspaces
- Bruno collections

## Core Features Working Now

✅ **AI-Powered Generation**
- Uses Claude/GPT-4 to generate MockServer configs
- Understands natural language descriptions
- Creates realistic test data
- Follows REST best practices

✅ **S3 State Management**
- Saves configurations to S3
- Version tracking
- Project isolation

✅ **Lambda Deployment (Ready)**
- Lambda handler implemented
- Can process MockServer expectations
- Function URL support for easy access

✅ **Interactive REPL**
- Simplified, focused interface
- Clear step-by-step process
- Immediate AI assistance

## Example Usage

### Quick API Generation
```bash
./automock init --project user-api

# Choose: "Quick - Describe API in natural language"
# Enter: "User management system with registration, login, profile management, and admin functions"
# Select features: Authentication, CRUD, Error responses
# AI generates complete MockServer configuration
```

### Detailed Example-Based
```bash
./automock init --project order-api

# Choose: "Detailed - Provide request/response examples"
# Add endpoint: POST /api/orders
# Paste request body: {"product_id": "123", "quantity": 2}
# Paste response: {"order_id": "ord_456", "status": "pending"}
# AI generates complete configuration with all edge cases
```

## Testing Your Setup

Run the test script to verify everything works:
```bash
go run test_core.go
```

This will:
1. Test AI mock generation
2. Verify provider availability
3. Generate a sample configuration

## MockServer Configuration Format

AutoMock generates standard MockServer JSON:

```json
[
  {
    "httpRequest": {
      "method": "GET",
      "path": "/api/users/{id}"
    },
    "httpResponse": {
      "statusCode": 200,
      "headers": {
        "Content-Type": "application/json"
      },
      "body": {
        "id": "123",
        "name": "John Doe",
        "email": "john@example.com"
      }
    }
  }
]
```

## What's Next?

### Ready to Use
- Generate mock configurations with AI
- Save to S3
- Export to files
- Lambda function ready

### Coming Soon
- Full AWS deployment (ECS/Fargate)
- TTL and auto-cleanup
- Collection imports
- Web UI

## Troubleshooting

### No API Key
```
❌ ANTHROPIC_API_KEY environment variable not set
```
**Solution:** Export your API key before running

### Build Errors
```
go: errors parsing go.mod
```
**Solution:** Run `go mod tidy`

### S3 Access Issues
```
failed to create bucket
```
**Solution:** Check AWS credentials and permissions

## Support

For issues or questions:
- Check the test script: `go run test_core.go`
- Review logs for detailed error messages
- Ensure AWS credentials are configured

---

## Architecture Overview

```
User Input → REPL → AI Provider → MockServer JSON → S3/Lambda
     ↓          ↓         ↓              ↓            ↓
   Natural   Examples  Claude/GPT   Validated    Deployed
   Language             Generated     Config       Mock
```

The system is now properly leveraging AI to generate high-quality mock configurations from minimal user input.
