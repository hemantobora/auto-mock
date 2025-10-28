package repl

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/mcp"
	"github.com/hemantobora/auto-mock/internal/models"
)

// StartMockGenerationREPL is the main entry point for mock generation
func StartMockGenerationREPL(projectName string) (string, error) {
	fmt.Printf("🎯 MockServer Configuration Generator Initialized\n")
	fmt.Printf("📦 Project: %s\n", projectName)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Simple mock config generation for now
	// Step 1: Choose generation method
	var method string
	if err := survey.AskOne(&survey.Select{
		Message: "How do you want to generate your mock configuration?",
		Options: []string{
			"interactive - Build endpoints step-by-step (7-step builder)",
			"collection - Import from Postman/Bruno/Insomnia",
			"describe - Describe your API in natural language (AI-powered)",
			"upload - Upload expectation file directly (JSON)",
		},
		Default: "interactive - Build endpoints step-by-step (7-step builder)",
	}, &method); err != nil {
		return "", err
	}

	method = strings.Split(method, " ")[0]

	// Step 2: Generate mock configuration using MCP engine
	mockServerJSON, err := generateMockConfiguration(method, projectName)
	if err != nil {
		return "", fmt.Errorf("failed to generate configuration: %w", err)
	}

	// Only show menu if we have JSON to work with
	if mockServerJSON == "" {
		return "", fmt.Errorf("no configuration generated")
	}

	// Display the result
	fmt.Println("\n📋 Generated MockServer Configuration:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(mockServerJSON)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Handle the result (save, deploy, etc.)
	// return handleFinalResult(mockServerJSON, projectName)
	return mockServerJSON, nil
}

func ResolveProjectInteractively(existing []models.ProjectInfo) (models.ProjectInfo, error) {
	var options []string
	var nameToProject map[string]models.ProjectInfo = make(map[string]models.ProjectInfo)
	for _, info := range existing {
		options = append(options, info.ProjectID)
		nameToProject[info.ProjectID] = info
	}
	options = append(options, "📝 Create New Project")

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "Select project:",
		Options: options,
	}, &choice); err != nil {
		return models.ProjectInfo{}, err
	}

	if strings.Contains(choice, "Create New") {
		return models.ProjectInfo{}, nil
	}
	return nameToProject[choice], nil
}

func SelectProjectAction(projectName string, existingConfig *models.MockConfiguration) models.ActionType {
	var action string

	// Check if expectations already exist
	expectationsExist := existingConfig != nil && existingConfig.Expectations != nil && len(existingConfig.Expectations) > 0
	var options []string
	if expectationsExist {
		// When expectations exist: management operations + view/download
		options = []string{
			"view - View expectations or entire configuration file",
			"download - Download the entire expectations file",
			"edit - Edit a particular expectation (modify method, path, response, etc.)",
			"remove - Remove specific expectations while keeping others",
			"replace - Replace ALL existing expectations with new ones",
			"delete - Delete the entire project and tear down infrastructure (if running)",
			"add - Add new expectations to existing ones",
			"cancel - Cancel the operation and exit",
		}
	} else {
		// When no expectations exist: only generation (no management operations)
		options = []string{
			"generate - Create a set of expectations from Collection, Interactively or examples",
			"cancel - Cancel the operation and exit",
		}
	}

	survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Project: %s - What would you like to do?", projectName),
		Options: options,
	}, &action)

	// Extract the action keyword (first word before " - ")
	return models.ActionType(strings.Split(action, " ")[0])
}

// generateMockConfiguration uses the MCP engine to generate configurations
// Returns: (mockServerJSON, error)
func generateMockConfiguration(method, projectName string) (string, error) {
	ctx := context.Background()
	switch method {
	case "describe":
		return generateFromDescription(ctx, projectName)
	case "interactive":
		return generateInteractiveWithMenu()
	case "collection":
		return generateFromCollectionWithMenu(projectName)
	case "upload":
		return configureUploadedExpectationWithMenu(projectName)
	default:
		return generateFromDescription(ctx, projectName)
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
