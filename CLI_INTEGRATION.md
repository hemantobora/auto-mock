# CLI Integration Guide for AutoMock Infrastructure

> **Purpose**: Complete guide for understanding how the CLI integrates with Terraform infrastructure deployment.  
> **Date**: 2025-01-05  
> **For**: Future development and maintenance

---

## Overview

The AutoMock CLI provides two paths for infrastructure deployment:
1. **Interactive (REPL)** - Integrated into `automock init` workflow
2. **Standalone Commands** - Direct deployment management

This document explains how everything fits together.

---

## Architecture Flow

```
User Input
    ↓
CLI Command (cmd/auto-mock/main.go)
    ↓
Command Handler (cmd/auto-mock/infrastructure.go)
    ↓
Terraform Manager (internal/terraform/manager.go)
    ↓
Terraform Modules (terraform/modules/*)
    ↓
AWS Infrastructure
```

---

## File Structure & Responsibilities

### 1. CLI Entry Point
**File**: `cmd/auto-mock/main.go`

**Purpose**: Defines all CLI commands and routes to handlers

**Commands**:
- `init` - Main workflow (generate expectations + optional deploy)
- `deploy` - Deploy infrastructure for existing project
- `destroy` - Tear down infrastructure
- `status` - Check infrastructure status
- `extend-ttl` - Extend TTL for running infrastructure

### 2. Command Handlers
**File**: `cmd/auto-mock/infrastructure.go`

**Purpose**: Implements command logic, user prompts, confirmations

**Functions**:
- `deployCommand()` - Handles deploy command
- `destroyCommand()` - Handles destroy with confirmation
- `statusCommand()` - Shows infrastructure status
- `extendTTLCommand()` - Extends TTL
- `confirmDeployment()` - User confirmation helper

### 3. Terraform Manager
**File**: `internal/terraform/manager.go`

**Purpose**: Orchestrates Terraform operations

**Key Methods**:
- `Deploy(options)` - Deploy infrastructure
- `Destroy()` - Tear down infrastructure
- `GetCurrentStatus()` - Get deployed state
- `CheckInfrastructureExists()` - Quick existence check

### 4. Display Functions
**File**: `internal/terraform/display.go`

**Purpose**: User-friendly output formatting

### 5. Status Checking
**File**: `internal/terraform/status.go`

**Purpose**: Check infrastructure without full Terraform init

---

## Command Usage Examples

### Deploy Command
```bash
# Basic
automock deploy --project user-api

# With options
automock deploy --project user-api --instance-size medium --ttl-hours 4

# With custom domain
automock deploy --project user-api --custom-domain api.example.com --hosted-zone-id Z123

# Skip confirmation (CI/CD)
automock deploy --project user-api --skip-confirmation
```

### Destroy Command
```bash
# Safe (with confirmation)
automock destroy --project user-api

# Force (no confirmation)
automock destroy --project user-api --force
```

### Status Command
```bash
# Basic
automock status --project user-api

# Detailed
automock status --project user-api --detailed
```

### Extend TTL Command
```bash
# Add hours
automock extend-ttl --project user-api --hours 4
```

---

## REPL Integration (TODO)

### Current State
**Status**: NOT IMPLEMENTED YET

### Where to Add
**File**: `internal/repl/repl.go`

### Implementation Plan

1. **Add deployment method to REPL**:
```go
func (r *REPL) deployInfrastructure() error {
    // 1. Prompt for deployment options
    options := r.promptDeploymentOptions()
    
    // 2. Show cost estimate
    terraform.DisplayCostEstimate(10, 200, options.TTLHours)
    
    // 3. Confirm
    if !r.confirmDeployment(options) {
        return nil
    }
    
    // 4. Deploy
    manager := terraform.NewManager(r.projectName, r.awsProfile)
    outputs, err := manager.Deploy(options)
    if err != nil {
        return err
    }
    
    // 5. Display results
    terraform.DisplayDeploymentResults(outputs, r.projectName)
    
    return nil
}
```

2. **Update result handling menu**:
```go
options := []string{
    "Save to S3 only",
    "Deploy complete infrastructure (ECS + ALB)",
    "Start local MockServer",
    "Exit without saving",
}
```

3. **Wire up selection**:
```go
switch choice {
case "Deploy complete infrastructure (ECS + ALB)":
    return r.deployInfrastructure()
}
```

---

## Data Structures

### DeploymentOptions
```go
type DeploymentOptions struct {
    InstanceSize      string  // small, medium, large, xlarge
    TTLHours          int     // 0 = disabled
    CustomDomain      string  // Optional
    HostedZoneID      string  // For custom domain
    NotificationEmail string  // For TTL alerts
    EnableTTLCleanup  bool    // Auto-set based on TTLHours
}
```

### InfrastructureOutputs
```go
type InfrastructureOutputs struct {
    MockServerURL         string
    DashboardURL          string
    ConfigBucket          string
    IntegrationSummary    map[string]interface{}
    CLICommands           map[string]string
    InfrastructureSummary map[string]interface{}
}
```

---

## Error Handling

### Common Errors

**1. Terraform not found**
- Error: `terraform not found in PATH`
- Solution: Install Terraform from https://terraform.io/downloads
- Check: `terraform version`

**2. AWS credentials not configured**
- Error: `NoCredentialProviders: no valid providers in chain`
- Solution: Run `aws configure --profile dev`
- Check: `aws sts get-caller-identity`

**3. S3 bucket already exists**
- Error: `BucketAlreadyExists: The requested bucket name is not available`
- Solution: Choose different project name or delete old bucket
- Check: `aws s3 ls | grep auto-mock`

**4. Terraform state locked**
- Error: `Error locking state: ConditionalCheckFailedException`
- Solution: Wait for other operation to complete or force unlock
- Check: `terraform force-unlock <lock-id>`

**5. ECS task fails to start**
- Error: Task stops immediately after starting
- Common causes:
  - Config loader can't read S3 (IAM permissions)
  - Expectations file doesn't exist
  - MockServer port conflict
- Solution: Check CloudWatch logs
- Check: `aws logs tail /ecs/automock/{project}/config-loader --follow`

---

## Testing the CLI

### Manual Testing Checklist

```bash
# 1. Test deploy
automock deploy --project test-cli --ttl-hours 1
# Expected: Infrastructure deploys, URLs shown

# 2. Test status
automock status --project test-cli
# Expected: Shows running infrastructure

# 3. Test extend-ttl
automock extend-ttl --project test-cli --hours 2
# Expected: TTL updated

# 4. Wait for TTL
sleep 3600
# Expected: Infrastructure auto-deleted

# 5. Test status after TTL
automock status --project test-cli
# Expected: No infrastructure found

# 6. Test destroy
automock deploy --project test-cli --ttl-hours 0
automock destroy --project test-cli
# Expected: Confirmation prompts, then deletion
```

### Integration Testing

Test the full workflow:
```bash
# Full cycle test
automock init --project integration-test
# Select "Deploy complete infrastructure"
# Verify deployment succeeds
# Test endpoints
# Run destroy
```

---

## Deployment Metadata

### Stored in S3
**File**: `s3://{bucket}/deployment-metadata.json`

**Structure**:
```json
{
  "project_name": "user-api",
  "deployment_status": "deployed",
  "deployed_at": "2025-01-05T10:00:00Z",
  "ttl_hours": 8,
  "ttl_expiry": "2025-01-05T18:00:00Z",
  "infrastructure": {
    "cluster_name": "automock-user-api-dev",
    "service_name": "automock-user-api-dev-service",
    "alb_dns": "automock-user-api-alb-123.us-east-1.elb.amazonaws.com",
    "vpc_id": "vpc-123",
    "region": "us-east-1"
  },
  "options": {
    "instance_size": "small",
    "min_tasks": 10,
    "max_tasks": 200,
    "custom_domain": ""
  }
}
```

### Why Store Metadata?

1. **Status checking** - Quick check without Terraform
2. **TTL tracking** - Lambda reads this for cleanup decisions
3. **History** - Track deployment events
4. **CLI integration** - REPL can show deployment status

---

## Cost Tracking

### Cost by Command

| Command | Typical Cost | Duration |
|---------|--------------|----------|
| deploy | ~$1.24/hour | Ongoing |
| status | $0 | Instant |
| extend-ttl | $0 | Instant |
| destroy | $0 | 5-10 min |

### Monthly Cost Examples

**Conservative User** (5 days × 8 hours):
- Base: 40 hours × $1.24 = $49.60
- Occasional scaling: +$5-10
- **Total: ~$55-60/month**

**Heavy User** (20 days × 8 hours):
- Base: 160 hours × $1.24 = $198.40
- Frequent scaling: +$20-30
- **Total: ~$220-230/month**

**Forgot TTL** (24/7):
- Base: 730 hours × $1.24 = $905.20
- **Total: ~$900/month** (This is why TTL is critical!)

---

## Implementation Status

### Completed
- CLI command definitions
- Command handler functions
- Terraform manager integration
- Display functions
- Status checking (basic)
- Documentation

### TODO (Not Implemented)
- REPL integration (Option A)
- TTL extension logic
- Deployment metadata storage
- Advanced status checking
- Error recovery mechanisms
- Unit tests
- Integration tests

### Testing Required
- End-to-end deployment flow
- TTL cleanup verification
- Error scenarios
- Cost validation
- Performance under load

---

## Troubleshooting Guide

### Deployment Fails

**Check Prerequisites**:
```bash
terraform version  # Must be >= 1.0
aws sts get-caller-identity  # AWS credentials work
aws s3 ls  # S3 access works
```

**Check Terraform State**:
```bash
cd terraform
terraform init
terraform state list  # What resources exist?
```

**Check Logs**:
```bash
# Terraform logs
export TF_LOG=DEBUG
automock deploy --project test

# ECS logs
aws logs tail /ecs/automock/test/mockserver --follow
aws logs tail /ecs/automock/test/config-loader --follow
```

### Status Command Fails

**Symptoms**: `No infrastructure found` but resources exist

**Causes**:
1. Terraform state out of sync
2. Wrong AWS profile/region
3. Manually deleted resources

**Solutions**:
```bash
# Check if resources actually exist
aws ecs describe-clusters --clusters automock-{project}-dev
aws s3 ls | grep auto-mock-{project}

# Re-initialize Terraform
cd terraform
terraform init
terraform refresh
```

### TTL Not Working

**Check Lambda**:
```bash
# Lambda exists?
aws lambda get-function --function-name automock-{project}-dev-ttl-cleanup

# EventBridge rule enabled?
aws events describe-rule --name automock-{project}-dev-ttl-check

# Check metadata has TTL
aws s3 cp s3://auto-mock-{project}-config-{suffix}/project-metadata.json -
```

**Manual Trigger**:
```bash
# Test Lambda manually
aws lambda invoke \
  --function-name automock-{project}-dev-ttl-cleanup \
  --payload '{}' \
  response.json
  
cat response.json
```

---

## Future Improvements

### Short Term
1. Implement REPL deployment integration
2. Add deployment metadata storage
3. Implement TTL extension logic
4. Add unit tests
5. Improve error messages

### Medium Term
1. Add rollback capability
2. Support blue/green deployments
3. Add deployment history
4. Implement cost alerts
5. Add CloudWatch dashboards

### Long Term
1. Multi-region support
2. GCP/Azure providers
3. Kubernetes deployment option
4. GitOps integration
5. Advanced monitoring

---

## References

### Related Files
- `INFRASTRUCTURE.md` - Complete architecture
- `terraform/README.md` - Terraform module docs
- `IMPLEMENTATION_SUMMARY.md` - What was built
- `PROJECT_CONTEXT.md` - Overall project context

### External Documentation
- [Terraform CLI](https://www.terraform.io/docs/cli)
- [AWS ECS](https://docs.aws.amazon.com/ecs/)
- [AWS CLI](https://docs.aws.amazon.com/cli/)

---

**Status**: CLI commands implemented, REPL integration pending  
**Next Step**: Test deployment flow and implement REPL integration  
**Testing**: Manual testing recommended before production use
