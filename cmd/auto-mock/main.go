package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/client"
	"github.com/hemantobora/auto-mock/internal/cloud"
	"github.com/hemantobora/auto-mock/internal/commands"
	"github.com/hemantobora/auto-mock/internal/models"
	"github.com/hemantobora/auto-mock/internal/repl"
	"github.com/hemantobora/auto-mock/internal/terraform"
	"github.com/urfave/cli/v2"
)

// version is set via -ldflags "-X main.version=<version>" during build
var version = "0.0.1-alpha"

func main() {
	app := &cli.App{
		Name:    "automock",
		Usage:   "Generate and deploy mock API infrastructure",
		Version: version,
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
				Usage: "Generate and optionally upload a Locust load testing bundle",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "project", Usage: "Project name."},
					&cli.BoolFlag{Name: "upload", Usage: "Upload bundle to cloud storage."},
					&cli.BoolFlag{Name: "dry-run", Usage: "Simulate upload without persisting objects."},
					&cli.BoolFlag{Name: "edit", Usage: "Download current active bundle for editing (interactive re-upload option)."},
					&cli.BoolFlag{Name: "delete-pointer", Usage: "Delete current bundle pointer (current.json) only; keep versions/bundles."},
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
	project := c.String("project")
	collectionType := c.String("collection-type")
	collectionFile := c.String("collection-file")
	outDir := c.String("dir")
	headless := c.Bool("headless")
	distributed := c.Bool("distributed")
	upload := c.Bool("upload")
	edit := c.Bool("edit")
	deletePtr := c.Bool("delete-pointer")

	options := &client.Options{
		CollectionType:             collectionType,
		CollectionPath:             collectionFile,
		OutDir:                     outDir,
		Headless:                   &headless,
		GenerateDistributedHelpers: &distributed,
	}

	profile := c.String("profile")
	return commands.RunLocust(profile, project, *options, upload, edit, deletePtr)
}

// deployCommand handles infrastructure deployment
func deployCommand(c *cli.Context) error {
	profile := c.String("profile")
	projectName := c.String("project")

	fmt.Println("\nChecking Infrastructure Prerequisites")
	fmt.Println(strings.Repeat("=", 80))

	manager := cloud.NewCloudManager(profile)
	if err := manager.AutoDetectProvider(profile); err != nil {
		return err
	}
	ctx := context.Background()
	// 1. Check project existence
	exists, _ := manager.Provider.ProjectExists(ctx, projectName)
	if !exists {
		fmt.Printf("❌ Project '%s' does not exist. Run 'automock init' (for mocks) or 'automock locust' (for load tests) first.\n", projectName)
		return nil
	}

	// 2. Detect pointers/config presence
	mockConfig, mockErr := manager.Provider.GetConfig(ctx, projectName)
	var hasMock bool = mockErr == nil && mockConfig != nil
	loadPtr, loadPtrErr := manager.Provider.GetLoadTestPointer(ctx, projectName)
	var hasLoad bool = loadPtrErr == nil && loadPtr != nil && loadPtr.ActiveVersion != ""

	if !hasMock && !hasLoad {
		fmt.Println("ℹ️  No mock configuration or load test bundle found.")
		fmt.Println("👉 Generate mocks: 'automock init' or upload a load test bundle: 'automock locust'.")
		return nil
	}

	// 3. Fetch deployment metadata
	mockMeta, _ := manager.Provider.GetDeploymentMetadata()
	loadMeta, _ := manager.Provider.GetLoadTestDeploymentMetadata()
	mockDeployed := mockMeta != nil && mockMeta.DeploymentStatus == "deployed"
	loadDeployed := loadMeta != nil && loadMeta.DeploymentStatus == "deployed"

	// Helper lambdas
	deployMocks := func() error {
		deployer := repl.NewDeployment(projectName, profile, manager.Provider)
		return deployer.DeployInfrastructureWithTerraform(c.Bool("skip-confirmation"))
	}
	deployLoad := func() error {
		fmt.Println("🚀 Deploying Locust infrastructure...")
		opts := &models.LoadTestDeploymentOptions{WorkerDesiredCount: 0}

		// Collect BYO networking like mockserver (VPC/Subnets)
		useBYO := false
		_ = survey.AskOne(&survey.Confirm{Message: "Bring your own networking and IAM (BYO)?", Default: false}, &useBYO)
		if useBYO {
			opts.UseExistingVPC = true
			opts.UseExistingSubnets = true
			var vpcID string
			_ = survey.AskOne(&survey.Input{Message: "Network ID (e.g., AWS VPC ID vpc-xxxx):"}, &vpcID)
			vpcID = strings.TrimSpace(vpcID)
			if vpcID == "" {
				return fmt.Errorf("VPC ID is required when using BYO networking")
			}
			opts.VpcID = vpcID
			var subnetsCSV string
			_ = survey.AskOne(&survey.Input{Message: "Public subnet IDs (comma-separated):", Help: "e.g., AWS: subnet-aaaa,subnet-bbbb"}, &subnetsCSV)
			subnetsCSV = strings.TrimSpace(subnetsCSV)
			if subnetsCSV == "" {
				return fmt.Errorf("At least one subnet ID is required when using BYO networking")
			}
			parts := strings.Split(subnetsCSV, ",")
			var subs []string
			for _, p := range parts {
				pp := strings.TrimSpace(p)
				if pp != "" {
					subs = append(subs, pp)
				}
			}
			if len(subs) == 0 {
				return fmt.Errorf("No valid subnet IDs provided")
			}
			opts.PublicSubnetIDs = subs

			// Optionally BYO IAM roles
			useIAM := false
			_ = survey.AskOne(&survey.Confirm{Message: "Use existing IAM roles for ECS (execution & task)?", Default: false}, &useIAM)
			if useIAM {
				opts.UseExistingIAMRoles = true
				var execArn, taskArn string
				_ = survey.AskOne(&survey.Input{Message: "Execution role (ARN on AWS):"}, &execArn)
				_ = survey.AskOne(&survey.Input{Message: "Task role (ARN on AWS; press Enter to reuse execution role):"}, &taskArn)
				execArn = strings.TrimSpace(execArn)
				taskArn = strings.TrimSpace(taskArn)
				if execArn == "" {
					return fmt.Errorf("Execution Role ARN is required when using existing IAM roles")
				}
				if taskArn == "" {
					taskArn = execArn
				}
				opts.ExecutionRoleARN = execArn
				opts.TaskRoleARN = taskArn
			}

			// Optionally BYO Security Groups
			useSG := false
			_ = survey.AskOne(&survey.Confirm{Message: "Use existing Security Groups (ALB & ECS)?", Default: false}, &useSG)
			if useSG {
				opts.UseExistingSecurityGroups = true
				var albSG, ecsSG string
				_ = survey.AskOne(&survey.Input{Message: "ALB Security Group ID:"}, &albSG)
				_ = survey.AskOne(&survey.Input{Message: "ECS Tasks Security Group ID:"}, &ecsSG)
				albSG = strings.TrimSpace(albSG)
				ecsSG = strings.TrimSpace(ecsSG)
				if albSG == "" || ecsSG == "" {
					return fmt.Errorf("Both ALB and ECS security group IDs are required when using existing security groups")
				}
				opts.ALBSecurityGroupID = albSG
				opts.ECSSecurityGroupID = ecsSG
			}
		}

		// Prompt for worker count
		var workerStr string
		_ = survey.AskOne(&survey.Input{Message: "Desired worker count (0 for none):", Default: "0"}, &workerStr)
		workerStr = strings.TrimSpace(workerStr)
		if workerStr != "" {
			if n, err := strconv.Atoi(workerStr); err == nil && n >= 0 {
				opts.WorkerDesiredCount = n
			} else {
				return fmt.Errorf("invalid worker count: %s", workerStr)
			}
		}
		mgr, err := terraform.NewLoadTestManager(projectName, profile, manager.Provider)
		if err != nil {
			return err
		}
		out, err := mgr.Deploy(opts)
		if err != nil {
			return err
		}
		fmt.Printf("✅ Locust deployed: ALB=%s MasterFQDN=%s Workers=%d\n", out.ALBDNSName, out.CloudMapMasterFQDN, out.WorkerDesiredCount)
		return nil
	}
	scaleWorkers := func() error {
		if !loadDeployed {
			fmt.Println("⚠️  Load test infra not deployed; cannot scale.")
			return nil
		}
		mgr, err := terraform.NewLoadTestManager(projectName, profile, manager.Provider)
		if err != nil {
			return err
		}
		var desiredStr string
		_ = survey.AskOne(&survey.Input{Message: "Enter desired worker count:"}, &desiredStr)
		desiredStr = strings.TrimSpace(desiredStr)
		if desiredStr == "" {
			return fmt.Errorf("no worker count provided")
		}
		n, convErr := strconv.Atoi(desiredStr)
		if convErr != nil {
			return fmt.Errorf("invalid worker count: %s", desiredStr)
		}
		if err := mgr.ScaleWorkers(n); err != nil {
			return err
		}
		fmt.Println("✅ Scaled workers to:", n)
		return nil
	}

	// 4. Decision matrix
	// Case: both pointers present
	if hasMock && hasLoad {
		switch {
		case mockDeployed && loadDeployed:
			// Offer scale workers or exit
			choice := ""
			_ = survey.AskOne(&survey.Select{Message: "Both mock & loadtest deployed. Action:", Options: []string{"scale-workers", "redeploy-mocks", "redeploy-loadtest", "exit"}, Default: "scale-workers"}, &choice)
			if choice == "scale-workers" {
				return scaleWorkers()
			}
			if choice == "redeploy-mocks" {
				return deployMocks()
			}
			if choice == "redeploy-loadtest" {
				return deployLoad()
			}
			fmt.Println("✅ Nothing done.")
			return nil
		case loadDeployed && !mockDeployed:
			choice := ""
			_ = survey.AskOne(&survey.Select{Message: "Loadtest deployed; mocks not deployed. Action:", Options: []string{"deploy-mocks", "scale-workers", "exit"}, Default: "deploy-mocks"}, &choice)
			if choice == "deploy-mocks" {
				return deployMocks()
			}
			if choice == "scale-workers" {
				return scaleWorkers()
			}
			fmt.Println("✅ Nothing done.")
			return nil
		case mockDeployed && !loadDeployed:
			var proceed bool
			_ = survey.AskOne(&survey.Confirm{Message: "Mock infra deployed; deploy loadtest now?", Default: true}, &proceed)
			if proceed {
				return deployLoad()
			}
			fmt.Println("✅ Skipped loadtest deployment.")
			return nil
		default: // neither deployed but both bundles/config exist
			choice := ""
			_ = survey.AskOne(&survey.Select{Message: "Mocks & loadtest artifacts found. Deploy:", Options: []string{"both", "only-mocks", "only-loadtest", "exit"}, Default: "both"}, &choice)
			if choice == "both" {
				if err := deployMocks(); err != nil {
					return err
				}
				return deployLoad()
			}
			if choice == "only-mocks" {
				return deployMocks()
			}
			if choice == "only-loadtest" {
				return deployLoad()
			}
			fmt.Println("✅ Nothing done.")
			return nil
		}
	}

	// Case: only mocks
	if hasMock && !hasLoad {
		if mockDeployed {
			choice := ""
			_ = survey.AskOne(&survey.Select{Message: "Mock infra already deployed. Action:", Options: []string{"redeploy-mocks", "exit"}, Default: "exit"}, &choice)
			if choice == "redeploy-mocks" {
				return deployMocks()
			}
			fmt.Println("✅ Nothing done.")
			return nil
		}
		return deployMocks()
	}

	// Case: only loadtest
	if hasLoad && !hasMock {
		if loadDeployed {
			choice := ""
			_ = survey.AskOne(&survey.Select{Message: "Loadtest infra deployed. Action:", Options: []string{"scale-workers", "redeploy-loadtest", "exit"}, Default: "scale-workers"}, &choice)
			if choice == "scale-workers" {
				return scaleWorkers()
			}
			if choice == "redeploy-loadtest" {
				return deployLoad()
			}
			fmt.Println("✅ Nothing done.")
			return nil
		}
		return deployLoad()
	}

	fmt.Println("⚠️  Unexpected state; nothing done.")
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
	if err := manager.AutoDetectProvider(profile); err != nil {
		return err
	}
	ctx := context.Background()

	// Discover presence
	hasMock := false
	if _, e := manager.Provider.GetConfig(ctx, projectName); e == nil {
		hasMock = true
	}
	hasLoad := false
	if p, e := manager.Provider.GetLoadTestPointer(ctx, projectName); e == nil && p != nil && p.ActiveVersion != "" {
		hasLoad = true
	}

	if !hasMock && !hasLoad {
		fmt.Println("ℹ️  Nothing to destroy: no mock config or loadtest bundle found.")
		return nil
	}

	// Ask what to destroy
	choice := ""
	options := []string{}
	if hasMock {
		options = append(options, "mocks")
	}
	if hasLoad {
		options = append(options, "loadtest")
	}
	if hasMock && hasLoad {
		options = append(options, "both")
	}
	options = append(options, "exit")
	_ = survey.AskOne(&survey.Select{Message: "Select what to destroy:", Options: options, Default: options[0]}, &choice)
	if choice == "exit" {
		fmt.Println("✅ Skipped destroy")
		return nil
	}

	// Destroy mocks
	if choice == "mocks" || choice == "both" {
		destroyer, err := terraform.NewManager(projectName, profile, manager.Provider)
		if err != nil {
			return fmt.Errorf("failed to create terraform manager: %w", err)
		}
		fmt.Println("\nDestroying mock infrastructure...")
		if err := destroyer.Destroy(); err != nil {
			return err
		}
		_ = manager.Provider.DeleteDeploymentMetadata()
		fmt.Println("✅ Mock infra destroyed")
		if choice != "both" {
			return nil
		}
	}

	// Destroy loadtest
	if choice == "loadtest" || choice == "both" {
		lt, err := terraform.NewLoadTestManager(projectName, profile, manager.Provider)
		if err != nil {
			return err
		}
		fmt.Println("\nDestroying load test infrastructure...")
		if err := lt.Destroy(); err != nil {
			return err
		}
		_ = manager.Provider.DeleteLoadTestDeploymentMetadata()
		fmt.Println("✅ Load test infra destroyed")
	}
	return nil
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

%sUSAGE%s
	automock [global flags] <command> [command flags]
	Note: Global flags must come before the command (urfa ve/cli v2)
	      e.g. automock --profile sandbox deploy --project myproj

%sGLOBAL FLAG%s
  --profile <name>                 Credential profile name (e.g., dev, prod)
				   You can also set AWS_PROFILE instead of --profile

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
	automock --profile sandbox init
	automock --profile sandbox init --project user-service
	automock --profile sandbox init --provider anthropic
	automock --profile sandbox init --collection-file api.postman.json --collection-type postman
	AWS_PROFILE=sandbox automock init

%sdeploy — flags%s
  --project <name>                 (required)
  --skip-confirmation              Skip deployment confirmation prompt

Examples:
	automock --profile sandbox deploy --project user-service
	automock --profile sandbox deploy --project user-service --skip-confirmation
	AWS_PROFILE=sandbox automock deploy --project user-service

%sdestroy — flags%s
  --project <name>                 (required)
  --force                          Skip confirmation prompts

Examples:
	automock --profile sandbox destroy --project user-service
	automock --profile sandbox destroy --project user-service --force

%sstatus — flags%s
  --project <name>                 (required)
  --detailed                       Show detailed information including metrics

Examples:
	automock --profile sandbox status --project user-service
	automock --profile sandbox status --project user-service --detailed

%slocust — flags%s
	--collection-file <path>         Path to API collection (Postman/Bruno/Insomnia)
	--collection-type <postman|bruno|insomnia>
																	 Required when using --collection-file
	--dir <path>                     Output directory for generated Locust files
	--headless                       Run Locust in headless mode (no UI)
	--distributed                    Generate/run in distributed mode
	--upload                         Upload generated bundle to cloud storage
	--edit                           Download active bundle for local edits (interactive re-upload)
	--delete-pointer                 Delete current bundle pointer only (keep versions)

Examples:
  automock locust --collection-file api.postman.json --collection-type postman --dir ./load
  automock locust --collection-file api.postman.json --collection-type postman --headless
	automock locust --project user-service --upload --dir ./load
	automock locust --project user-service --edit
	automock locust --project user-service --delete-pointer

ENVIRONMENT
  AWS_PROFILE                      AWS profile to use (alternative to --profile)
  ANTHROPIC_API_KEY                Used when --provider anthropic
  ANTHROPIC_MODEL                  Used to select Anthropic model (default: claude-sonnet-4-5)
  OPENAI_API_KEY                   Used when --provider openai
  OPENAI_MODEL                     Used to select OpenAI model (default: gpt-5-mini)
`, h1, reset, version, h2, reset, h2, reset, h2, reset, h2, reset, h2, reset, h2, reset, h2, reset, h2, reset)

	fmt.Print(help)
	return nil
}
