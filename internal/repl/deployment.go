// internal/repl/deployment.go
// REPL deployment integration with Terraform
package repl

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/terraform"
)

type Deployment struct {
	ProjectName string
	Options     *terraform.DeploymentOptions
	Provider    internal.Provider
	Profile     string
}

// NewDeployment creates a new Deployment instance
func NewDeployment(projectName, profile string, provider internal.Provider, options *terraform.DeploymentOptions) *Deployment {
	return &Deployment{
		ProjectName: projectName,
		Options:     options,
		Provider:    provider,
		Profile:     profile,
	}
}

// DeployInfrastructureWithTerraform deploys actual infrastructure using Terraform
func (d *Deployment) DeployInfrastructureWithTerraform(skip_confirmation bool) error {

	fmt.Println("\n🏗️  Complete Infrastructure Deployment")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Check Terraform installation
	if err := terraform.CheckTerraformInstalled(); err != nil {
		return fmt.Errorf("terraform not found: %w\nPlease install from https://terraform.io/downloads", err)
	}

	// Prompt for deployment options
	err := promptDeploymentOptionsREPL(d.Options)
	if err != nil {
		return err
	}

	if d.Provider.GetProviderType() == "aws" {
		// Show cost estimate
		terraform.DisplayAwsCostEstimate(*d.Options)
	}

	// Confirm deployment
	if !skip_confirmation {
		var confirmed bool
		confirmPrompt := &survey.Confirm{
			Message: "Proceed with infrastructure deployment?",
			Default: true,
			Help:    "This will create ECS Fargate cluster, ALB, and supporting resources",
		}

		if err := survey.AskOne(confirmPrompt, &confirmed); err != nil {
			return err
		}

		if !confirmed {
			fmt.Println("\n❌ Deployment cancelled")
			return nil
		}
	}

	// Create Terraform manager
	// The manager will automatically find the existing S3 bucket
	manager, err := terraform.NewManager(d.ProjectName, d.Profile, d.Provider)
	if err != nil {
		return fmt.Errorf("failed to create terraform manager: %w", err)
	}

	// Validate bucket was found
	if manager.ExistingBucketName == "" {
		return fmt.Errorf("❌ No S3 bucket found for project '%s'. Please run 'automock init --project %s' first to create the project", d.ProjectName, d.ProjectName)
	}

	fmt.Printf("\n✓ Found existing bucket: %s\n", manager.ExistingBucketName)

	// Deploy infrastructure
	fmt.Println("\n🚀 Deploying infrastructure with Terraform...")
	outputs, err := manager.Deploy(d.Options)
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	// Display results
	terraform.DisplayDeploymentResults(outputs, d.ProjectName)

	return nil
}

func validateScalingConfiguration(minTasks, maxTasks int) error {
	// For percentage-based scaling with +200% max adjustment
	recommendedMax := minTasks * 6
	absoluteMinMax := minTasks * 3

	if maxTasks < absoluteMinMax {
		return fmt.Errorf(
			"max_tasks (%d) is too low for min_tasks (%d)\n"+
				"  With +200%% scaling, you need at least: %d tasks\n"+
				"  Recommended max: %d tasks",
			maxTasks, minTasks, absoluteMinMax, recommendedMax)
	}

	if maxTasks < recommendedMax {
		fmt.Printf("⚠️  Warning: max_tasks (%d) may be too low for optimal scaling\n", maxTasks)
		fmt.Printf("   Recommended max for min=%d: %d tasks\n", minTasks, recommendedMax)
		fmt.Printf("   Current max allows only %.1fx growth\n", float64(maxTasks)/float64(minTasks))
	}

	return nil
}

// promptDeploymentOptionsREPL prompts for deployment configuration in REPL
func promptDeploymentOptionsREPL(options *terraform.DeploymentOptions) error {

	fmt.Println("\n⚙️  Deployment Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Your size map (cpu in CPU units; memory in MiB)
	taskConfig := map[string]struct{ CPU, MemMiB int }{
		"small":  {CPU: 256, MemMiB: 512},   // 0.25 vCPU, 0.5 GB
		"medium": {CPU: 512, MemMiB: 1024},  // 0.5 vCPU, 1 GB
		"large":  {CPU: 1024, MemMiB: 2048}, // 1 vCPU, 2 GB
		"xlarge": {CPU: 2048, MemMiB: 4096}, // 2 vCPU, 4 GB
	}

	// Instance size
	if options.InstanceSize == "" {
		var instanceSize string
		sizePrompt := &survey.Select{
			Message: "Select instance size:",
			Options: []string{"small", "medium", "large", "xlarge"},
			Default: "small",
			Description: func(value string, index int) string {
				switch value {
				case "small":
					return "0.5 vCPU, 1GB RAM (recommended for testing)"
				case "medium":
					return "1 vCPU, 2GB RAM (moderate load)"
				case "large":
					return "2 vCPU, 4GB RAM (high load)"
				case "xlarge":
					return "4 vCPU, 8GB RAM (very high load)"
				default:
					return ""
				}
			},
		}

		if err := survey.AskOne(sizePrompt, &instanceSize); err != nil {
			return err
		}
		options.InstanceSize = instanceSize
	}

	cfg, ok := taskConfig[options.InstanceSize]
	if !ok {
		return fmt.Errorf("  Unknown size %q. Valid: small, medium, large, xlarge", options.InstanceSize)
	}
	options.CPUUnits = cfg.CPU
	options.MemoryUnits = cfg.MemMiB

	// Min tasks
	if options.MinTasks == 0 {
		var minTask string
		minPrompt := &survey.Input{
			Message: "Minimum number of tasks (Fargate instances):",
			Default: "5",
			Help:    "Minimum number of Fargate tasks to run (scales between min and max based on load)",
		}

		if err := survey.AskOne(minPrompt, &minTask); err != nil {
			return err
		}

		// Convert to int
		var minTaskInt int
		fmt.Sscanf(minTask, "%d", &minTaskInt)
		options.MinTasks = minTaskInt

		// Validate min > 0
		if options.MinTasks <= 0 {
			return fmt.Errorf("minimum tasks must be greater than zero")
		}
	}

	// Max tasks
	if options.MaxTasks == 0 {
		// Calculate recommended max
		recommendedMax := options.MinTasks * 9

		for {
			var maxTask string
			maxPrompt := &survey.Input{
				Message: fmt.Sprintf("Maximum number of tasks (Fargate instances) [recommended: %d]:", recommendedMax),
				Default: fmt.Sprintf("%d", recommendedMax),
				Help:    fmt.Sprintf("Maximum number of Fargate tasks to run (scales between min and max based on load). Recommended: %d (min × 6 for optimal scaling)", recommendedMax),
			}

			if err := survey.AskOne(maxPrompt, &maxTask); err != nil {
				return err
			}

			// Convert to int
			var maxTaskInt int
			if _, err := fmt.Sscanf(maxTask, "%d", &maxTaskInt); err != nil || maxTaskInt <= 0 {
				fmt.Println("❌ Invalid input. Please enter a positive number.")
				continue
			}
			options.MaxTasks = maxTaskInt

			// Validate max >= min
			if options.MaxTasks < options.MinTasks {
				fmt.Printf("❌ Maximum tasks (%d) must be greater than or equal to minimum tasks (%d)\n",
					options.MaxTasks, options.MinTasks)
				continue
			}

			// Validate scaling configuration
			if err := validateScalingConfiguration(options.MinTasks, options.MaxTasks); err != nil {
				fmt.Printf("❌ %v\n", err)

				// Ask if they want to continue anyway
				var continueAnyway bool
				continuePrompt := &survey.Confirm{
					Message: "Continue with this configuration anyway?",
					Default: false,
				}
				if err := survey.AskOne(continuePrompt, &continueAnyway); err != nil {
					return err
				}

				if !continueAnyway {
					continue // Go back to max tasks input
				}
			}

			// All validations passed, break out of loop
			break
		}
	}

	// TTL hours
	if options.TTLHours == 0 {
		var ttlHours string
		ttlPrompt := &survey.Input{
			Message: "Auto-teardown timeout (hours, 0 = disabled):",
			Default: "4",
			Help:    "Infrastructure will be automatically deleted after this time to prevent runaway costs",
		}

		if err := survey.AskOne(ttlPrompt, &ttlHours); err != nil {
			return err
		}

		// Convert to int
		var ttlInt int
		fmt.Sscanf(ttlHours, "%d", &ttlInt)
		options.TTLHours = ttlInt
		options.EnableTTLCleanup = ttlInt > 0

		// IAM Role Configuration (if TTL enabled)
		if ttlInt > 0 {
			mode, roleARN, err := promptIAMRoleConfiguration()
			if err != nil {
				return err
			}

			options.IAMRoleMode = mode
			options.CleanupRoleARN = roleARN

			// Auto-cleanup is only enabled when mode is "create"
			// If user provides their own role or skips, we can't auto-cleanup
			switch mode {
			case "skip":
				options.EnableTTLCleanup = false
				fmt.Println("⚠️  Auto-cleanup disabled (user chose skip)")
			case "provided":
				options.EnableTTLCleanup = false
				fmt.Println("⚠️  Auto-cleanup disabled (user-provided role cannot be deleted)")
			case "create":
				options.EnableTTLCleanup = true
				fmt.Println("✓ Auto-cleanup enabled (auto-mock will create and manage cleanup resources)")
			}
		}

		// Notification email (if TTL enabled)
		if ttlInt > 0 {
			var emailWanted bool
			emailPrompt := &survey.Confirm{
				Message: "Receive notification before auto-teardown?",
				Default: false,
			}

			if err := survey.AskOne(emailPrompt, &emailWanted); err != nil {
				return err
			}

			if emailWanted {
				var email string
				emailInputPrompt := &survey.Input{
					Message: "Notification email:",
				}

				if err := survey.AskOne(emailInputPrompt, &email); err != nil {
					return err
				}
				options.NotificationEmail = email
			}
		}
	}
	// Custom domain (optional)
	var useCustomDomain bool
	domainPrompt := &survey.Confirm{
		Message: "Use custom domain?",
		Default: false,
	}

	if err := survey.AskOne(domainPrompt, &useCustomDomain); err != nil {
		return err
	}

	if useCustomDomain {
		var domain string
		domainInputPrompt := &survey.Input{
			Message: "Custom domain (e.g., api.example.com):",
		}

		if err := survey.AskOne(domainInputPrompt, &domain); err != nil {
			return err
		}
		options.CustomDomain = domain

		var hostedZoneID string
		zonePrompt := &survey.Input{
			Message: "Route53 Hosted Zone ID:",
		}

		if err := survey.AskOne(zonePrompt, &hostedZoneID); err != nil {
			return err
		}
		options.HostedZoneID = hostedZoneID
	}

	return nil
}

func promptIAMRoleConfiguration() (string, string, error) {
	var mode string
	modePrompt := &survey.Select{
		Message: "How do you want to handle IAM roles for auto-cleanup?",
		Options: []string{
			"provided - I'll provide an existing role ARN (Recommended if organization governed, no auto-teardown)",
			"create - Let auto-mock create roles (requires iam:CreateRole)",
			"skip - Skip cleanup feature (manual teardown only)",
		},
		Default: "skip - Skip cleanup feature (manual teardown only)",
		Help:    "Auto-cleanup requires IAM roles. Choose how to handle them.",
	}

	if err := survey.AskOne(modePrompt, &mode); err != nil {
		return "", "", err
	}

	mode = strings.Split(mode, " ")[0]

	switch mode {
	case "provided":
		return promptForExistingRole()
	case "create":
		return "create", "", nil
	case "skip":
		fmt.Println("\n⚠️  Auto-cleanup disabled. You must manually destroy infrastructure.")
		return "skip", "", nil
	default:
		return "skip", "", nil
	}
}

func promptForExistingRole() (string, string, error) {
	// Display required policy
	fmt.Println("\n📋 Required IAM Policy for Cleanup Role")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	policy := generateCleanupPolicyJSON()
	fmt.Println(policy)

	fmt.Println("\nTrust Relationship:")
	fmt.Println(generateTrustPolicyJSON())

	// Prompt for ARN
	var roleARN string
	arnPrompt := &survey.Input{
		Message: "Enter the IAM Role ARN:",
		Help:    "Format: arn:aws:iam::123456789012:role/role-name",
	}

	if err := survey.AskOne(arnPrompt, &roleARN); err != nil {
		return "", "", err
	}

	// Validate ARN format
	if !strings.HasPrefix(roleARN, "arn:aws:iam::") {
		return "", "", fmt.Errorf("invalid role ARN format")
	}

	// Validate role exists
	fmt.Println("\n🔍 Validating role...")
	if err := validateIAMRole(roleARN); err != nil {
		return "", "", fmt.Errorf("role validation failed: %w", err)
	}

	fmt.Println("✓ Role validated")
	fmt.Println("\n⚠️  Note: User-provided roles are not eligible for auto-teardown")
	fmt.Println("    Infrastructure must be destroyed manually")

	return "provided", roleARN, nil
}

func generateCleanupPolicyJSON() string {
	return `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ECSCleanup",
      "Effect": "Allow",
      "Action": [
        "ecs:UpdateService",
        "ecs:DeleteService",
        "ecs:DeleteCluster",
        "ecs:DescribeServices",
        "ecs:DescribeClusters"
      ],
      "Resource": "arn:aws:ecs:*:*:*"
    },
    {
      "Sid": "LoadBalancerCleanup",
      "Effect": "Allow",
      "Action": [
        "elasticloadbalancing:DeleteLoadBalancer",
        "elasticloadbalancing:DeleteTargetGroup",
        "elasticloadbalancing:DescribeLoadBalancers",
        "elasticloadbalancing:DescribeTargetGroups"
      ],
      "Resource": "*"
    },
    {
      "Sid": "NetworkCleanup",
      "Effect": "Allow",
      "Action": [
        "ec2:DeleteVpc",
        "ec2:DeleteSubnet",
        "ec2:DeleteSecurityGroup",
        "ec2:DeleteRouteTable",
        "ec2:DeleteInternetGateway",
        "ec2:DeleteNatGateway",
        "ec2:ReleaseAddress",
        "ec2:DetachInternetGateway",
        "ec2:DisassociateRouteTable",
        "ec2:DescribeVpcs",
        "ec2:DescribeSubnets",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeRouteTables",
        "ec2:DescribeInternetGateways",
        "ec2:DescribeNatGateways",
        "ec2:DescribeAddresses"
      ],
      "Resource": "*"
    },
    {
      "Sid": "S3MetadataUpdate",
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject"
      ],
      "Resource": "arn:aws:s3:::auto-mock-*/*"
    },
    {
      "Sid": "CloudWatchLogs",
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ],
      "Resource": "arn:aws:logs:*:*:*"
    }
  ]
}`
}

func generateTrustPolicyJSON() string {
	return `{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": { "Service": "lambda.amazonaws.com" },
    "Action": "sts:AssumeRole"
  }]
}`
}

func validateIAMRole(roleARN string) error {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	iamClient := iam.NewFromConfig(cfg)

	// Extract role name from ARN
	parts := strings.Split(roleARN, "/")
	roleName := parts[len(parts)-1]

	// Check if role exists
	_, err = iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: &roleName,
	})

	if err != nil {
		return fmt.Errorf("role not found or no permission to access: %w", err)
	}

	return nil
}
