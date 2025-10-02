package repl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/mcp"
	"github.com/hemantobora/auto-mock/internal/state"
	"github.com/hemantobora/auto-mock/internal/utils"
)

// DeploymentConfig contains complete deployment configuration for ECS
type DeploymentConfig struct {
	ProjectName  string        `json:"project_name"`
	Environment  string        `json:"environment"`
	Provider     string        `json:"provider"`
	MockConfig   string        `json:"mock_config"`
	TTL          *TTLConfig    `json:"ttl"`
	Domain       *DomainConfig `json:"domain"`
	Region       string        `json:"region"`
	InstanceSize string        `json:"instance_size"`
}

// TTLConfig contains auto-teardown configuration
type TTLConfig struct {
	Enabled           bool   `json:"enabled"`
	Hours             int    `json:"hours"`
	NotificationEmail string `json:"notification_email,omitempty"`
}

// DomainConfig contains domain and TLS configuration
type DomainConfig struct {
	Type         string `json:"type"` // auto, custom
	CustomDomain string `json:"custom_domain,omitempty"`
	TLSEnabled   bool   `json:"tls_enabled"`
	DNSProvider  string `json:"dns_provider"` // always "route53"
	HostedZoneId string `json:"hosted_zone_id,omitempty"`
	AutoDNSSetup bool   `json:"auto_dns_setup"`
}

// DeploymentResult contains deployment outcome information
type DeploymentResult struct {
	DeploymentID   string    `json:"deployment_id"`
	ProjectName    string    `json:"project_name"`
	Environment    string    `json:"environment"`
	Provider       string    `json:"provider"`
	MockServerURL  string    `json:"mockserver_url"`
	DashboardURL   string    `json:"dashboard_url"`
	TTLExpiry      time.Time `json:"ttl_expiry,omitempty"`
	TerraformState string    `json:"terraform_state_location"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// StartMockGenerationREPL is the main entry point for mock generation
func StartMockGenerationREPL(projectName string) error {
	cleanName := utils.ExtractUserProjectName(projectName)
	fmt.Printf("🎯 MockServer Configuration Generator\n")
	fmt.Printf("📦 Project: %s\n", cleanName)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Simple mock config generation for now
	// Step 1: Choose generation method
	var method string
	if err := survey.AskOne(&survey.Select{
		Message: "How do you want to generate your mock configuration?",
		Options: []string{
			"describe - Describe your API in natural language (AI-powered)",
			"interactive - Build endpoints step-by-step (7-step builder)",
			"collection - Import from Postman/Bruno/Insomnia",
			"template - Quick templates for common APIs",
			"upload - Upload expectation file directly (JSON)",
		},
		Default: "describe - Describe your API in natural language (AI-powered)",
	}, &method); err != nil {
		return err
	}

	method = strings.Split(method, " ")[0]

	// Step 2: Generate mock configuration using MCP engine
	mockServerJSON, err := generateMockConfiguration(method, projectName)
	if err != nil {
		return fmt.Errorf("failed to generate configuration: %w", err)
	}

	// Only show menu if we have JSON to work with
	if mockServerJSON == "" {
		return fmt.Errorf("no configuration generated")
	}

	// Display the result
	displayResult(mockServerJSON)

	// Handle the result (save, deploy, etc.)
	return handleFinalResult(mockServerJSON, projectName)
}

// Display result
func displayResult(mockServerJSON string) {
	fmt.Println("\n📋 Generated MockServer Configuration:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(mockServerJSON)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// Compatibility functions for existing code
func StartCollectionImportREPL(projectName, collectionFile, collectionType string) error {
	fmt.Printf("📂 Collection import for %s files will be implemented soon.\n", collectionType)
	return StartMockGenerationREPL(projectName)
}

func ResolveProjectInteractively(existing []string) (string, bool, error) {
	if len(existing) == 0 {
		return createNewProject()
	}

	var options []string
	for _, bucket := range existing {
		cleanName := utils.ExtractUserProjectName(utils.RemoveBucketPrefix(bucket))
		if cleanName != "" {
			options = append(options, cleanName)
		}
	}
	options = append(options, "📝 Create New Project")

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "Select project:",
		Options: options,
	}, &choice); err != nil {
		return "", false, err
	}

	if strings.Contains(choice, "Create New") {
		return createNewProject()
	}

	for _, bucket := range existing {
		if strings.Contains(bucket, choice) {
			return utils.RemoveBucketPrefix(bucket), true, nil
		}
	}

	return "", false, fmt.Errorf("project not found")
}

func createNewProject() (string, bool, error) {
	var name string
	if err := survey.AskOne(&survey.Input{
		Message: "Project name:",
		Help:    "Choose a unique name for your mock project",
	}, &name); err != nil {
		return "", false, err
	}

	suffix, _ := utils.GenerateRandomSuffix()
	return fmt.Sprintf("%s-%s", name, suffix), false, nil
}

// CheckExpectationsExist checks if expectations already exist for a project
func CheckExpectationsExist(projectName string) bool {
	ctx := context.Background()
	cleanName := utils.ExtractUserProjectName(projectName)

	// Initialize S3 store using factory
	store, err := state.StoreForProject(ctx, projectName)
	if err != nil {
		fmt.Printf("Warning: Failed to create store for expectation check: %v\n", err)
		return false
	}

	// Try to get existing config
	_, err = store.GetConfig(ctx, cleanName)
	if err != nil {
		// Config doesn't exist or error occurred
		return false
	}

	return true
}

func SelectProjectAction(projectName string) string {
	cleanName := utils.ExtractUserProjectName(projectName)
	var action string

	// Check if expectations already exist
	expectationsExist := CheckExpectationsExist(projectName)

	var options []string
	if expectationsExist {
		// When expectations exist: management operations + view/download
		options = []string{
			"view - View expectations or entire configuration file",
			"download - Download the entire expectations file",
			"edit - Edit a particular expectation (modify method, path, response, etc.)",
			"remove - Remove specific expectations while keeping others",
			"replace - Replace ALL existing expectations with new ones",
			"delete - Delete the entire project and tear down infrastructure",
			"cancel - Cancel the operation and exit",
		}
	} else {
		// When no expectations exist: only generation (no management operations)
		options = []string{
			"generate - Create a set of expectations from API documentation or examples",
			"cancel - Cancel the operation and exit",
		}
	}

	survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Project: %s - What would you like to do?", cleanName),
		Options: options,
	}, &action)

	// Extract the action keyword (first word before " - ")
	return strings.Split(action, " ")[0]
}

// generateMockConfiguration uses the MCP engine to generate configurations
// Returns: (mockServerJSON, error)
func generateMockConfiguration(method, projectName string) (string, error) {
	ctx := context.Background()
	cleanName := utils.ExtractUserProjectName(projectName)

	switch method {
	case "describe":
		return generateFromDescription(ctx, cleanName)
	case "interactive":
		return generateInteractiveWithMenu(cleanName)
	case "collection":
		return generateFromCollectionWithMenu(cleanName)
	case "template":
		return generateFromTemplate(ctx, cleanName)
	case "upload":
		return configureUploadedExpectationWithMenu(cleanName)
	default:
		return generateFromDescription(ctx, cleanName)
	}
}

// generateFromDescription uses AI to generate from natural language
func generateFromDescription(ctx context.Context, projectName string) (string, error) {
	fmt.Println("🤖 AI-Powered Generation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Check available providers
	providers := mcp.ListProviders()
	fmt.Println("🔍 Available AI Providers:")
	for _, provider := range providers {
		if provider.Available {
			fmt.Printf("✅ %s (%s)\n", provider.Name, provider.Cost)
		} else {
			fmt.Printf("❌ %s (not configured)\n", provider.Name)
		}
	}
	fmt.Println()

	// Get user description
	var description string
	if err := survey.AskOne(&survey.Multiline{
		Message: "Describe your API (be specific about endpoints, data, functionality):",
		Help:    "Example: 'User management API with signup, login, profile endpoints. Users have email, name, role.'",
	}, &description); err != nil {
		return "", err
	}

	if strings.TrimSpace(description) == "" {
		return "", fmt.Errorf("description cannot be empty")
	}

	// Generate using MCP
	fmt.Println("🤖 Generating with AI...")
	result, err := mcp.GenerateWithProvider(ctx, description, "", projectName)
	if err != nil {
		return "", fmt.Errorf("AI generation failed: %w", err)
	}

	// Show generation info
	fmt.Printf("✅ Generated by: %s\n", result.Provider)
	if result.TokensUsed > 0 {
		fmt.Printf("📊 Tokens used: %d\n", result.TokensUsed)
	}
	fmt.Printf("⏱️  Generation time: %s\n", result.GenerationTime)

	// Show warnings/suggestions
	for _, warning := range result.Warnings {
		fmt.Printf("⚠️  %s\n", warning)
	}
	for _, suggestion := range result.Suggestions {
		fmt.Printf("💡 %s\n", suggestion)
	}

	return result.MockServerJSON, nil
}

// generateFromTemplate uses quick templates
func generateFromTemplate(ctx context.Context, projectName string) (string, error) {
	fmt.Println("📝 Template Generator")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Choose template type
	var templateType string
	if err := survey.AskOne(&survey.Select{
		Message: "Choose API template:",
		Options: []string{
			"user - User management API",
			"auth - Authentication API",
			"product - Product/catalog API",
			"order - Order/payment API",
			"file - File upload API",
			"custom - Custom CRUD API",
		},
	}, &templateType); err != nil {
		return "", err
	}

	templateType = strings.Split(templateType, " ")[0]

	// Generate from template
	result, err := mcp.GenerateWithProvider(ctx, templateType+" management API", "template", projectName)
	if err != nil {
		return "", fmt.Errorf("template generation failed: %w", err)
	}

	return result.MockServerJSON, nil
}
