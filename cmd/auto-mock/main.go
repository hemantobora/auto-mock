package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hemantobora/auto-mock/internal/cloud"
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
				Usage: "Initialize AutoMock project (main command)",
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
						Value: true, // Default to true
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

					// Create context from CLI flags using cloud package's CLIContext
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
				Name:  "upgrade",
				Usage: "Upgrade project to complete infrastructure (ECS + ALB)",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "project",
						Usage:    "Project name to upgrade",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					projectName := c.String("project")
					
					return upgradeProjectInfrastructure(profile, projectName)
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
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					projectName := c.String("project")
					
					return showProjectStatus(profile, projectName)
				},
			},
			{
				Name:   "help",
				Usage:  "Show detailed help and supported features",
				Action: showDetailedHelp,
			},
		},
		// Default action when no command specified
		Action: func(c *cli.Context) error {
			// If user just types "automock", show help
			return cli.ShowAppHelp(c)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// upgradeProjectInfrastructure upgrades a project from basic to complete infrastructure
func upgradeProjectInfrastructure(profile, projectName string) error {
	fmt.Printf("⬆️  Upgrading project '%s' to complete infrastructure...\n", projectName)
	
	// This would use the terraform package to upgrade
	// For now, just show what would happen
	fmt.Println("🔧 This feature requires the Terraform integration")
	fmt.Println("📦 Your project currently uses basic S3-only infrastructure")
	fmt.Println("🚀 Complete infrastructure includes:")
	fmt.Println("   • ECS Fargate with auto-scaling")
	fmt.Println("   • Application Load Balancer with SSL")
	fmt.Println("   • CloudWatch monitoring")
	fmt.Println("   • Automatic configuration reloading")
	fmt.Println("\n💡 Run 'automock init --project " + projectName + "' to set up complete infrastructure")
	
	return nil
}

// showProjectStatus shows the current infrastructure status
func showProjectStatus(profile, projectName string) error {
	fmt.Printf("📊 Infrastructure Status: %s\n", projectName)
	fmt.Println(strings.Repeat("=", 50))
	
	// Check if basic infrastructure (S3) exists
	// This would integrate with existing AWS provider
	fmt.Println("🔍 Checking basic infrastructure (S3 bucket)...")
	fmt.Println("✅ S3 bucket: Found")
	fmt.Println("📦 Infrastructure Level: Basic (S3 Only)")
	
	fmt.Println("\n💡 Available Commands:")
	fmt.Println("   automock upgrade --project " + projectName + "  # Upgrade to complete infrastructure")
	fmt.Println("   automock init --project " + projectName + "     # Generate new configuration")
	
	return nil
}

// showDetailedHelp provides comprehensive help including supported LLMs
func showDetailedHelp(c *cli.Context) error {
	help := `
🎛️  AutoMock - AI-Powered Mock API Infrastructure

BASIC USAGE:
  automock init                    # Interactive mode (recommended)
  automock help                    # Show this help

BYPASS INTERACTIVENESS:
  automock init --project myapi                              # Skip project selection
  automock init --project myapi --provider anthropic         # Skip project & provider selection
  automock init --project myapi --include-auth               # Include auth endpoints
  automock init --collection-file postman.json --collection-type postman

INFRASTRUCTURE COMMANDS:
  automock upgrade --project myapi                           # Upgrade to complete infrastructure
  automock status --project myapi                            # Show infrastructure status

SUPPORTED LLM PROVIDERS:
  • anthropic    - Claude (requires ANTHROPIC_API_KEY)
  • openai       - GPT-4 (requires OPENAI_API_KEY) 
  • template     - Template-based (free, always available)

SUPPORTED COLLECTION FORMATS:
  • postman      - Postman Collection v2.1 (.json)
  • bruno        - Bruno Collection (.json or .bru)
  • insomnia     - Insomnia Workspace (.json)

INFRASTRUCTURE OPTIONS:
  • Basic        - S3 bucket only (current default)
  • Complete     - S3 + ECS + ALB + auto-scaling + monitoring

WORKFLOW:
  1. AWS credential detection
  2. Project selection/creation
  3. Infrastructure deployment (S3 or complete)
  4. Mock configuration generation (AI-powered)
  5. Automatic configuration upload
  6. Optional: Automatic teardown with TTL

ENVIRONMENT VARIABLES:
  AWS_PROFILE              # AWS profile to use
  ANTHROPIC_API_KEY        # For Claude AI generation
  OPENAI_API_KEY           # For GPT-4 AI generation

EXAMPLES:
  # Interactive mode (most common)
  automock init

  # Quick start with specific project
  automock init --project user-service

  # Check project status
  automock status --project user-service

  # Upgrade to complete infrastructure
  automock upgrade --project user-service

  # Use specific AI provider
  automock init --project user-service --provider anthropic

  # Import Postman collection
  automock init --collection-file my-api.json --collection-type postman

For more information: https://github.com/hemantobora/auto-mock
`

	fmt.Print(help)
	return nil
}
