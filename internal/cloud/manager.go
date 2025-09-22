package cloud

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sts"

	awscloud "github.com/hemantobora/auto-mock/internal/cloud/aws"
	"github.com/hemantobora/auto-mock/internal/provider"
	"github.com/hemantobora/auto-mock/internal/repl"
	"github.com/hemantobora/auto-mock/internal/utils"
)

// InitializationMode defines how the tool should operate
type InitializationMode int

const (
	// ModeInteractive - Primary mode: REPL-driven with AI guidance (default)
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
	Provider      string `json:"provider,omitempty"` // LLM provider preference
	IncludeAuth   bool   `json:"include_auth"`       // Include auth endpoints
	IncludeErrors bool   `json:"include_errors"`     // Include error responses
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
	profile string
}

// NewCloudManager creates a new cloud manager instance
func NewCloudManager(profile string) *CloudManager {
	return &CloudManager{
		profile: profile,
	}
}

// AutoDetectAndInit is the main entrypoint for the CLI's `init` command.
// It supports both interactive (REPL) and CLI-driven (collection import) workflows.
func AutoDetectAndInit(profile string, cliContext *CLIContext) error {
	manager := NewCloudManager(profile)
	return manager.Initialize(cliContext)
}

// Initialize runs the complete initialization workflow
func (m *CloudManager) Initialize(cliContext *CLIContext) error {
	// Step 1: Validate cloud provider credentials
	if err := m.validateCloudProviders(); err != nil {
		return err
	}

	// Step 2: Resolve project (CLI-driven or interactive)
	projectName, err := m.resolveProject(cliContext)
	if err != nil {
		// Handle special case: project deletion completed
		if strings.Contains(err.Error(), "PROJECT_DELETED") {
			return nil // Exit cleanly after successful deletion
		}
		return err
	}

	// Step 3: Initialize cloud infrastructure for project
	if err := m.initializeProjectInfrastructure(projectName); err != nil {
		return err
	}

	// Step 4: Generate mock configuration based on mode
	return m.generateMockConfiguration(projectName, cliContext)
}

// validateCloudProviders checks if valid cloud provider credentials exist
func (m *CloudManager) validateCloudProviders() error {
	validProviders := []string{}

	if m.checkAWSCredentials() {
		validProviders = append(validProviders, "aws")
		fmt.Println("🔍 Detected valid AWS credentials — proceeding with AWS provider...")
	}

	if len(validProviders) == 0 {
		return errors.New("❌ No valid cloud provider credentials found. Please configure AWS credentials")
	}

	if len(validProviders) > 1 {
		return errors.New("⚠️ Multiple cloud providers detected — interactive selection not yet implemented")
	}

	return nil
}

// resolveProject determines the project name through CLI or interactive selection
func (m *CloudManager) resolveProject(cliContext *CLIContext) (string, error) {
	if cliContext.HasProject() {
		// CLI-driven: use provided project name
		return m.handleCLIProject(cliContext.ProjectName)
	}

	// REPL-driven: interactive project selection
	return m.handleInteractiveProject()
}

// handleCLIProject processes CLI-provided project names
func (m *CloudManager) handleCLIProject(projectName string) (string, error) {
	buckets, err := awscloud.ListBucketsWithPrefix(m.profile, "auto-mock-")
	if err != nil {
		// If we can't list buckets, create new project
		return m.generateNewProject(projectName), nil
	}

	// Check if project exists
	existingProject := m.findExistingProject(buckets, projectName)
	if existingProject != "" {
		fmt.Printf("📂 Using existing project: %s\n", projectName)
		return existingProject, nil
	}

	// Create new project
	newProject := m.generateNewProject(projectName)
	fmt.Printf("📂 Creating new project: %s\n", projectName)
	return newProject, nil
}

// handleInteractiveProject manages interactive project selection via REPL
func (m *CloudManager) handleInteractiveProject() (string, error) {
	buckets, err := awscloud.ListBucketsWithPrefix(m.profile, "auto-mock-")
	if err != nil {
		return "", fmt.Errorf("failed to list existing projects: %w", err)
	}

	selectedProject, exists, err := repl.ResolveProjectInteractively(buckets)
	if err != nil {
		return "", fmt.Errorf("project selection failed: %w", err)
	}

	if exists {
		return m.handleExistingProjectAction(selectedProject)
	}

	return selectedProject, nil
}

// handleExistingProjectAction processes user actions on existing projects
func (m *CloudManager) handleExistingProjectAction(projectName string) (string, error) {
	action := repl.SelectProjectAction(projectName)
	cleanName := utils.ExtractUserProjectName(projectName)

	switch action {
	case "Generate":
		fmt.Printf("🚀 Proceeding with mock generation for project: %s\n", cleanName)
		return projectName, nil

	case "Edit":
		fmt.Printf("🛠️ Edit stubs for project '%s' coming soon...\n", cleanName)
		return "", fmt.Errorf("edit functionality not implemented yet")

	case "Delete":
		fmt.Printf("🗑️ Deleting project: %s\n", cleanName)
		return "", m.deleteProject(projectName)

	case "Cancel":
		return "", fmt.Errorf("operation cancelled by user")

	default:
		return "", fmt.Errorf("invalid action selected: %s", action)
	}
}

// initializeProjectInfrastructure sets up cloud infrastructure for the project
func (m *CloudManager) initializeProjectInfrastructure(projectName string) error {
	awsProvider, err := awscloud.NewProvider(m.profile, projectName)
	if err != nil {
		return fmt.Errorf("failed to initialize AWS provider: %w", err)
	}

	var prov provider.Provider = awsProvider
	if err := prov.InitProject(); err != nil {
		return fmt.Errorf("failed to initialize project infrastructure: %w", err)
	}

	cleanName := utils.ExtractUserProjectName(projectName)
	fmt.Printf("✅ Project '%s' initialized successfully!\n", cleanName)
	return nil
}

// generateMockConfiguration orchestrates mock config generation based on mode
func (m *CloudManager) generateMockConfiguration(projectName string, cliContext *CLIContext) error {
	fmt.Println("🧠 Starting mock configuration generation...")

	switch cliContext.GetMode() {
	case ModeCollection:
		// CLI-driven: Process collection file with AI assistance
		return m.handleCollectionMode(projectName, cliContext)

	case ModeInteractive:
		// REPL-driven: Interactive AI-guided configuration (primary experience)
		return m.handleInteractiveMode(projectName, cliContext)

	default:
		return fmt.Errorf("unsupported initialization mode")
	}
}

// handleCollectionMode processes collection files with AI assistance
func (m *CloudManager) handleCollectionMode(projectName string, cliContext *CLIContext) error {
	cleanName := utils.ExtractUserProjectName(projectName)
	fmt.Printf("📂 Processing %s collection for project: %s\n", cliContext.CollectionType, cleanName)

	// Validate collection parameters
	if cliContext.CollectionType == "" {
		return fmt.Errorf("collection-type is required when using collection-file")
	}

	// The actual collection processing will be handled by REPL with pre-loaded context
	// This allows the AI to still ask intelligent questions about the collection
	return repl.StartCollectionImportREPL(projectName, cliContext.CollectionFile, cliContext.CollectionType)
}

// handleInteractiveMode starts the interactive REPL experience
func (m *CloudManager) handleInteractiveMode(projectName string, cliContext *CLIContext) error {
	cleanName := utils.ExtractUserProjectName(projectName)
	fmt.Printf("🎛️  Starting interactive mock generation for project: %s\n", cleanName)

	// Pass CLI context to REPL for any user preferences (provider, auth, etc.)
	return repl.StartMockGenerationREPL(projectName)
}

// Helper methods

// findExistingProject searches for an existing project by name
func (m *CloudManager) findExistingProject(buckets []string, projectName string) string {
	for _, bucket := range buckets {
		trimmed := utils.RemoveBucketPrefix(bucket)
		parts := strings.Split(trimmed, "-")
		if len(parts) < 2 {
			continue
		}
		base := strings.Join(parts[:len(parts)-1], "-")
		if strings.EqualFold(base, projectName) {
			return trimmed
		}
	}
	return ""
}

// generateNewProject creates a new project name with random suffix
func (m *CloudManager) generateNewProject(projectName string) string {
	suffix, _ := utils.GenerateRandomSuffix()
	return fmt.Sprintf("%s-%s", projectName, suffix)
}

// deleteProject removes a project and its infrastructure
func (m *CloudManager) deleteProject(projectName string) error {
	awsProvider, err := awscloud.NewProvider(m.profile, projectName)
	if err != nil {
		return fmt.Errorf("failed to initialize AWS provider for deletion: %w", err)
	}

	var prov provider.Provider = awsProvider
	if err := prov.DeleteProject(); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	// Project deleted successfully - exit the program cleanly
	fmt.Println("✅ Project deletion completed successfully!")
	return fmt.Errorf("PROJECT_DELETED") // Special error code to signal completion
}

// checkAWSCredentials verifies if AWS credentials are configured and valid
func (m *CloudManager) checkAWSCredentials() bool {
	cfg, err := awscloud.LoadAWSConfig(m.profile)
	if err != nil {
		return false
	}
	client := sts.NewFromConfig(cfg)
	_, err = client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	return err == nil
}
