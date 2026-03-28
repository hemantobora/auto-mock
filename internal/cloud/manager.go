package cloud

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/expectations"
	"github.com/hemantobora/auto-mock/internal/models"
	"github.com/hemantobora/auto-mock/internal/repl"
	"github.com/hemantobora/auto-mock/internal/terraform"
)

// InitializationMode defines how the tool should operate
type InitializationMode int

const (
	// ModeInteractive - Primary mode: REPL-driven (default)
	ModeInteractive InitializationMode = iota
	// ModeCollection - Secondary mode: CLI-driven collection import
	ModeCollection
)

// CLIContext holds CLI parameters and determines the initialization mode
type CLIContext struct {
	// Core project settings
	ProjectName string `json:"project_name,omitempty"`

	// Collection import settings (triggers ModeCollection)
	CollectionFile string `json:"collection_file,omitempty"`
	CollectionType string `json:"collection_type,omitempty"`

	// Optional CLI overrides (used in both modes)
	Provider string `json:"provider,omitempty"` // LLM provider preference
}

// GetMode determines which initialization mode to use based on CLI context
func (c *CLIContext) GetMode() InitializationMode {
	if c.CollectionFile != "" {
		return ModeCollection
	}
	return ModeInteractive
}

// HasProject returns true if a project name was provided via CLI
func (c *CLIContext) HasProject() bool {
	return c.ProjectName != ""
}

// CloudManager orchestrates the auto-mock initialization workflow
type CloudManager struct {
	profile  string
	Provider internal.Provider
	factory  *Factory
}

// NewCloudManager creates a new cloud manager instance
func NewCloudManager(profile string) *CloudManager {
	return &CloudManager{
		profile: profile,
		factory: NewFactory(),
	}
}

// AutoDetectAndInit is the main entrypoint for the CLI's `init` command.
// It supports both interactive (REPL) and CLI-driven (collection import) workflows.
func AutoDetectAndInit(profile string, cliContext *CLIContext) error {
	manager := NewCloudManager(profile)
	// Step 1: Validate cloud provider credentials
	if err := manager.AutoDetectProvider(profile); err != nil {
		return err
	}
	return manager.Initialize(cliContext)
}

// providerIcon maps a provider type string to a display label with a brand-specific icon.
var providerIcon = map[string]string{
	"aws":   "☁️  AWS",   // AWS — the original cloud
	"azure": "🔷  Azure", // Azure — Microsoft blue
	"gcp":   "🌐  GCP",   // GCP — Google, internet-first
}

func (m *CloudManager) AutoDetectProvider(profile string) error {
	ctx := context.Background()

	// Phase 1: lightweight credential check — no API calls.
	available := m.factory.DetectAvailableProviders(ctx, profile)

	if len(available) == 0 {
		return fmt.Errorf("❌ No valid cloud provider credentials found.\n" +
			"   AWS:   configure credentials or run 'aws configure'\n" +
			"   Azure: run 'az login'\n" +
			"   (GCP support is coming soon)")
	}

	// Phase 2: if multiple providers, let the user pick one.
	chosen := available[0]
	if len(available) > 1 {
		labels := make([]string, len(available))
		for i, pt := range available {
			label, ok := providerIcon[pt]
			if !ok {
				label = pt
			}
			labels[i] = label
		}

		var selected string
		if err := survey.AskOne(&survey.Select{
			Message: "Multiple cloud providers detected. Which one would you like to use?",
			Options: labels,
		}, &selected); err != nil {
			return err
		}

		for i, label := range labels {
			if label == selected {
				chosen = available[i]
				break
			}
		}
	}

	// Phase 3: initialise the chosen provider (makes actual cloud API calls).
	provider, err := m.factory.CreateProvider(ctx, chosen, WithProfile(profile))
	if err != nil {
		return fmt.Errorf("failed to initialise %s provider: %w", chosen, err)
	}
	m.Provider = provider
	return nil
}

// Initialize runs the complete initialization workflow
func (m *CloudManager) Initialize(cliContext *CLIContext) error {
	// Step 1: Resolve project (CLI-driven or interactive)
	actionType, err := m.resolveProject(cliContext)
	if err != nil {
		return err
	}
	if actionType == models.ActionExit {
		return nil
	}

	existingConfig, err := m.getMockConfiguration()
	if err != nil && actionType != models.ActionCreate && actionType != models.ActionGenerate {
		return fmt.Errorf("failed to load expectations: %w", err)
	}
	project := m.getCurrentProject()
	expManager, err := expectations.NewExpectationManager(project)
	if err != nil {
		return fmt.Errorf("failed to create expectation manager: %w", err)
	}
	deployer := repl.NewDeployment(project, m.profile, m.Provider)
	var refreshConfig bool = false
	for {
		switch actionType {
		case models.ActionCreate:
			if _, err := m.createNewProject(""); err != nil {
				return err
			}
			fallthrough
		case models.ActionGenerate:
			// Proceed to generation flow
			fmt.Printf("➕ Generating new expectations for project: %s\n", project)
			if err := m.generateMockConfiguration(cliContext); err != nil {
				return fmt.Errorf("mock expectation generation failed: %w", err)
			}
			return nil
		case models.ActionAdd:
			fmt.Printf("➕ Adding new expectations to project: %s\n", project)
			if err := m.addMockConfiguration(cliContext, existingConfig); err != nil {
				return fmt.Errorf("add failed: %w", err)
			}
			refreshConfig = true
		case models.ActionView:
			fmt.Printf("👁️ Viewing expectations for project: %s\n", project)
			if err := expManager.ViewExpectations(existingConfig); err != nil {
				return fmt.Errorf("view failed: %w", err)
			}
		case models.ActionDownload:
			fmt.Printf("💾 Downloading expectations for project: %s\n", project)
			if err := expManager.DownloadExpectations(existingConfig); err != nil {
				return fmt.Errorf("download failed: %w", err)
			}
		case models.ActionEdit:
			if err := m.handleEditExpectations(expManager, existingConfig); err != nil {
				return fmt.Errorf("edit failed: %w", err)
			}
			refreshConfig = true
		case models.ActionRemove:
			// Manager handles actual removal (data operations)
			if err := m.handleRemoveExpectations(expManager, existingConfig); err != nil {
				return fmt.Errorf("remove failed: %w", err)
			}
			refreshConfig = true
		case models.ActionDelete:
			fmt.Printf("🗑️ Deleting project: %s\n", project)
			if err := expManager.DeleteProjectPrompt(); err != nil {
				return err
			}
			if err := m.destroyInfrastructureAndDeleteProject(); err != nil {
				return fmt.Errorf("failed to destroy infrastructure: %w", err)
			}
			return nil
		case models.ActionReplace:
			fmt.Printf("🔄 Replacing expectations for project: %s\n", project)
			if err := expManager.ReplaceExpectationsPrompt(); err != nil {
				return fmt.Errorf("replace failed: %w", err)
			}
			if err := m.generateMockConfiguration(cliContext); err != nil {
				return fmt.Errorf("failed to generate mock configuration: %w", err)
			}
			refreshConfig = true
		case models.ActionExit:
			fmt.Println("👋 Goodbye!")
			return nil
		case models.ActionDeploy:
			fmt.Printf("🚀 Preparing mock infrastructure deployment for project: %s\n", project)
			deployed, _ := m.Provider.IsDeployed()
			if deployed {
				fmt.Println("✅ Infrastructure is already deployed.")
			} else {
				if err := deployer.DeployInfrastructureWithTerraform(false); err != nil {
					return fmt.Errorf("deployment failed: %w", err)
				}
			}
			return nil
		default:
			return fmt.Errorf("unsupported action type")
		}
		if refreshConfig {
			existingConfig, err = m.getMockConfiguration()
			if err != nil {
				return fmt.Errorf("failed to get latest mock configuration: %w", err)
			}
			refreshConfig = false
		}
		actionType = repl.SelectProjectAction(m.getCurrentProject(), existingConfig)
		fmt.Println()
	}
}

func (m *CloudManager) addMockConfiguration(cliContext *CLIContext, existingConfiguration *models.MockConfiguration) error {
	additionalExpectations, err := m.generateMockExpectations(cliContext)
	if err != nil {
		return fmt.Errorf("failed to generate mock expectations: %w", err)
	}
	additionalConfigurations, err := models.ParseMockServerJSON(additionalExpectations)
	if err != nil {
		return fmt.Errorf("failed to parse additional expectations: %w", err)
	}
	existingConfiguration.Expectations = append(existingConfiguration.Expectations, additionalConfigurations.Expectations...)
	return m.Provider.UpdateConfig(context.Background(), existingConfiguration)
}

func (m *CloudManager) generateMockConfiguration(cliContext *CLIContext) error {
	mockConfiguration, err := m.generateMockExpectations(cliContext)
	if err != nil {
		return fmt.Errorf("failed to generate mock expectations: %w", err)
	}
	return m.handleGeneratedMock(mockConfiguration)
}

// createNewProject handles new project creation flow. Expectations would be handled later.
func (m *CloudManager) createNewProject(project string) (models.ActionType, error) {
	var name string
	if project == "" {
		const maxAttempts = 3
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			if err := survey.AskOne(&survey.Input{
				Message: "Project name:",
				Help:    "Choose a unique name for your mock project",
			}, &name); err != nil {
				return models.ActionExit, err
			}
			if err := m.Provider.ValidateProjectName(name); err != nil {
				if attempt < maxAttempts {
					fmt.Printf("⚠️  Invalid name: %v. Try again (%d/%d).\n", err, attempt, maxAttempts)
					name = ""
					continue
				}
				return models.ActionExit, fmt.Errorf("invalid project name: %w", err)
			}
			break
		}
	} else {
		name = project
	}

	fmt.Printf("📂 Creating new project: %s\n", name)
	if err := m.Provider.InitProject(context.Background(), name); err != nil {
		return models.ActionExit, fmt.Errorf("failed to initialize project: %w", err)
	}
	return models.ActionGenerate, nil
}

// resolveProject determines the project name through CLI or interactive selection
func (m *CloudManager) resolveProject(cliContext *CLIContext) (models.ActionType, error) {
	if cliContext.HasProject() {
		// CLI-driven: use provided project name
		return m.handleCLIProject(cliContext.ProjectName)
	}

	// REPL-driven: interactive project selection
	return m.handleInteractiveProject()
}

// handleCLIProject processes CLI-provided project names
func (m *CloudManager) handleCLIProject(projectName string) (models.ActionType, error) {
	if err := m.Provider.ValidateProjectName(projectName); err != nil {
		return models.ActionExit, fmt.Errorf("invalid project name: %w", err)
	}
	exists, _ := m.Provider.ProjectExists(context.Background(), projectName)
	if exists {
		fmt.Printf("📂 Using existing project: %s\n", projectName)
		existingConfig, _ := m.Provider.GetConfig(context.Background(), projectName)
		return repl.SelectProjectAction(projectName, existingConfig), nil
	}
	return m.createNewProject(projectName)
}

// handleInteractiveProject manages interactive project selection via REPL
func (m *CloudManager) handleInteractiveProject() (models.ActionType, error) {
	projects, err := m.Provider.ListProjects(context.Background())
	if err != nil {
		return models.ActionExit, fmt.Errorf("failed to list existing projects: %w", err)
	}

	if len(projects) == 0 {
		// No existing projects - force new project creation
		fmt.Println("📂 No existing projects found. Let's create a new one.")
		return m.createNewProject("")
	}

	selectedProject, err := repl.ResolveProjectInteractively(projects)
	if err != nil {
		return models.ActionExit, fmt.Errorf("project selection failed: %w", err)
	}

	if strings.TrimSpace(selectedProject.ProjectID) == "" {
		return m.createNewProject("")
	}

	if strings.TrimSpace(selectedProject.ProjectID) == "Exit-Auto-Mock" {
		fmt.Println("👋 Good bye!!!")
		return models.ActionExit, nil
	}

	m.Provider.SetProjectName(selectedProject.ProjectID)
	m.Provider.SetStorageName(selectedProject.StorageName)
	existingConfig, _ := m.getMockConfiguration()
	return repl.SelectProjectAction(selectedProject.ProjectID, existingConfig), nil
}

// generateMockExpectations orchestrates mock expectation generation based on mode
func (m *CloudManager) generateMockExpectations(cliContext *CLIContext) (string, error) {
	fmt.Println("🧠 Starting mock expectation generation...")

	switch cliContext.GetMode() {
	case ModeCollection:
		// CLI-driven: Process collection file with AI assistance
		return repl.HandleCollectionMode(cliContext.CollectionType, cliContext.CollectionFile, m.getCurrentProject())

	case ModeInteractive:
		// REPL-driven: Interactive AI-guided configuration (primary experience)
		// Pass through any CLI provider override (e.g., --provider anthropic)
		return repl.StartMockGenerationREPL(m.getCurrentProject(), cliContext.Provider)
	default:
		return "", fmt.Errorf("unsupported initialization mode")
	}
}

func (m *CloudManager) destroyInfrastructureAndDeleteProject() error {
	fmt.Println("\n🗑️  Deleting project...")

	// Create Terraform manager
	destroyer, err := terraform.NewManager(m.getCurrentProject(), m.profile, m.getCloudProvider())
	if err != nil {
		return fmt.Errorf("failed to create terraform manager: %w", err)
	}

	status, _ := m.Provider.IsDeployed()
	if status {
		fmt.Println("\nDestroying infrastructure...")
		err = destroyer.Destroy()
		if err != nil {
			return err
		}
	}
	if err := m.Provider.DeleteProject(m.getCurrentProject()); err != nil {
		return fmt.Errorf("failed to delete project data: %w", err)
	}
	return nil
}

func (m *CloudManager) getMockConfiguration() (*models.MockConfiguration, error) {
	return m.Provider.GetConfig(context.Background(), m.Provider.GetProjectName())
}

func (m *CloudManager) getCurrentProject() string {
	return m.Provider.GetProjectName()
}

// handleRemoveExpectations processes the removal of expectations
func (m *CloudManager) handleRemoveExpectations(expManager *expectations.ExpectationManager, existingConfig *models.MockConfiguration) error {

	fmt.Printf("🗑️ Removing expectations for project: %s\n", m.getCurrentProject())
	// Prompt user for which expectations to remove (REPL handles UI)
	indicesToRemove, err := expManager.RemoveExpectations(existingConfig)
	if err != nil {
		return fmt.Errorf("remove selection failed: %w", err)
	}

	// No indices means user cancelled
	if len(indicesToRemove) == 0 {
		fmt.Println("✅ No expectations removed.")
		return nil
	}

	ctx := context.Background()
	// Get current configuration
	config, err := m.getMockConfiguration()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Special case: remove all (indices contains -1)
	if len(indicesToRemove) == 1 && indicesToRemove[0] == -1 {
		fmt.Println("\n🔄 Clearing project...")

		// Delete the configuration (empties the project)
		if err := m.Provider.DeleteProject(m.getCurrentProject()); err != nil {
			return fmt.Errorf("failed to clear expectations: %w", err)
		}

		fmt.Printf("\n✅ All expectations removed successfully!\n")
		return nil
	}

	// Partial removal: filter out selected indices
	filteredExpectations := []models.MockExpectation{}
	for i, exp := range config.Expectations {
		shouldRemove := false
		for _, idx := range indicesToRemove {
			if i == idx {
				shouldRemove = true
				break
			}
		}
		if !shouldRemove {
			filteredExpectations = append(filteredExpectations, exp)
		}
	}

	// Update configuration with filtered expectations
	config.Expectations = filteredExpectations
	config.Metadata.Version = fmt.Sprintf("v%d", time.Now().Unix())
	config.Metadata.UpdatedAt = time.Now()

	// Save updated configuration
	fmt.Printf("\n🔄 Creating new version with %d expectation(s) removed...\n", len(indicesToRemove))

	if err := m.Provider.UpdateConfig(ctx, config); err != nil {
		return fmt.Errorf("failed to save after removal: %w", err)
	}

	fmt.Printf("\n✅ Successfully removed %d expectation(s). Remaining: %d\n", len(indicesToRemove), len(filteredExpectations))
	return nil
}

func (m *CloudManager) handleEditExpectations(expManager *expectations.ExpectationManager, existingConfig *models.MockConfiguration) error {
	fmt.Printf("🛠️ Starting expectation editor for project: %s\n", existingConfig.GetProjectID())

	// Call expectations package for UI/editing (expectations handles UI)
	modifiedConfig, err := expManager.EditExpectations(existingConfig)
	if err != nil {
		return fmt.Errorf("edit failed: %w", err)
	}

	// User cancelled
	if modifiedConfig == nil {
		fmt.Println("✅ Edit cancelled.")
		return nil
	}

	// Save modified config back to storage (manager handles storage)
	ctx := context.Background()
	modifiedConfig.Metadata.Version = fmt.Sprintf("v%d", time.Now().Unix())
	modifiedConfig.Metadata.UpdatedAt = time.Now()

	if err := m.Provider.UpdateConfig(ctx, modifiedConfig); err != nil {
		return fmt.Errorf("failed to save changes: %w", err)
	}
	fmt.Printf("✅ Edit completed successfully!\n")
	fmt.Println("📊 Configuration updated and saved to cloud storage.")
	fmt.Println()
	return nil
}

// Handle final result
func (m *CloudManager) handleGeneratedMock(mockConfiguration string) error {
	for {
		var action string
		if err := survey.AskOne(&survey.Select{
			Message: "What would you like to do with this configuration?",
			Options: []string{
				"save - Save the expectation file",
				"view - View full JSON configuration",
				"local - Start MockServer locally",
				"exit - Exit without saving",
			},
		}, &action); err != nil {
			return err
		}

		action = strings.Split(action, " ")[0]

		switch models.ActionType(action) {
		case models.ActionSave:
			return m.saveToFile(mockConfiguration)
		case models.ActionLocal:
			return startLocalMockServer(mockConfiguration, m.getCurrentProject())
		case models.ActionView:
			fmt.Printf("\n📋 Full JSON Configuration:\n%s\n\n", mockConfiguration)
			continue
		case models.ActionExit:
			fmt.Println("\n⚠️  Are you sure you want to exit without saving?")
			fmt.Println("   • The uploaded expectations will not be saved")
			var confirmExit bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Exit without saving?",
				Default: false,
			}, &confirmExit); err != nil {
				return err
			}
			if confirmExit {
				fmt.Println("👋 Exiting without saving.")
				return nil
			}
			continue
		}
	}
}

func (m *CloudManager) saveToFile(mockServerJSON string) error {
	ctx := context.Background()

	fmt.Println("\n☁️ Saving expectation file to your cloud storage")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Parse MockServer JSON to MockConfiguration format
	mockConfig, err := models.ParseMockServerJSON(mockServerJSON)
	if err != nil {
		return fmt.Errorf("failed to parse mock server JSON: %w", err)
	}

	// Set metadata
	mockConfig.Metadata.ProjectID = m.getCurrentProject()
	mockConfig.Metadata.Provider = "auto-mock-cli"
	mockConfig.Metadata.Description = "Generated via interactive mock generation"
	mockConfig.Metadata.Version = fmt.Sprintf("v%d", time.Now().Unix())
	mockConfig.Metadata.CreatedAt = time.Now()
	mockConfig.Metadata.UpdatedAt = time.Now()

	// Persist via provider (cloud storage abstraction)
	if err := m.Provider.SaveConfig(ctx, mockConfig); err != nil {
		return fmt.Errorf("failed to persist configuration to cloud storage: %w", err)
	}

	fmt.Printf("\n✅ MockServer configuration saved to cloud storage!\n")
	fmt.Printf("📁 Project: %s\n", m.getCurrentProject())
	fmt.Printf("🔗 Configuration stored for team access\n")
	return nil
}

// Start local MockServer
func startLocalMockServer(mockServerJSON, projectName string) error {
	fmt.Println("\n🚀 Local MockServer Setup")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	configFile := fmt.Sprintf("%s-expectations.json", projectName)

	if err := os.WriteFile(configFile, []byte(mockServerJSON), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✅ Configuration saved as: %s\n\n", configFile)
	fmt.Println("🐳 Docker commands:")
	fmt.Println("1. Start MockServer:")
	fmt.Println("   docker run -d -p 1080:1080 -p 1090:1090 mockserver/mockserver:5.15.0")
	fmt.Println("2. Load expectations:")
	fmt.Printf("   curl -X PUT http://localhost:1080/mockserver/expectation -d @%s\n", configFile)
	fmt.Println("3. Access your API: http://localhost:1080")
	fmt.Println("4. View dashboard: http://localhost:1080/mockserver/dashboard")
	return nil
}

func (m *CloudManager) getCloudProvider() internal.Provider {
	return m.Provider
}
