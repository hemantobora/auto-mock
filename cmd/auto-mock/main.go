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

	fmt.Printf("\n🛰️  Checking infrastructure status for: %s\n", projectName)
	fmt.Println(strings.Repeat("━", 80))

	manager := cloud.NewCloudManager(profile)

	// Step 1: Validate cloud provider credentials
	if err := manager.AutoDetectProvider(profile); err != nil {
		return err
	}

	exists, _ := manager.Provider.ProjectExists(context.Background(), projectName)
	if !exists {
		fmt.Printf("❌ No project found with name: %s\n", projectName)
		fmt.Println("💡 Run 'automock init' to create a new project.")
		return nil
	}

	metadata, err := manager.Provider.GetDeploymentMetadata()
	if err != nil {
		fmt.Printf("⚠️  %v\n", err)
		fmt.Println("❌ No infrastructure found for this project.")
		fmt.Printf("💡 Run 'automock deploy --project %s' to create it if expectations exist.\n", projectName)
		fmt.Println("💡 Otherwise, run 'automock init' first.")
		return nil
	}

	fmt.Println("\n✅ Infrastructure is deployed with the following details:")

	deployedAt := metadata.DeployedAt.UTC()
	deployedLocal := deployedAt.Local()
	uptime := time.Since(deployedAt).Hours()

	fmt.Printf("🕓 Deployed At (Local): %s\n", deployedLocal.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("⏱️  Uptime: %.2f hours\n", uptime)
	fmt.Println()

	if !detailed {
		fmt.Println("📊 Summary Status:")
		metadata.Details = nil // hide the nested infra outputs
	} else {
		fmt.Println("🧾 Detailed Status:")
	}

	jsonBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		fmt.Printf("❌ Failed to marshal metadata: %v\n", err)
		return err
	}

	fmt.Println(string(jsonBytes))
	return nil
}

func showDetailedHelp(c *cli.Context) error {
	const (
		h1    = "\033[36m" // cyan
		h2    = "\033[33m" // yellow
		reset = "\033[0m"
	)

	version := c.App.Version
	if version == "" {
		version = "beta"
	}

	help := fmt.Sprintf(`
%sAutoMock — Mock API generator & infra helper%s
Version: %s

%sGLOBAL FLAG%s
  --profile <name>                 Credential profile name (e.g., dev, prod)

%sCOMMANDS%s
  init                             Initialize a project and initiate expectation generation
  deploy                           Deploy infrastructure for an existing project expectations
  destroy                          Destroy infrastructure for a project
  status                           Show infrastructure status for a project
  locust                           Generate a Locust load-testing bundle from an API collection
  help                             Show this help

%sinit — flags%s
  --project <name>                 Project name (bypasses interactive project selection)
  --provider <anthropic|openai>
                                   LLM provider; bypasses provider selection
  --collection-file <path>         Path to API collection (Postman/Bruno/Insomnia)
  --collection-type <postman|bruno|insomnia>
                                   Required when using --collection-file

Examples:
  automock init
  automock init --project user-service
  automock init --provider anthropic
  automock init --collection-file api.postman.json --collection-type postman

%sdeploy — flags%s
  --project <name>                 (required)
  --skip-confirmation              Skip deployment confirmation prompt

Examples:
  automock deploy --project user-service
  automock deploy --project user-service --skip-confirmation

%sdestroy — flags%s
  --project <name>                 (required)
  --force                          Skip confirmation prompts

Examples:
  automock destroy --project user-service
  automock destroy --project user-service --force

%sstatus — flags%s
  --project <name>                 (required)
  --detailed                       Show detailed information including metrics

Examples:
  automock status --project user-service
  automock status --project user-service --detailed

%slocust — flags%s
  --collection-file <path>         Path to API collection (Postman/Bruno/Insomnia)
  --collection-type <postman|bruno|insomnia>
                                   Required when using --collection-file
  --dir <path>                     Output directory for generated Locust files
  --headless                       Run Locust in headless mode (no UI)
  --distributed                    Generate/run in distributed mode

Examples:
  automock locust --collection-file api.postman.json --collection-type postman --dir ./load
  automock locust --collection-file api.postman.json --collection-type postman --headless

ENVIRONMENT
  AWS_PROFILE                      AWS profile to use (alternative to --profile)
  ANTHROPIC_API_KEY                Used when --provider anthropic
  ANTHROPIC_MODEL                  Used to select Anthropic model (default: claude-sonnet-4-5)
  OPENAI_API_KEY                   Used when --provider openai
  OPENAI_MODEL                     Used to select OpenAI model (default: gpt-5-mini)
`, h1, reset, version, h2, reset, h2, reset, h2, reset, h2, reset, h2, reset, h2, reset, h2, reset)

	fmt.Print(help)
	return nil
}
