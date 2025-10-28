package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/client"
	"github.com/hemantobora/auto-mock/internal/cloud"
	"github.com/hemantobora/auto-mock/internal/models"
	"github.com/hemantobora/auto-mock/internal/repl"
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
				Usage: "Credential profile name (e.g., dev, prod)",
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
					// === Other Options ===
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
				Name:  "locust",
				Usage: "Generate Locust load testing bundle from API collection",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "collection-file",
						Usage: "Path to API collection file (Postman/Bruno/Insomnia)",
					},
					&cli.StringFlag{
						Name:  "collection-type",
						Usage: "Collection type (postman, bruno, insomnia) - required with --collection-file",
					},
					&cli.StringFlag{
						Name:  "dir",
						Usage: "Output directory for the generated Locust files",
					},
					&cli.BoolFlag{
						Name:  "headless",
						Usage: "Run Locust in headless mode (without UI)",
						Value: false,
					},
					&cli.BoolFlag{
						Name:  "distributed",
						Usage: "Run Locust in distributed mode",
						Value: false,
					},
				},
				Action: func(c *cli.Context) error {
					return locustCommand(c)
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

func locustCommand(c *cli.Context) error {
	collectionType := c.String("collection-type")
	collectionFile := c.String("collection-file")
	outDir := c.String("dir")
	headless := c.Bool("headless")
	distributed := c.Bool("distributed")

	options := &client.Options{
		CollectionType:             collectionType,
		CollectionPath:             collectionFile,
		OutDir:                     outDir,
		Headless:                   &headless,
		GenerateDistributedHelpers: &distributed,
	}

	return client.GenerateLoadtestBundle(*options)
}

// deployCommand handles infrastructure deployment
func deployCommand(c *cli.Context) error {
	profile := c.String("profile")
	projectName := c.String("project")

	fmt.Println("\nChecking Infrastructure Prerequisites")
	fmt.Println(strings.Repeat("=", 80))

	manager := cloud.NewCloudManager(profile)
	// Step 1: Validate cloud provider credentials
	if err := manager.AutoDetectProvider(profile); err != nil {
		return err
	}
	deployer := repl.NewDeployment(projectName, profile, manager.Provider)
	return deployer.DeployInfrastructureWithTerraform(c.Bool("skip-confirmation"))
}

// parseCommaSeparated splits a comma-separated string into a slice, trimming whitespace
func parseCommaSeparated(input string) []string {
	if input == "" {
		return nil
	}

	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
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
			Message: fmt.Sprintf("Are you sure? Project '%s' will be destroyed. This action cannot be undone.", projectName),
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

	manager := cloud.NewCloudManager(profile)
	// Step 1: Validate cloud provider credentials
	if err := manager.AutoDetectProvider(profile); err != nil {
		return err
	}

	// Create Terraform manager
	destroyer, err := terraform.NewManager(projectName, profile, manager.Provider)
	if err != nil {
		return fmt.Errorf("failed to create terraform manager: %w", err)
	}

	// Destroy infrastructure
	fmt.Println("\nDestroying infrastructure...")
	err = destroyer.Destroy()

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

	manager := cloud.NewCloudManager(profile)
	// Step 1: Validate cloud provider credentials
	if err := manager.AutoDetectProvider(profile); err != nil {
		return err
	}

	exists, _ := manager.Provider.ProjectExists(context.Background(), projectName)
	if !exists {
		fmt.Printf("No project found with name: %s\n", projectName)
		fmt.Println("Run 'automock init' to create a new project.")
		return nil
	}
	metadata, err := manager.Provider.GetDeploymentMetadata()
	if err != nil {
		fmt.Printf("%q\n", err)
		fmt.Println("No infrastructure found for this project.")
		fmt.Println("Run 'automock deploy --project " + projectName + "' to create it if expectations exist. Otherwise, run 'automock init' first.")
		return nil
	}

	fmt.Println("\nInfrastructure is deployed with the following details:")

	// Compute uptime and add deployment info
	deployedAt := metadata.DeployedAt.UTC()
	deployedLocal := deployedAt.Local()
	uptime := time.Since(deployedAt).Hours()

	fmt.Printf("🕓 Deployed At (Local): %s\n", deployedLocal.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("⏱️  Uptime: %.2f hours\n", uptime)
	fmt.Println()

	// Clear details for simple view
	if !detailed {
		metadata.Details = models.InfrastructureOutputs{}
		fmt.Println("Summary Status:")
	} else {
		fmt.Println("Detailed Status:")
	}

	// Pretty-print JSON
	jsonBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		fmt.Printf("❌ Failed to marshal metadata: %v\n", err)
		return err
	}

	fmt.Println(string(jsonBytes))

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

WORKFLOW (Interactive Mode):
  1. Run 'automock init'
  2. Select or create project
  3. Generate expectations (AI, collection import, or interactive builder)
  4. Choose generated expectations option:
     - Save to Storage only
     - Deploy complete infrastructure (ECS + ALB)
     - Start local MockServer
  5. Infrastructure deploys automatically

SUPPORTED LLM PROVIDERS:
  • anthropic    - Claude (requires ANTHROPIC_API_KEY)
  • openai       - GPT-4 (requires OPENAI_API_KEY)

SUPPORTED COLLECTION FORMATS:
  • postman      - Postman Collection v2.1 (.json)
  • bruno        - Bruno Collection (.json)
  • insomnia     - Insomnia Workspace (.json)

INFRASTRUCTURE:
  • Complete     - ECS Fargate + ALB + Auto-scaling
  • Auto-scaling - 10-200 tasks based on CPU/Memory/Requests
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
  automock deploy --project user-service

  # Deploy with existing networking resources (restricted VPC permissions)
  automock deploy --project user-service \
    --vpc-id vpc-0abcd1234 \
    --subnet-ids subnet-111,subnet-222,subnet-333 \
    --security-group-ids sg-abc123

  # Deploy with existing IAM roles (restricted IAM permissions)
  automock deploy --project user-service \
    --execution-role-arn arn:aws:iam::123456789:role/ECSTaskExecutionRole \
    --task-role-arn arn:aws:iam::123456789:role/MyAppTaskRole

  # Check what's running
  automock status --project user-service

  # Clean up
  automock destroy --project user-service

For more information: See INFRASTRUCTURE.md and CLI_INTEGRATION.md
`

	fmt.Print(help)
	return nil
}
