package mcp

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hemantobora/auto-mock/internal/mcp/providers"
	"github.com/hemantobora/auto-mock/internal/mcp/security"
	"github.com/hemantobora/auto-mock/internal/models"
)

// MockSpecEngine handles mock specification generation for CLI/REPL
type MockSpecEngine struct {
	providerManager *providers.ProviderManager
	sanitizer       *security.CollectionSanitizer
	logger          *log.Logger
}

// Global engine instance for backward compatibility
var defaultEngine *MockSpecEngine

// Initialize the engine on package load
func init() {
	logger := log.New(os.Stdout, "[AutoMock] ", log.LstdFlags)
	defaultEngine = NewMockSpecEngine(logger)
}

// NewMockSpecEngine creates a new mock spec engine
func NewMockSpecEngine(logger *log.Logger) *MockSpecEngine {
	return &MockSpecEngine{
		providerManager: providers.NewProviderManager(),
		sanitizer:       security.NewCollectionSanitizer(),
		logger:          logger,
	}
}

// RunMCP - Your existing function signature, now powered by providers
// This maintains backward compatibility with your existing code
func RunMCP(input string) (string, error) {
	ctx := context.Background()

	options := &providers.GenerationOptions{
		IncludeExamples: true,
		IncludeAuth:     false,
		IncludeErrors:   true,
		InputType:       "description",
		AllowPaid:       true, // Allow paid providers by default
	}

	result, err := defaultEngine.GenerateMockConfig(ctx, input, options)
	if err != nil {
		return "", err
	}

	return result.MockServerJSON, nil
}

// ===== NEW CLI/REPL METHODS =====

// GenerateMockConfig - Enhanced version for CLI with full options
func (e *MockSpecEngine) GenerateMockConfig(ctx context.Context, input string, options *providers.GenerationOptions) (*providers.GenerationResult, error) {
	if options == nil {
		options = e.getDefaultOptions()
	}

	// Log generation start
	if e.logger != nil {
		e.logger.Printf("🧠 Generating mock configuration...")
		if options.ProjectName != "" {
			e.logger.Printf("📝 Project: %s", options.ProjectName)
		}
		e.logger.Printf("📋 Input type: %s", options.InputType)
	}

	result, err := e.providerManager.GenerateMockConfig(ctx, input, options)
	if err != nil {
		// Wrap with AIGenerationError for better context
		return nil, &models.AIGenerationError{
			Provider: options.PreferredProvider,
			Input:    input,
			Cause:    err,
		}
	}

	return result, nil
}

// GenerateFromDescription - CLI method for natural language input
func (e *MockSpecEngine) GenerateFromDescription(ctx context.Context, description string, options *GenerationOptions) (*GenerationResult, error) {
	providerOptions := e.convertToProviderOptions(options)
	providerOptions.InputType = "description"

	result, err := e.providerManager.GenerateMockConfig(ctx, description, providerOptions)
	if err != nil {
		return nil, err
	}

	return e.convertFromProviderResult(result), nil
}

// GenerateFromCollection - CLI method for collection input (Postman, Bruno, Insomnia)
func (e *MockSpecEngine) GenerateFromCollection(ctx context.Context, collectionData []byte, collectionType string, options *GenerationOptions) (*GenerationResult, error) {
	if e.logger != nil {
		e.logger.Printf("📂 Processing %s collection...", collectionType)
	}

	// Sanitize collection first (remove credentials)
	sanitizationResult, err := e.sanitizer.SanitizeCollection(collectionData, collectionType)
	if err != nil {
		return nil, fmt.Errorf("failed to sanitize collection: %w", err)
	}

	// Log security findings
	if len(sanitizationResult.CredentialLocations) > 0 {
		if e.logger != nil {
			e.logger.Printf("🔒 Found and secured %d credential(s)", len(sanitizationResult.CredentialLocations))
		}
	}

	// Convert sanitized collection to prompt
	prompt := e.collectionToPrompt(sanitizationResult.SanitizedCollection)

	// Generate mock config
	providerOptions := e.convertToProviderOptions(options)
	providerOptions.InputType = collectionType

	result, err := e.providerManager.GenerateMockConfig(ctx, prompt, providerOptions)
	if err != nil {
		return nil, err
	}

	// Add security warnings to result
	convertedResult := e.convertFromProviderResult(result)
	convertedResult.SecurityWarnings = sanitizationResult.SecurityWarnings
	convertedResult.CredentialsSanitized = len(sanitizationResult.CredentialLocations)

	return convertedResult, nil
}

// GetAvailableProviders - CLI method to list providers
func (e *MockSpecEngine) GetAvailableProviders() []ProviderInfo {
	providerInfos := e.providerManager.GetAvailableProviders()

	// Convert to our format
	var results []ProviderInfo
	for _, info := range providerInfos {
		results = append(results, ProviderInfo{
			Name:      info.Name,
			Available: info.Available,
			Cost:      info.Cost,
			IsFree:    info.Cost == "Free",
		})
	}

	return results
}

// CheckProviderStatus - CLI method to check provider health
func (e *MockSpecEngine) CheckProviderStatus(ctx context.Context) error {
	if e.logger != nil {
		e.logger.Println("🔍 Checking provider status...")
	}

	providers := e.providerManager.GetAvailableProviders()
	if len(providers) == 0 {
		return fmt.Errorf("no providers available")
	}

	availableCount := 0
	for _, provider := range providers {
		if provider.Available {
			availableCount++
			if e.logger != nil {
				e.logger.Printf("✅ %s: Available (%s)", provider.Name, provider.Cost)
			}
		} else {
			if e.logger != nil {
				e.logger.Printf("❌ %s: Not available (missing API key?)", provider.Name)
			}
		}
	}

	if availableCount == 0 {
		return fmt.Errorf("no providers are available - configure API keys or use template provider")
	}

	if e.logger != nil {
		e.logger.Printf("🎯 %d provider(s) available", availableCount)
	}

	return nil
}

// ===== CONVENIENCE FUNCTIONS FOR CLI =====

// GenerateQuick - Simple one-liner for basic CLI usage
func GenerateQuick(description string) (string, error) {
	return RunMCP(description)
}

// GenerateWithProvider - CLI function with specific provider selection
func GenerateWithProvider(ctx context.Context, description string, providerName string, projectName string) (*GenerationResult, error) {
	options := &GenerationOptions{
		ProjectName:       projectName,
		PreferredProvider: providerName,
		IncludeExamples:   true,
		IncludeAuth:       false,
		IncludeErrors:     true,
		AllowPaid:         true,
	}

	return defaultEngine.GenerateFromDescription(ctx, description, options)
}

// GenerateFromPostman - CLI function for Postman collections
func GenerateFromPostman(ctx context.Context, collectionData []byte, projectName string) (*GenerationResult, error) {
	options := &GenerationOptions{
		ProjectName:     projectName,
		IncludeExamples: true,
		IncludeAuth:     true,
		IncludeErrors:   true,
		AllowPaid:       true,
	}

	return defaultEngine.GenerateFromCollection(ctx, collectionData, "postman", options)
}

// GenerateFromBruno - CLI function for Bruno collections
func GenerateFromBruno(ctx context.Context, collectionData []byte, projectName string) (*GenerationResult, error) {
	options := &GenerationOptions{
		ProjectName:     projectName,
		IncludeExamples: true,
		IncludeAuth:     true,
		IncludeErrors:   true,
		AllowPaid:       true,
	}

	return defaultEngine.GenerateFromCollection(ctx, collectionData, "bruno", options)
}

// GenerateFromInsomnia - CLI function for Insomnia collections
func GenerateFromInsomnia(ctx context.Context, collectionData []byte, projectName string) (*GenerationResult, error) {
	options := &GenerationOptions{
		ProjectName:     projectName,
		IncludeExamples: true,
		IncludeAuth:     true,
		IncludeErrors:   true,
		AllowPaid:       true,
	}

	return defaultEngine.GenerateFromCollection(ctx, collectionData, "insomnia", options)
}

// CheckProviders - CLI function to check provider status
func CheckProviders() error {
	ctx := context.Background()
	return defaultEngine.CheckProviderStatus(ctx)
}

// ListProviders - CLI function to list available providers
func ListProviders() []ProviderInfo {
	return defaultEngine.GetAvailableProviders()
}

// NewTemplateProvider - Create a new template provider for direct use
func NewTemplateProvider() *providers.TemplateProvider {
	return providers.NewTemplateProvider()
}

// ===== TYPE DEFINITIONS FOR CLI/REPL =====

// GenerationOptions for CLI/REPL usage (simpler than provider options)
type GenerationOptions struct {
	ProjectName       string `json:"project_name,omitempty"`
	PreferredProvider string `json:"preferred_provider,omitempty"`
	IncludeExamples   bool   `json:"include_examples"`
	IncludeAuth       bool   `json:"include_auth"`
	IncludeErrors     bool   `json:"include_errors"`
	AllowPaid         bool   `json:"allow_paid"`
}

// GenerationResult for CLI/REPL (includes additional CLI-specific fields)
type GenerationResult struct {
	MockServerJSON string   `json:"mockserver_json"`
	Provider       string   `json:"provider"`
	TokensUsed     int      `json:"tokens_used,omitempty"`
	GenerationTime string   `json:"generation_time,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
	Suggestions    []string `json:"suggestions,omitempty"`

	// CLI-specific fields
	SecurityWarnings     []string `json:"security_warnings,omitempty"`
	CredentialsSanitized int      `json:"credentials_sanitized,omitempty"`
}

// ProviderInfo for CLI display
type ProviderInfo struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Cost      string `json:"cost"`
	IsFree    bool   `json:"is_free"`
}

// ===== HELPER METHODS =====

func (e *MockSpecEngine) getDefaultOptions() *providers.GenerationOptions {
	return &providers.GenerationOptions{
		IncludeExamples: true,
		IncludeAuth:     false,
		IncludeErrors:   true,
		InputType:       "description",
		AllowPaid:       true,  // DEFAULT TO TRUE - Use AI providers when available!
	}
}

func (e *MockSpecEngine) convertToProviderOptions(options *GenerationOptions) *providers.GenerationOptions {
	if options == nil {
		return e.getDefaultOptions()
	}

	return &providers.GenerationOptions{
		ProjectName:       options.ProjectName,
		IncludeExamples:   options.IncludeExamples,
		IncludeAuth:       options.IncludeAuth,
		IncludeErrors:     options.IncludeErrors,
		PreferredProvider: options.PreferredProvider,
		AllowPaid:         options.AllowPaid,
		InputType:         "description", // Will be overridden for collections
	}
}

func (e *MockSpecEngine) convertFromProviderResult(result *providers.GenerationResult) *GenerationResult {
	return &GenerationResult{
		MockServerJSON: result.MockServerJSON,
		Provider:       result.Provider,
		TokensUsed:     result.TokensUsed,
		GenerationTime: fmt.Sprintf("%.2fs", float64(time.Since(result.Timestamp).Nanoseconds())/1e9),
		Warnings:       result.Warnings,
		Suggestions:    result.Suggestions,
	}
}

func (e *MockSpecEngine) collectionToPrompt(collection *security.SanitizedCollection) string {
	prompt := fmt.Sprintf("Convert this %s collection to MockServer JSON:\n\n", collection.OriginalFormat)
	prompt += fmt.Sprintf("Collection: %s\n", collection.Name)

	if collection.Description != "" {
		prompt += fmt.Sprintf("Description: %s\n", collection.Description)
	}

	prompt += fmt.Sprintf("Endpoints (%d):\n", collection.EndpointCount)
	for _, endpoint := range collection.Endpoints {
		prompt += fmt.Sprintf("- %s %s", endpoint.Method, endpoint.Path)
		if endpoint.Name != "" {
			prompt += fmt.Sprintf(" (%s)", endpoint.Name)
		}
		if endpoint.RequiresAuth {
			prompt += " [Auth Required]"
		}
		prompt += "\n"
	}

	if collection.HasAuthentication {
		prompt += "\nAuthentication schemes found:\n"
		for _, auth := range collection.AuthSchemes {
			prompt += fmt.Sprintf("- %s (%s)\n", auth.Type, auth.Description)
		}
	}

	if len(collection.Variables) > 0 {
		prompt += "\nVariables:\n"
		for key, value := range collection.Variables {
			prompt += fmt.Sprintf("- %s: %s\n", key, value)
		}
	}

	return prompt
}
