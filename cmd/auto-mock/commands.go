package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/client"
	"github.com/hemantobora/auto-mock/internal/cloud"
	"github.com/hemantobora/auto-mock/internal/commands"
	"github.com/hemantobora/auto-mock/internal/models"
	"github.com/hemantobora/auto-mock/internal/prompts"
	"github.com/hemantobora/auto-mock/internal/repl"
	"github.com/hemantobora/auto-mock/internal/terraform"
	"github.com/urfave/cli/v2"
)

// humanUptimeSince returns a compact human-readable duration like:
//
//	"45s", "12m", "1h 5m", "2d 3h"
func humanUptimeSince(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := (int(d.Minutes()) % 60)
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	if h == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd %dh", days, h)
}

// locustCommand handles load-test bundle generation and management
func locustCommand(c *cli.Context) error {
	// Guard against common mistake: using subcommand-like words instead of flags
	if c.Args().Len() > 0 {
		for _, a := range c.Args().Slice() {
			switch a {
			case "upload", "download", "delete-pointer", "purge-all":
				return fmt.Errorf("'%s' is a flag; use --%s (e.g., automock load --%s)", a, a, a)
			}
		}
	}
	project := c.String("project")
	collectionType := c.String("collection-type")
	collectionFile := c.String("collection-file")
	outDir := c.String("dir")
	headless := c.Bool("headless")
	distributed := c.Bool("distributed")
	upload := c.Bool("upload")
	download := c.Bool("download")
	deletePtr := c.Bool("delete-pointer")
	purgeAll := c.Bool("purge-all")

	// Validate mutually exclusive flags
	exclusive := func(args ...string) string {
		for _, arg := range args {
			if c.Bool(arg) {
				return arg
			}
		}
		return ""
	}

	if upload {
		conflict := exclusive("download", "delete-pointer", "headless", "distributed", "purge-all")
		if conflict != "" {
			return fmt.Errorf("flag --upload is mutually exclusive with --%s", conflict)
		}
		if outDir == "" {
			return fmt.Errorf("flag --dir is required with upload")
		}
	}
	if download {
		conflict := exclusive("upload", "delete-pointer", "headless", "distributed", "purge-all")
		if conflict != "" {
			return fmt.Errorf("flag --download is mutually exclusive with --%s", conflict)
		}
		// --dir optional for download; if not provided, backend creates a default directory
	}
	if deletePtr {
		conflict := exclusive("upload", "download", "headless", "distributed", "purge-all")
		if conflict != "" {
			return fmt.Errorf("flag --delete-pointer is mutually exclusive with --%s", conflict)
		}
	}
	if purgeAll {
		conflict := exclusive("upload", "download", "delete-pointer", "headless", "distributed")
		if conflict != "" {
			return fmt.Errorf("flag --purge-all is mutually exclusive with --%s", conflict)
		}
	}

	options := &client.Options{
		CollectionType:             collectionType,
		CollectionPath:             collectionFile,
		OutDir:                     outDir,
		Headless:                   &headless,
		GenerateDistributedHelpers: &distributed,
	}

	profile := c.String("profile")
	return commands.RunLocust(profile, project, *options, upload, download, deletePtr, purgeAll)
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
		fmt.Printf("❌ Project '%s' does not exist. Run 'automock init' (for mocks) or 'automock load' (for load tests) first.\n", projectName)
		return nil
	}

	// 2. Detect pointers/config presence
	mockConfig, mockErr := manager.Provider.GetConfig(ctx, projectName)
	var hasMock bool = mockErr == nil && mockConfig != nil
	loadPtr, loadPtrErr := manager.Provider.GetLoadTestPointer(ctx, projectName)
	var hasLoad bool = loadPtrErr == nil && loadPtr != nil && loadPtr.ActiveVersion != ""

	if !hasMock && !hasLoad {
		fmt.Println("ℹ️  No mock configuration or load test bundle found.")
		fmt.Println("👉 Generate mocks: 'automock init' or upload a load test bundle: 'automock load'.")
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
		fmt.Println("🚀 Deploying load-test infrastructure...")
		opts := &models.LoadTestDeploymentOptions{WorkerDesiredCount: 0}

		// Collect BYO options using shared prompts package
		if err := prompts.PromptAllBYOOptions(opts); err != nil {
			return err
		}

		mgr, err := terraform.NewLoadTestManager(projectName, profile, manager.Provider)
		if err != nil {
			return err
		}
		out, err := mgr.Deploy(opts)
		if err != nil {
			return err
		}
		fmt.Printf(`✅ Load-test infra deployed: 
ALB TLS=https://%s 
ALB Open=http://%s 
Workers=%d`, out.ALBDNSName, out.ALBDNSName, out.WorkerDesiredCount)
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
		_ = survey.AskOne(&survey.Input{
			Message: "Enter desired worker count (-1 to stop the update):",
			Help:    "Provide the worker count",
			Default: "0",
		}, &desiredStr)
		desiredStr = strings.TrimSpace(desiredStr)
		if desiredStr == "" {
			return fmt.Errorf("no worker count provided")
		}
		n, convErr := strconv.Atoi(desiredStr)
		if convErr != nil {
			return fmt.Errorf("invalid worker count: %s", desiredStr)
		}
		if n < 0 {
			fmt.Println("✅ Scale update cancelled.")
			return nil
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
			return scaleWorkers()
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
			return deployLoad()
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
			fmt.Println("✅ Mock infra already deployed.")
			return nil
		}
		return deployMocks()
	}

	// Case: only loadtest
	if hasLoad && !hasMock {
		if loadDeployed {
			return scaleWorkers()
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

	exists, err := manager.Provider.ProjectExists(ctx, projectName)
	if !exists || err != nil {
		return fmt.Errorf("project %s does not exist", projectName)
	}

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
	if hasMock && hasLoad {
		options = append(options, "mocks", "loadtest", "both")
	} else if hasMock {
		choice = "mocks"
	} else if hasLoad {
		choice = "loadtest"
	}
	if choice == "" {
		_ = survey.AskOne(&survey.Select{Message: "Select what to destroy:", Options: options, Default: options[0]}, &choice)
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

	// 3. Fetch deployment metadata
	mockMeta, _ := manager.Provider.GetDeploymentMetadata()
	loadMeta, _ := manager.Provider.GetLoadTestDeploymentMetadata()
	mockDeployed := mockMeta != nil && mockMeta.DeploymentStatus == "deployed"
	loadDeployed := loadMeta != nil && loadMeta.DeploymentStatus == "deployed"

	if !mockDeployed && !loadDeployed {
		fmt.Println("❌ No infrastructure found for this project.")
		fmt.Printf("💡 Run 'automock deploy --project %s' to create it if expectations exists or load test scripts uploaded.\n", projectName)
		fmt.Println("💡 Otherwise, run 'automock init' or 'automock load' first.")
		return nil
	}

	if mockDeployed {
		fmt.Println("✅ Mock infrastructure is deployed.")
		fmt.Println("\n✅ Mock Infrastructure is deployed with the following details:")

		deployedAt := mockMeta.DeployedAt.UTC()
		deployedLocal := deployedAt.Local()
		uptimeStr := humanUptimeSince(deployedAt)

		fmt.Printf("🕓 Deployed At (Local): %s\n", deployedLocal.Format("2006-01-02 15:04:05 MST"))
		fmt.Printf("⏱️  Uptime: %s\n", uptimeStr)
		fmt.Println()

		if !detailed {
			fmt.Println("📊 Summary Status:")
			mockMeta.Details = nil // hide the nested infra outputs
		} else {
			fmt.Println("🧾 Detailed Status:")
		}

		jsonBytes, err := json.MarshalIndent(mockMeta, "", "  ")
		if err != nil {
			fmt.Printf("❌ Failed to marshal metadata: %v\n", err)
			return err
		}

		fmt.Println(string(jsonBytes))
	}
	if loadDeployed {
		fmt.Println("\n✅ Load Test infrastructure is deployed.")

		deployedAt := loadMeta.DeployedAt.UTC()
		deployedLocal := deployedAt.Local()
		uptimeStr := humanUptimeSince(deployedAt)

		fmt.Printf("🕓 Deployed At (Local): %s\n", deployedLocal.Format("2006-01-02 15:04:05 MST"))
		fmt.Printf("⏱️  Uptime: %s\n", uptimeStr)
		fmt.Println()

		if !detailed {
			fmt.Println("📊 Summary Status:")
			loadMeta.Details = nil // hide the nested infra outputs
		} else {
			loadMeta.Details.Extras = nil // hide extra verbose info
			fmt.Println("🧾 Detailed Status:")
		}

		jsonBytes, err := json.MarshalIndent(loadMeta, "", "  ")
		if err != nil {
			fmt.Printf("❌ Failed to marshal metadata: %v\n", err)
			return err
		}

		fmt.Println(string(jsonBytes))
	}
	return nil
}

// showDetailedHelp displays comprehensive CLI help documentation
func showDetailedHelp(c *cli.Context) error {
	const (
		cyan   = "\033[36m"
		yellow = "\033[33m"
		reset  = "\033[0m"
	)
	v := c.App.Version
	if v == "" {
		v = "beta"
	}

	// Concise help: core usage, primary commands, key flags, env vars, minimal examples.
	help := fmt.Sprintf(`
%sAutoMock%s v%s — mock & load-test infra CLI

%sUSAGE%s
	automock [global flags] <command> [flags]
	Example: automock --profile dev deploy --project orders

%sCOMMANDS%s
	init      Generate expectations & bootstrap project
	deploy    Deploy mock and/or load-test infrastructure
	destroy   Tear down infrastructure and metadata
	status    Show deployment status (add --detailed)
	load      Generate / upload / download load-test bundle; manage pointers
	help      Show this help

%sGLOBAL FLAGS%s
	--profile <name>   Cloud credential profile (or AWS_PROFILE env)

%sINIT FLAGS%s
	--project <name>
	--provider <anthropic|openai>
	--collection-file <path> --collection-type <postman|bruno|insomnia>

%sDEPLOY FLAGS%s
	--project <name>  (required)
	--skip-confirmation

%sDESTROY FLAGS%s
	--project <name>  (required)
	--force            Skip confirmations

%sSTATUS FLAGS%s
	--project <name>  (required)
	--detailed

%sLOAD FLAGS%s
	--collection-file <path> --collection-type <type>
	--dir <path>              Output directory
	--headless | --distributed (generation only)
	--upload | --download | --delete-pointer | --purge-all

%sENV VARS%s
	AWS_PROFILE           Alternative to --profile
	ANTHROPIC_API_KEY     Used with provider anthropic
	OPENAI_API_KEY        Used with provider openai

%sQUICK EXAMPLES%s
	automock init --project users --provider anthropic
	automock load --project users --upload --dir ./load
	automock load --project users --download --dir ./work
	automock load --project users --delete-pointer
	automock deploy --project users
	automock status --project users --detailed
	automock destroy --project users --force

Run 'automock <command> --help' for command-specific flags.
`,
		cyan, reset, v,
		yellow, reset,
		yellow, reset,
		yellow, reset,
		yellow, reset,
		yellow, reset,
		yellow, reset,
		yellow, reset,
		yellow, reset,
		yellow, reset,
		yellow, reset,
	)
	fmt.Print(help)
	return nil
}
