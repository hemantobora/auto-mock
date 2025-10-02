package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/cloud"
	"github.com/hemantobora/auto-mock/internal/state"
	"github.com/hemantobora/auto-mock/internal/terraform"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "automock",
		Usage: "Generate and deploy mock API infrastructure",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "profile",
				Usage: "AWS credential profile name (e.g., dev, prod)",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Initialize AutoMock project with expectations and optional infrastructure deployment",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "project",
						Usage: "Project name (bypasses interactive project selection)",
					},
					&cli.StringFlag{
						Name:  "provider",
						Usage: "LLM provider (anthropic, openai, template) - bypasses provider selection",
					},
					&cli.BoolFlag{
						Name:  "include-auth",
						Usage: "Include authentication endpoints in mock config",
					},
					&cli.BoolFlag{
						Name:  "include-errors",
						Usage: "Include error responses in mock config",
						Value: true,
					},
					&cli.StringFlag{
						Name:  "collection-file",
						Usage: "Path to API collection file (Postman/Bruno/Insomnia)",
					},
					&cli.StringFlag{
						Name:  "collection-type",
						Usage: "Collection type (postman, bruno, insomnia) - required with --collection-file",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")

					cliContext := &cloud.CLIContext{
						ProjectName:    c.String("project"),
						Provider:       c.String("provider"),
						IncludeAuth:    c.Bool("include-auth"),
						IncludeErrors:  c.Bool("include-errors"),
						CollectionFile: c.String("collection-file"),
						CollectionType: c.String("collection-type"),
					}

					return cloud.AutoDetectAndInit(profile, cliContext)
				},
			},
			{
				Name:  "deploy",
				Usage: "Deploy complete infrastructure for existing project",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "project",
						Usage:    "Project name to deploy",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "instance-size",
						Usage: "Instance size (small, medium, large, xlarge)",
						Value: "small",
					},
					&cli.IntFlag{
						Name:  "ttl-hours",
						Usage: "Auto-teardown timeout in hours (0 = disabled)",
						Value: 4,
					},
					&cli.StringFlag{
						Name:  "custom-domain",
						Usage: "Custom domain for the API (optional)",
					},
					&cli.StringFlag{
						Name:  "hosted-zone-id",
						Usage: "Route53 hosted zone ID for custom domain",
					},
					&cli.StringFlag{
						Name:  "notification-email",
						Usage: "Email for TTL expiration notifications",
					},
					&cli.BoolFlag{
						Name:  "skip-confirmation",
						Usage: "Skip deployment confirmation prompt",
					},
				},
				Action: func(c *cli.Context) error {
					return deployCommand(c)
				},
			},
			{
				Name:  "destroy",
				Usage: "Destroy infrastructure for a project",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "project",
						Usage:    "Project name to destroy",
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "force",
						Usage: "Skip confirmation prompts",
					},
				},
				Action: func(c *cli.Context) error {
					return destroyCommand(c)
				},
			},
			{
				Name:  "status",
				Usage: "Show infrastructure status for a project",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "project",
						Usage:    "Project name to check",
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "detailed",
						Usage: "Show detailed information including metrics",
					},
				},
				Action: func(c *cli.Context) error {
					return statusCommand(c)
				},
			},
			{
				Name:  "extend-ttl",
				Usage: "Extend TTL for running infrastructure",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "project",
						Usage:    "Project name",
						Required: true,
					},
					&cli.IntFlag{
						Name:     "hours",
						Usage:    "Additional hours to add to TTL",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					return extendTTLCommand(c)
				},
			},
			{
				Name:   "help",
				Usage:  "Show detailed help and supported features",
				Action: showDetailedHelp,
			},
		},
		Action: func(c *cli.Context) error {
			return cli.ShowAppHelp(c)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// deployCommand handles infrastructure deployment
func deployCommand(c *cli.Context) error {
	profile := c.String("profile")
	projectName := c.String("project")

	fmt.Println("\nDeploying Complete Infrastructure")
	fmt.Println(strings.Repeat("=", 80))

	// Check Terraform installation
	if err := terraform.CheckTerraformInstalled(); err != nil {
		return err
	}

	// Build deployment options from flags
	options := &terraform.DeploymentOptions{
		InstanceSize:      c.String("instance-size"),
		TTLHours:          c.Int("ttl-hours"),
		CustomDomain:      c.String("custom-domain"),
		HostedZoneID:      c.String("hosted-zone-id"),
		NotificationEmail: c.String("notification-email"),
		EnableTTLCleanup:  c.Int("ttl-hours") > 0,
	}

	// Show cost estimate
	minTasks := 10 // Default from terraform
	maxTasks := 200
	terraform.DisplayCostEstimate(minTasks, maxTasks, options.TTLHours)

	// Confirm deployment unless --skip-confirmation
	if !c.Bool("skip-confirmation") {
		if !confirmDeployment(projectName, options) {
			fmt.Println("\nDeployment cancelled")
			return nil
		}
	}

	// Create Terraform manager
	manager := terraform.NewManager(projectName, profile)

	// Deploy infrastructure
	fmt.Println("\nStarting deployment...")
	outputs, err := manager.Deploy(options)
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	// Display results
	terraform.DisplayDeploymentResults(outputs, projectName)

	return nil
}

// destroyCommand handles infrastructure teardown
func destroyCommand(c *cli.Context) error {
	profile := c.String("profile")
	projectName := c.String("project")
	force := c.Bool("force")

	// Show confirmation unless --force
	if !force {
		terraform.DisplayDestroyConfirmation(projectName)

		// First confirmation
		var confirmed bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Type the project name '%s' to confirm:", projectName),
		}

		// Ask for project name confirmation
		var inputName string
		namePrompt := &survey.Input{
			Message: "Enter project name:",
		}

		if err := survey.AskOne(namePrompt, &inputName); err != nil {
			return err
		}

		if inputName != projectName {
			fmt.Println("\nProject name does not match. Deletion cancelled.")
			return nil
		}

		// Final confirmation
		if err := survey.AskOne(prompt, &confirmed); err != nil {
			return err
		}

		if !confirmed {
			fmt.Println("\nDeletion cancelled")
			return nil
		}
	}

	// Create Terraform manager
	manager := terraform.NewManager(projectName, profile)

	// Destroy infrastructure
	fmt.Println("\nDestroying infrastructure...")
	err := manager.Destroy()

	terraform.DisplayDestroyResults(projectName, err == nil)

	return err
}

// statusCommand shows current infrastructure status
func statusCommand(c *cli.Context) error {
	profile := c.String("profile")
	projectName := c.String("project")
	detailed := c.Bool("detailed")

	fmt.Printf("\nChecking infrastructure status for: %s\n", projectName)
	fmt.Println(strings.Repeat("=", 80))

	// Create Terraform manager
	manager := terraform.NewManager(projectName, profile)

	// Get current outputs (this requires terraform to be initialized)
	// For now, we'll use a simpler approach - check AWS directly
	outputs, err := manager.GetCurrentStatus()
	if err != nil {
		// Infrastructure might not exist
		fmt.Println("\nNo infrastructure found for this project.")
		fmt.Println("Run 'automock deploy --project " + projectName + "' to create it.")
		return nil
	}

	// Display status
	if detailed {
		terraform.DisplayStatusInfo(outputs)

		// Show additional details
		if summary, ok := outputs.InfrastructureSummary["compute"].(map[string]interface{}); ok {
			fmt.Println("\nDetailed Metrics:")
			fmt.Printf("  Instance Size: %v\n", summary["instance_size"])
			fmt.Printf("  Min Tasks:     %v\n", summary["min_tasks"])
			fmt.Printf("  Max Tasks:     %v\n", summary["max_tasks"])
			fmt.Printf("  Current Tasks: %v\n", summary["current_tasks"])
		}

		// Show CLI commands
		fmt.Println("\nManagement Commands:")
		for name, cmd := range outputs.CLICommands {
			fmt.Printf("  %s:\n    %s\n\n", name, cmd)
		}
	} else {
		// Simple status
		terraform.DisplayStatusInfo(outputs)
	}

	return nil
}

// extendTTLCommand extends the TTL for running infrastructure
func extendTTLCommand(c *cli.Context) error {
	profile := c.String("profile")
	projectName := c.String("project")
	additionalHours := c.Int("hours")

	fmt.Printf("\nExtending TTL for project: %s\n", projectName)
	fmt.Printf("Adding %d hours to current TTL\n", additionalHours)
	fmt.Println(strings.Repeat("=", 80))

	// This would:
	// 1. Read current metadata from S3
	// 2. Calculate new TTL expiry
	// 3. Update metadata in S3
	// 4. Confirm to user

	err := extendTTL(profile, projectName, additionalHours)
	if err != nil {
		return fmt.Errorf("failed to extend TTL: %w", err)
	}

	fmt.Println("\nTTL extended successfully")
	fmt.Printf("New expiration: %d hours from now\n", additionalHours)

	return nil
}

// confirmDeployment shows final confirmation before deployment
func confirmDeployment(projectName string, options *terraform.DeploymentOptions) bool {
	fmt.Println("\nDeployment Summary")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Project:       %s\n", projectName)
	fmt.Printf("Instance Size: %s\n", options.InstanceSize)
	fmt.Printf("TTL:           %d hours\n", options.TTLHours)

	if options.CustomDomain != "" {
		fmt.Printf("Custom Domain: %s\n", options.CustomDomain)
	}

	if options.NotificationEmail != "" {
		fmt.Printf("Notifications: %s\n", options.NotificationEmail)
	}

	fmt.Println()

	var confirmed bool
	prompt := &survey.Confirm{
		Message: "Proceed with deployment?",
		Default: true,
	}

	survey.AskOne(prompt, &confirmed)
	return confirmed
}

// extendTTL extends the TTL for an existing deployment
func extendTTL(profile, projectName string, additionalHours int) error {
	ctx := context.Background()

	// Create S3 store
	fmt.Println("  Reading current metadata from S3...")
	store, err := state.StoreForProject(ctx, projectName)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	// Get current metadata
	metadata, err := store.GetDeploymentMetadata(ctx)
	if err != nil {
		return fmt.Errorf("failed to get deployment metadata: %w\nIs infrastructure deployed?", err)
	}

	if metadata.DeploymentStatus != "deployed" {
		return fmt.Errorf("infrastructure is not deployed (status: %s)", metadata.DeploymentStatus)
	}

	if metadata.TTLExpiry.IsZero() {
		return fmt.Errorf("no TTL configured for this deployment")
	}

	// Show current TTL info
	fmt.Printf("  Current TTL expiry: %s\n", metadata.TTLExpiry.Format("2006-01-02 15:04:05 MST"))
	remaining := time.Until(metadata.TTLExpiry)
	if remaining > 0 {
		fmt.Printf("  Time remaining: %s\n", remaining.Round(time.Minute))
	} else {
		fmt.Println("  Warning: TTL already expired!")
	}

	// Extend TTL
	fmt.Printf("  Adding %d hours...\n", additionalHours)
	if err := store.ExtendTTL(ctx, additionalHours); err != nil {
		return fmt.Errorf("failed to extend TTL: %w", err)
	}

	// Get updated metadata
	updatedMetadata, _ := store.GetDeploymentMetadata(ctx)
	fmt.Printf("  New TTL expiry: %s\n", updatedMetadata.TTLExpiry.Format("2006-01-02 15:04:05 MST"))
	newRemaining := time.Until(updatedMetadata.TTLExpiry)
	fmt.Printf("  New time remaining: %s\n", newRemaining.Round(time.Minute))

	return nil
}

// showDetailedHelp provides comprehensive help
func showDetailedHelp(c *cli.Context) error {
	help := `
AutoMock - AI-Powered Mock API Infrastructure

BASIC USAGE:
  automock init                    # Interactive mode (recommended)
  automock help                    # Show this help

MAIN COMMANDS:
  automock init --project myapi    # Create/update project and generate expectations
  automock deploy --project myapi  # Deploy infrastructure for existing project  
  automock status --project myapi  # Check infrastructure status
  automock destroy --project myapi # Tear down infrastructure
  automock extend-ttl --project myapi --hours 4  # Extend TTL

WORKFLOW (Interactive Mode):
  1. Run 'automock init'
  2. Select or create project
  3. Generate expectations (AI, collection import, or interactive builder)
  4. Choose deployment option:
     - Save to S3 only
     - Deploy complete infrastructure (ECS + ALB)
     - Start local MockServer
  5. Infrastructure deploys automatically with TTL-based auto-teardown

SUPPORTED LLM PROVIDERS:
  • anthropic    - Claude (requires ANTHROPIC_API_KEY)
  • openai       - GPT-4 (requires OPENAI_API_KEY)
  • template     - Template-based (free, always available)

SUPPORTED COLLECTION FORMATS:
  • postman      - Postman Collection v2.1 (.json)
  • bruno        - Bruno Collection (.json or .bru)
  • insomnia     - Insomnia Workspace (.json)

INFRASTRUCTURE:
  • Complete     - ECS Fargate + ALB + Auto-scaling + TTL cleanup
  • Auto-scaling - 10-200 tasks based on CPU/Memory/Requests
  • TTL Cleanup  - Automatic teardown after expiration (default: 8 hours)
  • Cost         - ~$1.24/hour with 10 tasks, auto-teardown prevents runaway costs

ENVIRONMENT VARIABLES:
  AWS_PROFILE              # AWS profile to use
  ANTHROPIC_API_KEY        # For Claude AI generation
  OPENAI_API_KEY           # For GPT-4 AI generation

EXAMPLES:
  # Interactive mode (recommended for first-time users)
  automock init

  # Quick project with AI generation
  automock init --project user-service --provider anthropic

  # Import Postman collection and deploy
  automock init --collection-file api.json --collection-type postman

  # Deploy standalone (expectations already exist)
  automock deploy --project user-service --ttl-hours 4

  # Check what's running
  automock status --project user-service

  # Extend running infrastructure
  automock extend-ttl --project user-service --hours 4

  # Clean up
  automock destroy --project user-service

COST ESTIMATES (with TTL):
  • 8 hours  = ~$10
  • 40 hours/month (5 days × 8 hours) = ~$50/month
  • Without TTL (24/7) = ~$125/month

For more information: See INFRASTRUCTURE.md and CLI_INTEGRATION.md
`

	fmt.Print(help)
	return nil
}
