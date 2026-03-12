package aws

import (
	"context"
	"fmt"

	"github.com/hemantobora/auto-mock/internal/models"
)

func PrintECSRoleIAMPolicies() {
	fmt.Println("\n───────────────────────────────")
	fmt.Println("📜 ECS TASK ROLE:")
	fmt.Println("───────────────────────────────")
	fmt.Println(`Use the following trust policy when creating this role.
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "",
            "Effect": "Allow",
            "Principal": {
                "Service": "ecs-tasks.amazonaws.com"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}	
How to create: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-iam-roles.html
Steps:
  1. Go to IAM → "Create Role"
  2. Select "AWS Service" → choose "Elastic Container Service"
  3. Select "Task Role for Elastic Container Service"
  4. Click "Next" twice → name the role (e.g., auto-mock-ecs-task-role)
  5. Click "Create Role"`)

	fmt.Println("\nAttach this inline policy (S3 read + KMS decrypt):")
	fmt.Println(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject"],
      "Resource": "arn:aws:s3:::auto-mock-*/*"
    },
    {
      "Effect": "Allow",
      "Action": ["s3:ListBucket"],
      "Resource": "arn:aws:s3:::auto-mock-*"
    },
    {
      "Effect": "Allow",
      "Action": ["kms:Decrypt","kms:DescribeKey"],
      "Resource": [
        "arn:aws:kms:*:*:key/*",
        "arn:aws:kms:*:*:alias/auto-mock-*"
      ]
    }
  ]
}`)
	fmt.Println()
	fmt.Println()
}

// PrintIAMPolicies prints clear step-by-step guidance and the minimal JSON policies
func PrintECSIAMPolicies() {
	fmt.Println("\n───────────────────────────────")
	fmt.Println("📜 ECS EXECUTION ROLE:")
	fmt.Println("───────────────────────────────")

	fmt.Println(`Use the following trust policy when creating this role.
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "",
            "Effect": "Allow",
            "Principal": {
                "Service": "ecs-tasks.amazonaws.com"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}

How to create: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html
Steps:
  1. Go to IAM → "Create Role"
  2. Select "AWS Service" → choose "Elastic Container Service"
  3. Select "Task Execution Role for Elastic Container Service"
  4. Click "Next" twice → name the role (e.g., auto-mock-ecs-execution-role)
  5. Click "Create Role"`)

	fmt.Println("\nAttach the managed policy:")
	fmt.Println("  • AmazonECSTaskExecutionRolePolicy")
	fmt.Println()
	fmt.Println()
}

func (p *Provider) CreateDeploymentConfiguration() *models.DeploymentOptions {
	// ── 1) Collect capabilities + BYO inputs (survey) ─────────────────────────
	fmt.Println("\n🔍 Running pre-deployment checks...")
	cap, in, err := p.promptCapabilityAndInputs(context.Background())
	if err != nil {
		return nil
	}
	fmt.Println("✓ Pre-deployment checks complete")

	// ── 2) Build Terraform options from capability/inputs ─────────────────────
	options, err := assembleOptions(*cap, *in) // uses deriveUseExisting + validateInputs
	if err != nil {
		return nil
	}
	fmt.Println("✓ Networking configuration complete")
	options.ProjectName = p.GetProjectName()
	options.Region = p.GetRegion()
	options.BucketName = p.BucketName
	options.Provider = p.GetProviderType()
	// ── 3) Final confirmation/review ─────────────────────────────────────────
	promptDeploymentOptionsREPL(options)
	return options
}

// CreateDefaultDeploymentConfiguration returns a minimal DeploymentOptions used
// by the destroy command.  It restores CustomDomain from the persisted deployment
// metadata so that Terraform evaluates the same conditional paths (self-signed vs
// ACM) that were active when the infrastructure was created.
func (p *Provider) CreateDefaultDeploymentConfiguration() *models.DeploymentOptions {
	opts := &models.DeploymentOptions{
		InstanceSize: "small",
		Region:       p.GetRegion(),
		BucketName:   p.BucketName,
		ProjectName:  p.GetProjectName(),
		Provider:     p.GetProviderType(),
	}

	// Restore flags from prior deployment so destroy tfvars exactly match
	// what was originally applied. We always restore when metadata is present,
	// not just when CustomDomain is set — PrivateALB = false is meaningful.
	if md, err := p.GetDeploymentMetadata(); err == nil {
		opts.CustomDomain = md.CustomDomain
		opts.CreateHostedZone = md.CreateHostedZone
		opts.PrivateALB = md.PrivateALB
	}

	return opts
}
