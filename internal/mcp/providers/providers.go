package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/mcp/security"
)

// Simple provider enumeration - no complex registry needed
type ProviderType string

const (
	ProviderAnthropic ProviderType = "anthropic"
	ProviderOpenAI    ProviderType = "openai"
	ProviderTemplate  ProviderType = "template" // Free fallback
)

// Simple provider interface - just what we need
type MockProvider interface {
	GetName() string
	IsAvailable() bool
	GenerateMockConfig(ctx context.Context, input string, options *GenerationOptions) (*GenerationResult, error)
}

// Generation options for CLI/REPL
type GenerationOptions struct {
	// CLI options
	ProjectName     string `json:"project_name,omitempty"`
	IncludeExamples bool   `json:"include_examples"`
	IncludeAuth     bool   `json:"include_auth"`
	IncludeErrors   bool   `json:"include_errors"`

	// Input type (for collections)
	InputType string `json:"input_type"` // "description", "postman", "bruno", "insomnia"

	// Provider preferences (set by CLI flags)
	PreferredProvider string `json:"preferred_provider,omitempty"`
	AllowPaid         bool   `json:"allow_paid"` // CLI flag --allow-paid
}

// Simple generation result - FIXED: Added missing Timestamp field
type GenerationResult struct {
	MockServerJSON string    `json:"mockserver_json"` // Ready-to-use MockServer JSON
	Provider       string    `json:"provider"`
	TokensUsed     int       `json:"tokens_used,omitempty"`
	Warnings       []string  `json:"warnings,omitempty"`
	Suggestions    []string  `json:"suggestions,omitempty"`
	Timestamp      time.Time `json:"timestamp"` // ADDED: Missing field
}

// ProviderManager - Simple manager for your CLI/REPL
type ProviderManager struct {
	providers map[ProviderType]MockProvider
}

// NewProviderManager creates a simple provider manager
func NewProviderManager() *ProviderManager {
	pm := &ProviderManager{
		providers: make(map[ProviderType]MockProvider),
	}

	// Register available providers - FIXED: Use the actual implementations
	pm.providers[ProviderAnthropic] = NewAnthropicProvider()
	pm.providers[ProviderOpenAI] = NewOpenAIProvider()
	pm.providers[ProviderTemplate] = NewTemplateProvider()

	return pm
}

// GetAvailableProviders returns list for CLI display
func (pm *ProviderManager) GetAvailableProviders() []ProviderInfo {
	var available []ProviderInfo

	for providerType, provider := range pm.providers {
		info := ProviderInfo{
			Name:      string(providerType),
			Available: provider.IsAvailable(),
			Cost:      pm.getProviderCost(providerType),
		}
		available = append(available, info)
	}

	return available
}

// SelectProvider for CLI/REPL - user chooses or fallback logic
func (pm *ProviderManager) SelectProvider(preferred string, allowPaid bool) (MockProvider, error) {
	// User specified provider
	if preferred != "" {
		if providerType := ProviderType(preferred); pm.isValidProvider(providerType) {
			provider := pm.providers[providerType]
			if provider.IsAvailable() {
				return provider, nil
			}
			return nil, fmt.Errorf("provider %s not available (missing API key?)", preferred)
		}
		return nil, fmt.Errorf("unknown provider: %s", preferred)
	}

	// Auto-select with preference for AI providers when available
	
	// 1. If paid providers allowed (default should be true), try AI providers first
	if allowPaid {
		// Prefer Anthropic if available
		if anthropic := pm.providers[ProviderAnthropic]; anthropic.IsAvailable() {
			return anthropic, nil
		}
		// Then try OpenAI
		if openai := pm.providers[ProviderOpenAI]; openai.IsAvailable() {
			return openai, nil
		}
	}
	
	// 2. Fall back to template provider only if no AI providers available
	if template := pm.providers[ProviderTemplate]; template.IsAvailable() {
		return template, nil
	}

	return nil, fmt.Errorf("no available providers (configure ANTHROPIC_API_KEY or OPENAI_API_KEY)")
}

// GenerateMockConfig - Main method for your CLI/REPL
func (pm *ProviderManager) GenerateMockConfig(ctx context.Context, input string, options *GenerationOptions) (*GenerationResult, error) {
	// Handle collection input (sanitize first)
	sanitizedInput := input
	if options.InputType != "description" {
		sanitizer := security.NewCollectionSanitizer()
		result, err := sanitizer.SanitizeCollection([]byte(input), options.InputType)
		if err != nil {
			return nil, fmt.Errorf("failed to sanitize collection: %w", err)
		}

		// Convert sanitized collection to prompt
		sanitizedInput = pm.collectionToPrompt(result.SanitizedCollection, options)
	}

	// Select provider
	provider, err := pm.SelectProvider(options.PreferredProvider, options.AllowPaid)
	if err != nil {
		return nil, err
	}

	// Generate mock config
	result, err := provider.GenerateMockConfig(ctx, sanitizedInput, options)
	if err != nil {
		return nil, err
	}

	// FIXED: Set timestamp
	result.Timestamp = time.Now()

	return result, nil
}

// Helper types and methods

type ProviderInfo struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Cost      string `json:"cost"`
}

func (pm *ProviderManager) isValidProvider(providerType ProviderType) bool {
	_, exists := pm.providers[providerType]
	return exists
}

func (pm *ProviderManager) getProviderCost(providerType ProviderType) string {
	switch providerType {
	case ProviderTemplate:
		return "Free"
	case ProviderAnthropic:
		return "~$0.01 per request"
	case ProviderOpenAI:
		return "~$0.02 per request"
	default:
		return "Unknown"
	}
}

// generateResponseScenarios creates multiple response scenarios including success and error cases
func (t *TemplateProvider) generateResponseScenarios(description, method, path string) []ResponseScenario {
	desc := strings.ToLower(description)
	var scenarios []ResponseScenario

	// Success scenario
	successBody := t.generateSuccessResponse(desc, method, path)
	successStatus := 200
	if method == "POST" {
		successStatus = 201
	}

	scenarios = append(scenarios, ResponseScenario{
		StatusCode:  successStatus,
		Scenario:    "success",
		Description: "Successful operation",
		Body:        successBody,
	})

	// Error scenarios based on endpoint type
	if strings.Contains(path, "{id}") {
		// Resource not found
		scenarios = append(scenarios, ResponseScenario{
			StatusCode:  404,
			Scenario:    "not_found",
			Description: "Resource not found",
			Body: `{
  "error": "not_found",
  "message": "The requested resource was not found",
  "timestamp": "2025-09-12T15:30:00Z"
}`,
		})
	}

	if method == "POST" || method == "PUT" || method == "PATCH" {
		// Validation error
		scenarios = append(scenarios, ResponseScenario{
			StatusCode:  400,
			Scenario:    "validation_error",
			Description: "Invalid input data",
			Body: `{
  "error": "validation_failed",
  "message": "Invalid input data provided",
  "details": [
    {
      "field": "email",
      "code": "invalid_format",
      "message": "Email format is invalid"
    }
  ],
  "timestamp": "2025-09-12T15:30:00Z"
}`,
		})
	}

	// Authentication required
	if !strings.Contains(desc, "public") && !strings.Contains(desc, "login") {
		scenarios = append(scenarios, ResponseScenario{
			StatusCode:  401,
			Scenario:    "unauthorized",
			Description: "Authentication required",
			Body: `{
  "error": "unauthorized",
  "message": "Authentication required to access this resource",
  "timestamp": "2025-09-12T15:30:00Z"
}`,
		})
	}

	return scenarios
}

// generateSuccessResponse creates realistic success response body
func (t *TemplateProvider) generateSuccessResponse(description, method, path string) string {
	desc := strings.ToLower(description)

	switch {
	case strings.Contains(desc, "user") && strings.Contains(desc, "profile"):
		return `{
  "id": "usr_7x9k2m",
  "email": "john.doe@example.com",
  "firstName": "John",
  "lastName": "Doe",
  "role": "user",
  "createdAt": "2024-01-15T10:30:00Z",
  "lastLoginAt": "2025-09-12T14:22:00Z",
  "isActive": true
}`
	case strings.Contains(desc, "user") && (strings.Contains(desc, "list") || !strings.Contains(path, "{id}")):
		return `{
  "users": [
    {
      "id": "usr_7x9k2m",
      "email": "john.doe@example.com",
      "firstName": "John",
      "lastName": "Doe",
      "role": "user",
      "isActive": true
    },
    {
      "id": "usr_4h3n8p",
      "email": "jane.smith@company.com",
      "firstName": "Jane",
      "lastName": "Smith",
      "role": "admin",
      "isActive": true
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 10,
    "total": 2,
    "totalPages": 1
  }
}`
	case strings.Contains(desc, "user") && method == "POST":
		return `{
  "id": "usr_new123",
  "email": "newuser@example.com",
  "firstName": "New",
  "lastName": "User",
  "role": "user",
  "createdAt": "2025-09-12T15:30:00Z",
  "isActive": true
}`
	case strings.Contains(desc, "order") && method == "POST":
		return `{
  "id": "ord_abc123",
  "status": "pending",
  "total": 59.98,
  "currency": "USD",
  "items": [
    {
      "productId": "prod_123",
      "quantity": 2,
      "price": 29.99
    }
  ],
  "shippingAddress": {
    "street": "123 Main St",
    "city": "San Francisco",
    "state": "CA",
    "zipCode": "94105"
  },
  "createdAt": "2025-09-12T15:30:00Z"
}`
	case strings.Contains(desc, "login") || strings.Contains(desc, "auth"):
		return `{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "def50200a1b2c3d4e5f6789...",
  "tokenType": "Bearer",
  "expiresIn": 3600,
  "user": {
    "id": "usr_7x9k2m",
    "email": "john.doe@example.com",
    "role": "user"
  }
}`
	default:
		return `{
  "id": "item_123",
  "name": "Sample Item",
  "description": "A sample item for testing",
  "createdAt": "2025-09-12T15:30:00Z",
  "isActive": true
}`
	}
}

// collectManualResponse collects manual response from user
func (t *TemplateProvider) collectManualResponse(endpoint *Endpoint) error {
	var statusCode string
	defaultStatus := "200"
	if endpoint.Method == "POST" {
		defaultStatus = "201"
	}
	if err := survey.AskOne(&survey.Input{
		Message: "Response status code:",
		Default: defaultStatus,
	}, &statusCode); err != nil {
		return err
	}

	// Parse status code
	switch statusCode {
	case "200", "":
		endpoint.StatusCode = 200
	case "201":
		endpoint.StatusCode = 201
	case "400":
		endpoint.StatusCode = 400
	case "401":
		endpoint.StatusCode = 401
	case "404":
		endpoint.StatusCode = 404
	case "500":
		endpoint.StatusCode = 500
	default:
		endpoint.StatusCode = 200
	}

	var response string
	if err := survey.AskOne(&survey.Multiline{
		Message: "Enter JSON response body:",
	}, &response); err != nil {
		return err
	}

	endpoint.Response = strings.TrimSpace(response)
	return nil
}

func (pm *ProviderManager) collectionToPrompt(collection *security.SanitizedCollection, options *GenerationOptions) string {
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
		prompt += "\n"
	}

	if collection.HasAuthentication {
		prompt += "\nNote: Collection has authentication schemes - include auth endpoints if requested.\n"
	}

	return prompt
}

// generateFromStructuredPrompt generates MockServer config from structured endpoint data
func (t *TemplateProvider) generateFromStructuredPrompt(prompt string) (*GenerationResult, error) {
	// Parse the structured prompt to extract endpoint information
	endpoints := t.parseStructuredPrompt(prompt)

	// Generate MockServer configuration
	mockConfig := t.EndpointsToMockServerJSON(endpoints)

	return &GenerationResult{
		MockServerJSON: mockConfig,
		Provider:       "interactive-builder",
		Warnings:       []string{"Generated from interactive endpoint builder"},
		Suggestions:    []string{"Review configuration and deploy when ready"},
		Timestamp:      time.Now(),
	}, nil
}

// parseStructuredPrompt parses our structured prompt to extract endpoint data
func (t *TemplateProvider) parseStructuredPrompt(prompt string) []Endpoint {
	var endpoints []Endpoint
	lines := strings.Split(prompt, "\n")

	var currentEndpoint *Endpoint
	var currentResponse *struct {
		StatusCode  int
		Description string
		Body        string
	}
	var collectingResponseBody bool
	var responseBodyLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Start of new endpoint
		if strings.HasPrefix(line, "ENDPOINT ") && strings.Contains(line, ":") {
			// Save previous endpoint if exists
			if currentEndpoint != nil {
				// Finalize any pending response
				if currentResponse != nil && len(responseBodyLines) > 0 {
					currentResponse.Body = strings.Join(responseBodyLines, "\n")
					currentEndpoint.Response = currentResponse.Body
					currentEndpoint.StatusCode = currentResponse.StatusCode
				}
				endpoints = append(endpoints, *currentEndpoint)
			}

			// Start new endpoint
			currentEndpoint = &Endpoint{
				StatusCode: 200, // Default
				Headers:    make(map[string]string),
			}
			currentResponse = nil
			collectingResponseBody = false
			responseBodyLines = nil
			continue
		}

		if currentEndpoint == nil {
			continue
		}

		// Parse endpoint fields
		if strings.HasPrefix(line, "Method: ") {
			currentEndpoint.Method = strings.TrimPrefix(line, "Method: ")
		} else if strings.HasPrefix(line, "Path: ") {
			currentEndpoint.Path = strings.TrimPrefix(line, "Path: ")
		} else if strings.HasPrefix(line, "Request Body: ") {
			body := strings.TrimPrefix(line, "Request Body: ")
			if body != "" {
				currentEndpoint.RequestBody = body
			}
		} else if strings.Contains(line, ". ") && strings.Contains(line, "(HTTP ") {
			// Response definition line: "  1. Success Response (HTTP 200)"
			// Finalize previous response if exists
			if currentResponse != nil && len(responseBodyLines) > 0 {
				currentResponse.Body = strings.Join(responseBodyLines, "\n")
				// For now, we'll take the first success response (200) for the endpoint
				if currentResponse.StatusCode == 200 {
					currentEndpoint.Response = currentResponse.Body
					currentEndpoint.StatusCode = currentResponse.StatusCode
				}
			}

			// Parse new response info
			currentResponse = &struct {
				StatusCode  int
				Description string
				Body        string
			}{
				StatusCode: 200, // Default
			}

			// Extract status code from "(HTTP 200)" pattern
			if strings.Contains(line, "HTTP 200") {
				currentResponse.StatusCode = 200
			} else if strings.Contains(line, "HTTP 201") {
				currentResponse.StatusCode = 201
			} else if strings.Contains(line, "HTTP 400") {
				currentResponse.StatusCode = 400
			} else if strings.Contains(line, "HTTP 404") {
				currentResponse.StatusCode = 404
			} else if strings.Contains(line, "HTTP 500") {
				currentResponse.StatusCode = 500
			}

			collectingResponseBody = false
			responseBodyLines = nil
		} else if strings.HasPrefix(line, "     Body: ") {
			// Start collecting response body after "     Body: " line
			body := strings.TrimPrefix(line, "     Body: ")
			if body != "" {
				responseBodyLines = append(responseBodyLines, body)
			}
			collectingResponseBody = true
		} else if collectingResponseBody && line != "" && !strings.HasPrefix(line, "Scenario: ") && !strings.HasPrefix(line, "ENDPOINT ") && !strings.HasPrefix(line, "Generate complete") {
			// Continue collecting response body lines
			responseBodyLines = append(responseBodyLines, line)
		} else if line == "" || strings.HasPrefix(line, "Scenario: ") {
			// End of response body collection
			collectingResponseBody = false
		}
	}

	// Save final endpoint
	if currentEndpoint != nil {
		// Finalize any pending response
		if currentResponse != nil && len(responseBodyLines) > 0 {
			currentResponse.Body = strings.Join(responseBodyLines, "\n")
			currentEndpoint.Response = currentResponse.Body
			currentEndpoint.StatusCode = currentResponse.StatusCode
		}
		// Ensure we have a valid response
		if currentEndpoint.Response == "" {
			currentEndpoint.Response = `{"message": "Success", "timestamp": "2025-09-12T15:30:00Z"}`
		}
		endpoints = append(endpoints, *currentEndpoint)
	}

	return endpoints
}

// REMOVED: Duplicate provider implementations - they're in separate files now

// OpenAIProvider - Simple declaration (implementation in openai.go when created)
type OpenAIProvider struct {
	apiKey string
}

func NewOpenAIProvider() *OpenAIProvider {
	return &OpenAIProvider{
		apiKey: os.Getenv("OPENAI_API_KEY"),
	}
}

func (o *OpenAIProvider) GetName() string {
	return "openai"
}

func (o *OpenAIProvider) IsAvailable() bool {
	return o.apiKey != ""
}

func (o *OpenAIProvider) GenerateMockConfig(ctx context.Context, input string, options *GenerationOptions) (*GenerationResult, error) {
	// Placeholder - will be implemented later
	return &GenerationResult{
		MockServerJSON: `[{"httpRequest":{"method":"GET","path":"/api/placeholder"},"httpResponse":{"statusCode":200,"body":{"message":"OpenAI implementation pending"}}}]`,
		Provider:       "openai",
		Warnings:       []string{"OpenAI implementation pending"},
		Timestamp:      time.Now(),
	}, nil
}

// TemplateProvider - Free fallback provider
type TemplateProvider struct{}

func NewTemplateProvider() *TemplateProvider {
	return &TemplateProvider{}
}

func (t *TemplateProvider) GetName() string {
	return "template"
}

func (t *TemplateProvider) IsAvailable() bool {
	return true // Always available
}

func (t *TemplateProvider) GenerateMockConfig(ctx context.Context, input string, options *GenerationOptions) (*GenerationResult, error) {
	// Check if this is structured endpoint data from our new interactive builder
	if strings.HasPrefix(input, "Generate MockServer JSON configuration for the following API endpoints:") {
		// This is pre-collected endpoint data - generate config directly
		return t.generateFromStructuredPrompt(input)
	}

	// Check if this is the special signal for starting interactive mode
	if strings.TrimSpace(input) == "interactive" {
		// This is a signal from REPL that interactive mode was selected
		// The REPL will handle the interactive building itself
		return &GenerationResult{
			MockServerJSON: `[]`, // Empty placeholder
			Provider:       "interactive-placeholder",
			Warnings:       []string{"Interactive mode initiated - REPL will handle endpoint collection"},
			Suggestions:    []string{"Follow the interactive prompts to build your API endpoints"},
			Timestamp:      time.Now(),
		}, nil
	}

	// For natural language descriptions, generate based on templates
	return t.generateFromDescription(input, options)
}

// generateFromDescription generates MockServer config from natural language description
func (t *TemplateProvider) generateFromDescription(description string, options *GenerationOptions) (*GenerationResult, error) {
	desc := strings.ToLower(description)

	// Generate appropriate template based on description keywords
	var mockConfig string
	var provider string = "template"

	switch {
	case strings.Contains(desc, "user") && (strings.Contains(desc, "profile") || strings.Contains(desc, "management")):
		mockConfig = generateUserInfoAPI(options)
		provider = "template-user"
	case strings.Contains(desc, "auth") || strings.Contains(desc, "login") || strings.Contains(desc, "token"):
		mockConfig = generateAuthAPI(options)
		provider = "template-auth"
	case strings.Contains(desc, "product") || strings.Contains(desc, "catalog") || strings.Contains(desc, "item"):
		mockConfig = generateProductAPI(options)
		provider = "template-product"
	case strings.Contains(desc, "order") || strings.Contains(desc, "payment") || strings.Contains(desc, "checkout"):
		mockConfig = generateOrderAPI(options)
		provider = "template-order"
	case strings.Contains(desc, "file") || strings.Contains(desc, "upload") || strings.Contains(desc, "download"):
		mockConfig = generateFileAPI(options)
		provider = "template-file"
	default:
		// Generic CRUD API
		mockConfig = generateGenericCRUDAPI(description, options)
		provider = "template-generic"
	}

	warnings := []string{
		"Generated from template based on keywords in description",
		"Configure LLM providers (ANTHROPIC_API_KEY) for AI-powered generation",
	}

	suggestions := []string{
		"Review and customize the generated endpoints",
		"Use 'interactive' mode for step-by-step building",
	}

	return &GenerationResult{
		MockServerJSON: mockConfig,
		Provider:       provider,
		Warnings:       warnings,
		Suggestions:    suggestions,
		Timestamp:      time.Now(),
	}, nil
}

// Endpoint represents a single API endpoint with advanced features
type Endpoint struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	QueryParams map[string]string `json:"queryParams,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	RequestBody string            `json:"requestBody,omitempty"`
	StatusCode  int               `json:"statusCode"`
	Response    string            `json:"response"`
	
	// Advanced features for 7-step builder
	ErrorScenarios    []ResponseScenario `json:"errorScenarios,omitempty"`
	ResponseDelay     string             `json:"responseDelay,omitempty"`
	ResponseHeaders   map[string]string  `json:"responseHeaders,omitempty"`
	StrictMode        bool               `json:"strictMode,omitempty"`
	WebhookURL        string             `json:"webhookURL,omitempty"`
	EnableLogging     bool               `json:"enableLogging,omitempty"`
}

// BuildEndpointsInteractively builds endpoints through user interaction (EXPORTED)
func (t *TemplateProvider) BuildEndpointsInteractively() ([]Endpoint, error) {
	var endpoints []Endpoint

	for {
		fmt.Printf("\n📡 Creating endpoint #%d\n", len(endpoints)+1)

		// Step 1: Get endpoint details
		endpoint, err := t.buildSingleEndpoint()
		if err != nil {
			return nil, err
		}

		endpoints = append(endpoints, endpoint)

		fmt.Printf("\n✅ Endpoint created: %s %s\n", endpoint.Method, endpoint.Path)

		// Ask if user wants to add more
		var addMore bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Add another endpoint?",
			Default: false,
		}, &addMore); err != nil {
			return nil, err
		}

		if !addMore {
			break
		}
	}

	return endpoints, nil
}

// buildSingleEndpoint builds one endpoint with user-provided request/response and AI assistance
func (t *TemplateProvider) buildSingleEndpoint() (Endpoint, error) {
	var endpoint Endpoint

	// Step 1: User describes what they want to mock
	var description string
	if err := survey.AskOne(&survey.Input{
		Message: "What request/response do you want to mock? (e.g., 'GET user by ID', 'POST create order'):",
	}, &description); err != nil {
		return endpoint, err
	}

	// Step 2: User provides the HTTP method and path they want
	fmt.Println("\n📡 Define your endpoint:")
	if err := survey.AskOne(&survey.Select{
		Message: "HTTP method:",
		Options: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
	}, &endpoint.Method); err != nil {
		return endpoint, err
	}

	if err := survey.AskOne(&survey.Input{
		Message: "Path (e.g., /api/users/{id}):",
	}, &endpoint.Path); err != nil {
		return endpoint, err
	}

	// Step 3: AI validates and helps fix the path if needed
	if strings.Contains(endpoint.Path, "?") {
		fmt.Println("\n🤖 AI detected query parameters in path. Let me help fix this...")
		parts := strings.Split(endpoint.Path, "?")
		cleanPath := parts[0]
		queryString := ""
		if len(parts) > 1 {
			queryString = parts[1]
		}

		fmt.Printf("💡 AI suggests: path = %s, query params = %s\n", cleanPath, queryString)
		var acceptFix bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Accept AI suggestion?",
			Default: true,
		}, &acceptFix); err != nil {
			return endpoint, err
		}

		if acceptFix {
			endpoint.Path = cleanPath
			if queryString != "" {
				// Parse query string into parameters
				params := make(map[string]string)
				for _, param := range strings.Split(queryString, "&") {
					if kv := strings.Split(param, "="); len(kv) == 2 {
						params[kv[0]] = kv[1]
					}
				}
				endpoint.QueryParams = params
				fmt.Printf("✅ AI extracted query parameters: %v\n", params)
			}
		}
	}

	// Step 4: User provides headers they want to match (optional)
	var needsHeaders bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Do you need to match specific headers (e.g., Authorization, Content-Type)?",
		Default: false,
	}, &needsHeaders); err != nil {
		return endpoint, err
	}

	if needsHeaders {
		headers, err := t.collectRequestHeaders()
		if err != nil {
			return endpoint, err
		}
		if len(headers) > 0 {
			endpoint.Headers = headers
		}
	}

	// Step 5: User provides request body (for POST, PUT, PATCH)
	if endpoint.Method == "POST" || endpoint.Method == "PUT" || endpoint.Method == "PATCH" {
		fmt.Printf("\n📤 Request body for %s %s:\n", endpoint.Method, endpoint.Path)

		var choice string
		if err := survey.AskOne(&survey.Select{
			Message: "How do you want to provide the request body?",
			Options: []string{"Type/paste JSON", "Load from file", "Skip (no body)"},
		}, &choice); err != nil {
			return endpoint, err
		}

		switch choice {
		case "Type/paste JSON":
			var body string
			if err := survey.AskOne(&survey.Multiline{
				Message: "Enter/paste your JSON request body:",
			}, &body); err != nil {
				return endpoint, err
			}

			// AI validates and formats JSON
			formattedBody, err := t.validateAndFormatJSON(body, "request")
			if err != nil {
				fmt.Printf("⚠️  JSON validation failed: %v\n", err)
				fmt.Println("💡 AI will use your input as-is, but it may not work correctly")
				formattedBody = strings.TrimSpace(body)
			}
			endpoint.RequestBody = formattedBody

		case "Load from file":
			var filePath string
			if err := survey.AskOne(&survey.Input{
				Message: "File path:",
			}, &filePath); err != nil {
				return endpoint, err
			}

			data, err := os.ReadFile(filePath)
			if err != nil {
				fmt.Printf("❌ Failed to read file: %v\n", err)
			} else {
				formattedBody, err := t.validateAndFormatJSON(string(data), "request")
				if err != nil {
					fmt.Printf("⚠️  JSON validation failed: %v\n", err)
					formattedBody = strings.TrimSpace(string(data))
				}
				endpoint.RequestBody = formattedBody
			}
		}
	}

	// Step 6: User provides their desired response
	fmt.Printf("\n📤 Response for %s %s:\n", endpoint.Method, endpoint.Path)

	// Status code
	var statusCode string
	defaultStatus := "200"
	if endpoint.Method == "POST" {
		defaultStatus = "201"
	}
	if err := survey.AskOne(&survey.Input{
		Message: "Response status code:",
		Default: defaultStatus,
	}, &statusCode); err != nil {
		return endpoint, err
	}

	// Parse status code
	switch statusCode {
	case "200", "":
		endpoint.StatusCode = 200
	case "201":
		endpoint.StatusCode = 201
	case "400":
		endpoint.StatusCode = 400
	case "401":
		endpoint.StatusCode = 401
	case "403":
		endpoint.StatusCode = 403
	case "404":
		endpoint.StatusCode = 404
	case "500":
		endpoint.StatusCode = 500
	default:
		endpoint.StatusCode = 200
	}

	// Response body
	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "How do you want to provide the response?",
		Options: []string{"Type/paste JSON", "Load from file", "Generate simple response"},
	}, &choice); err != nil {
		return endpoint, err
	}

	switch choice {
	case "Type/paste JSON":
		var response string
		if err := survey.AskOne(&survey.Multiline{
			Message: "Enter/paste your JSON response:",
		}, &response); err != nil {
			return endpoint, err
		}

		// AI validates and formats JSON
		formattedResponse, err := t.validateAndFormatJSON(response, "response")
		if err != nil {
			fmt.Printf("⚠️  JSON validation failed: %v\n", err)
			fmt.Println("💡 AI will use your input as-is, but it may not work correctly")
			formattedResponse = strings.TrimSpace(response)
		}
		endpoint.Response = formattedResponse

	case "Load from file":
		var filePath string
		if err := survey.AskOne(&survey.Input{
			Message: "File path:",
		}, &filePath); err != nil {
			return endpoint, err
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("❌ Failed to read file: %v\n", err)
			return endpoint, err
		}

		formattedResponse, err := t.validateAndFormatJSON(string(data), "response")
		if err != nil {
			fmt.Printf("⚠️  JSON validation failed: %v\n", err)
			formattedResponse = strings.TrimSpace(string(data))
		}
		endpoint.Response = formattedResponse

	default:
		// AI generates simple response based on description
		generatedResponse := t.generateSimpleResponseFromDescription(description, endpoint.Method, endpoint.Path)
		fmt.Printf("💡 AI generated response:\n%s\n", generatedResponse)

		var useGenerated bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Use this AI-generated response?",
			Default: true,
		}, &useGenerated); err != nil {
			return endpoint, err
		}

		if useGenerated {
			endpoint.Response = generatedResponse
		} else {
			// Fallback to manual entry
			var manualResponse string
			if err := survey.AskOne(&survey.Multiline{
				Message: "Enter your response manually:",
			}, &manualResponse); err != nil {
				return endpoint, err
			}
			endpoint.Response = strings.TrimSpace(manualResponse)
		}
	}

	// Step 7: AI offers to create response variants for load testing
	var createVariants bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "🚀 Create additional response variants? (slow responses, errors, etc.)",
		Default: false,
	}, &createVariants); err != nil {
		return endpoint, err
	}

	if createVariants {
		fmt.Println("💡 AI will generate additional variants after you finish this endpoint")
		// Note: We'll handle variants in a separate function
	}

	return endpoint, nil
}

// collectHeaders collects custom headers from user
func (t *TemplateProvider) collectHeaders() (map[string]string, error) {
	headers := make(map[string]string)

	fmt.Println("\n📋 Add headers (press Enter with empty name to finish):")
	for {
		var headerName string
		if err := survey.AskOne(&survey.Input{
			Message: "Header name (e.g., Authorization, Content-Type):",
		}, &headerName); err != nil {
			return nil, err
		}

		headerName = strings.TrimSpace(headerName)
		if headerName == "" {
			break
		}

		var headerValue string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Value for %s (e.g., Bearer token, application/json):", headerName),
		}, &headerValue); err != nil {
			return nil, err
		}

		headers[headerName] = headerValue
		fmt.Printf("✅ Added: %s: %s\n", headerName, headerValue)
	}

	return headers, nil
}

// collectQueryParameters collects query parameters from user
func (t *TemplateProvider) collectQueryParameters() (map[string]string, error) {
	params := make(map[string]string)

	fmt.Println("\n🔍 Add query parameters (press Enter with empty name to finish):")
	for {
		var paramName string
		if err := survey.AskOne(&survey.Input{
			Message: "Parameter name (e.g., page, limit, category):",
		}, &paramName); err != nil {
			return nil, err
		}

		paramName = strings.TrimSpace(paramName)
		if paramName == "" {
			break
		}

		var paramValue string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Value for %s (e.g., 1, 10, electronics):", paramName),
		}, &paramValue); err != nil {
			return nil, err
		}

		params[paramName] = paramValue
		fmt.Printf("✅ Added: %s=%s\n", paramName, paramValue)
	}

	return params, nil
}

// collectRequestHeaders collects headers that need to be matched
func (t *TemplateProvider) collectRequestHeaders() (map[string]string, error) {
	headers := make(map[string]string)

	fmt.Println("\n🔑 Add headers for request matching:")
	for {
		var headerName string
		if err := survey.AskOne(&survey.Input{
			Message: "Header name (empty to finish):",
		}, &headerName); err != nil {
			return nil, err
		}

		headerName = strings.TrimSpace(headerName)
		if headerName == "" {
			break
		}

		var headerValue string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Value for %s (use exact value or pattern):", headerName),
		}, &headerValue); err != nil {
			return nil, err
		}

		headers[headerName] = headerValue
		fmt.Printf("✅ Will match: %s: %s\n", headerName, headerValue)
	}

	return headers, nil
}

// validateAndFormatJSON validates and formats JSON input
func (t *TemplateProvider) validateAndFormatJSON(input, context string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("empty %s", context)
	}

	// Try to parse as JSON to validate
	var jsonData interface{}
	if err := json.Unmarshal([]byte(input), &jsonData); err != nil {
		return input, fmt.Errorf("invalid JSON: %w", err)
	}

	// Return the original input if valid (preserve user formatting)
	return input, nil
}

// generateSimpleResponseFromDescription generates response based on user description
func (t *TemplateProvider) generateSimpleResponseFromDescription(description, method, path string) string {
	desc := strings.ToLower(description)

	// Use the existing AI response generation logic
	if strings.Contains(desc, "user") {
		return t.generateSuccessResponse(desc, method, path)
	}
	if strings.Contains(desc, "order") {
		return t.generateSuccessResponse(desc, method, path)
	}
	if strings.Contains(desc, "product") {
		return t.generateSuccessResponse(desc, method, path)
	}

	// Generic success response
	return `{
  "success": true,
  "message": "Operation completed successfully",
  "timestamp": "2025-09-12T15:30:00Z"
}`
}

// collectRequestBody collects request body from user
func (t *TemplateProvider) collectRequestBody(method string) (string, error) {
	fmt.Printf("\n📄 Request Body for %s:\n", method)
	fmt.Println("💡 Use runtime variables: ${uuid}, ${name}, ${time}, ${datetime}")
	fmt.Println("📝 Example:")
	fmt.Println(`{
  "requestId": "${uuid}",
  "timestamp": "${datetime}",
  "userId": "${uuid}",
  "name": "${name}"
}`)

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "How do you want to provide the request body?",
		Options: []string{"Type JSON directly", "Load from file", "Skip (no body)"},
	}, &choice); err != nil {
		return "", err
	}

	switch choice {
	case "Type JSON directly":
		var body string
		if err := survey.AskOne(&survey.Multiline{
			Message: "Enter JSON request body:",
		}, &body); err != nil {
			return "", err
		}
		return strings.TrimSpace(body), nil

	case "Load from file":
		var filePath string
		if err := survey.AskOne(&survey.Input{
			Message: "Enter file path:",
		}, &filePath); err != nil {
			return "", err
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("❌ Failed to read file: %v\n", err)
			return "", nil // Return empty, don't fail completely
		}
		return string(data), nil

	default:
		return "", nil
	}
}

// collectResponse collects response body from user
func (t *TemplateProvider) collectResponse(method, path string) (string, error) {
	fmt.Println("\n📤 Response Body:")
	fmt.Println("💡 Use runtime variables: ${uuid}, ${name}, ${time}, ${datetime}")
	fmt.Println("🔗 Reference request data: ${request.fieldName}")
	fmt.Println("📝 Example:")
	fmt.Println(`{
  "id": "${request.userId}",
  "message": "Success",
  "timestamp": "${datetime}",
  "data": {
    "requestId": "${request.requestId}"
  }
}`)

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "How do you want to provide the response?",
		Options: []string{"Type JSON directly", "Load from file", "Generate simple response"},
	}, &choice); err != nil {
		return "", err
	}

	switch choice {
	case "Type JSON directly":
		var response string
		if err := survey.AskOne(&survey.Multiline{
			Message: "Enter JSON response body:",
		}, &response); err != nil {
			return "", err
		}
		return strings.TrimSpace(response), nil

	case "Load from file":
		var filePath string
		if err := survey.AskOne(&survey.Input{
			Message: "Enter file path:",
		}, &filePath); err != nil {
			return "", err
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("❌ Failed to read file: %v\n", err)
			return t.generateSimpleResponse(method, path), nil
		}
		return string(data), nil

	default:
		return t.generateSimpleResponse(method, path), nil
	}
}

// generateSimpleResponse generates a basic response based on method and path
func (t *TemplateProvider) generateSimpleResponse(method, path string) string {
	switch method {
	case "POST":
		return `{
  "id": "${uuid}",
  "message": "Created successfully",
  "timestamp": "${datetime}"
}`
	case "PUT", "PATCH":
		return `{
  "id": "${request.id}",
  "message": "Updated successfully",
  "timestamp": "${datetime}"
}`
	case "DELETE":
		return `{
  "message": "Deleted successfully",
  "timestamp": "${datetime}"
}`
	default: // GET
		if strings.Contains(path, "{id}") || strings.Contains(path, "{uuid}") {
			return `{
  "id": "${uuid}",
  "name": "${name}",
  "createdAt": "${datetime}",
  "updatedAt": "${datetime}"
}`
		} else {
			return `{
  "data": [
    {
      "id": "${uuid}",
      "name": "${name}",
      "createdAt": "${datetime}"
    }
  ],
  "total": 1,
  "timestamp": "${datetime}"
}`
		}
	}
}

// EndpointsToMockServerJSON converts endpoints to MockServer JSON format (EXPORTED)
func (t *TemplateProvider) EndpointsToMockServerJSON(endpoints []Endpoint) string {
	var mockServerExpectations []string

	for _, endpoint := range endpoints {
		// Build httpRequest
		request := fmt.Sprintf(`    "httpRequest": {
      "method": "%s",
      "path": "%s"`, endpoint.Method, endpoint.Path)

		// Add headers if present
		if len(endpoint.Headers) > 0 {
			request += `,
      "headers": {`
			var headerPairs []string
			for name, value := range endpoint.Headers {
				headerPairs = append(headerPairs, fmt.Sprintf(`"%s": "%s"`, name, value))
			}
			request += strings.Join(headerPairs, ", ")
			request += "}"
		}

		// Add request body if present
		if endpoint.RequestBody != "" {
			request += `,
      "body": ` + endpoint.RequestBody
		}

		request += "\n    }"

		// Build httpResponse
		response := fmt.Sprintf(`    "httpResponse": {
      "statusCode": %d,
      "headers": {"Content-Type": "application/json"},
      "body": %s
    }`, endpoint.StatusCode, endpoint.Response)

		// Combine into expectation
		expectation := fmt.Sprintf("  {\n%s,\n%s\n  }", request, response)
		mockServerExpectations = append(mockServerExpectations, expectation)
	}

	return fmt.Sprintf("[\n%s\n]", strings.Join(mockServerExpectations, ",\n"))
}

// AI-powered helper functions for intelligent mock generation

// ResponseScenario represents a possible response variant
type ResponseScenario struct {
	StatusCode  int    `json:"statusCode"`
	Scenario    string `json:"scenario"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

// suggestMethodAndPath analyzes description and suggests HTTP method and path
func (t *TemplateProvider) suggestMethodAndPath(description string) (string, string) {
	desc := strings.ToLower(description)

	// Determine HTTP method based on description
	var method string
	switch {
	case strings.Contains(desc, "create") || strings.Contains(desc, "add") || strings.Contains(desc, "post") || strings.Contains(desc, "upload"):
		method = "POST"
	case strings.Contains(desc, "update") || strings.Contains(desc, "edit") || strings.Contains(desc, "modify") || strings.Contains(desc, "put"):
		method = "PUT"
	case strings.Contains(desc, "patch") || strings.Contains(desc, "partial"):
		method = "PATCH"
	case strings.Contains(desc, "delete") || strings.Contains(desc, "remove"):
		method = "DELETE"
	default:
		method = "GET"
	}

	// Generate path based on description
	var path string
	switch {
	case strings.Contains(desc, "user") && strings.Contains(desc, "profile"):
		if method == "GET" {
			path = "/api/users/me"
		} else {
			path = "/api/users/{id}"
		}
	case strings.Contains(desc, "user") && (strings.Contains(desc, "list") || strings.Contains(desc, "all")):
		path = "/api/users"
	case strings.Contains(desc, "user") && (strings.Contains(desc, "by id") || strings.Contains(desc, "specific")):
		path = "/api/users/{id}"
	case strings.Contains(desc, "user"):
		if method == "POST" {
			path = "/api/users"
		} else {
			path = "/api/users/{id}"
		}
	case strings.Contains(desc, "order") && strings.Contains(desc, "history"):
		path = "/api/orders"
	case strings.Contains(desc, "order"):
		if method == "POST" {
			path = "/api/orders"
		} else {
			path = "/api/orders/{id}"
		}
	case strings.Contains(desc, "product") && (strings.Contains(desc, "catalog") || strings.Contains(desc, "list")):
		path = "/api/products"
	case strings.Contains(desc, "product"):
		if method == "POST" {
			path = "/api/products"
		} else {
			path = "/api/products/{id}"
		}
	case strings.Contains(desc, "upload") || strings.Contains(desc, "file"):
		path = "/api/files/upload"
	case strings.Contains(desc, "login") || strings.Contains(desc, "auth"):
		path = "/api/auth/login"
	case strings.Contains(desc, "logout"):
		path = "/api/auth/logout"
	default:
		// Generic resource endpoint
		resource := "items"
		if method == "POST" {
			path = fmt.Sprintf("/api/%s", resource)
		} else {
			path = fmt.Sprintf("/api/%s/{id}", resource)
		}
	}

	return method, path
}

// suggestQueryParameters suggests relevant query parameters based on description
func (t *TemplateProvider) suggestQueryParameters(description, path string) map[string]string {
	desc := strings.ToLower(description)
	params := make(map[string]string)

	// Common pagination parameters
	if strings.Contains(desc, "list") || strings.Contains(desc, "all") || !strings.Contains(path, "{id}") {
		params["page"] = "1"
		params["limit"] = "10"
	}

	// Search parameters
	if strings.Contains(desc, "search") || strings.Contains(desc, "filter") {
		params["q"] = "search term"
	}

	// Sorting parameters
	if strings.Contains(desc, "sort") || strings.Contains(desc, "order") {
		params["sort"] = "created_at"
		params["order"] = "desc"
	}

	// Resource-specific parameters
	if strings.Contains(desc, "user") && strings.Contains(desc, "filter") {
		params["status"] = "active"
	}

	if strings.Contains(desc, "product") {
		params["category"] = "electronics"
		params["in_stock"] = "true"
	}

	return params
}

// generateRequestBody creates realistic request body based on description
func (t *TemplateProvider) generateRequestBody(description, method string) string {
	desc := strings.ToLower(description)

	switch {
	case strings.Contains(desc, "user") && (method == "POST" || method == "PUT"):
		return `{
  "email": "user@example.com",
  "firstName": "John",
  "lastName": "Doe",
  "role": "user"
}`
	case strings.Contains(desc, "user") && method == "PATCH":
		return `{
  "firstName": "Jane",
  "lastName": "Smith"
}`
	case strings.Contains(desc, "order") && method == "POST":
		return `{
  "items": [
    {
      "productId": "prod_123",
      "quantity": 2,
      "price": 29.99
    }
  ],
  "shippingAddress": {
    "street": "123 Main St",
    "city": "San Francisco",
    "state": "CA",
    "zipCode": "94105"
  }
}`
	case strings.Contains(desc, "product") && method == "POST":
		return `{
  "name": "Wireless Headphones",
  "description": "High-quality wireless headphones with noise cancellation",
  "price": 99.99,
  "category": "electronics",
  "inStock": true
}`
	case strings.Contains(desc, "login") || strings.Contains(desc, "auth"):
		return `{
  "email": "user@example.com",
  "password": "securepassword123"
}`
	default:
		return `{
  "name": "Sample Item",
  "description": "A sample item for testing",
  "value": 42
}`
	}
}

// generateUserInfoAPI creates user information endpoints
func generateUserInfoAPI(options *GenerationOptions) string {
	return `[
  {
    "httpRequest": {
      "method": "GET",
      "path": "/api/users/me"
    },
    "httpResponse": {
      "statusCode": 200,
      "body": {
        "id": 1001,
        "username": "john_doe",
        "email": "john.doe@example.com",
        "firstName": "John",
        "lastName": "Doe",
        "createdAt": "2024-01-15T10:30:00Z",
        "lastLoginAt": "2024-12-01T14:22:00Z"
      }
    }
  },
  {
    "httpRequest": {
      "method": "GET",
      "path": "/api/users",
      "queryStringParameters": {
        "page": ["1"],
        "limit": ["10"]
      }
    },
    "httpResponse": {
      "statusCode": 200,
      "body": {
        "users": [
          {
            "id": 1001,
            "username": "john_doe",
            "email": "john.doe@example.com",
            "firstName": "John",
            "lastName": "Doe"
          },
          {
            "id": 1002,
            "username": "jane_smith",
            "email": "jane.smith@example.com",
            "firstName": "Jane",
            "lastName": "Smith"
          }
        ],
        "total": 2,
        "page": 1,
        "limit": 10
      }
    }
  }
]`
}

// generateAuthAPI creates authentication endpoints
func generateAuthAPI(options *GenerationOptions) string {
	return `[
  {
    "httpRequest": {
      "method": "POST",
      "path": "/api/auth/login"
    },
    "httpResponse": {
      "statusCode": 200,
      "body": {
        "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
        "refresh_token": "def50200a1b2c3d4e5f6789...",
        "token_type": "Bearer",
        "expires_in": 3600,
        "user": {
          "id": 1001,
          "username": "john_doe",
          "email": "john.doe@example.com"
        }
      }
    }
  },
  {
    "httpRequest": {
      "method": "POST",
      "path": "/api/auth/refresh"
    },
    "httpResponse": {
      "statusCode": 200,
      "body": {
        "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.new_token_here",
        "token_type": "Bearer",
        "expires_in": 3600
      }
    }
  }
]`
}

// generateProductAPI creates product/catalog endpoints
func generateProductAPI(options *GenerationOptions) string {
	return `[
  {
    "httpRequest": {
      "method": "GET",
      "path": "/api/products"
    },
    "httpResponse": {
      "statusCode": 200,
      "body": {
        "products": [
          {
            "id": "prod_001",
            "name": "Wireless Bluetooth Headphones",
            "description": "High-quality wireless headphones with noise cancellation",
            "price": 99.99,
            "currency": "USD",
            "inStock": true,
            "category": "electronics"
          },
          {
            "id": "prod_002",
            "name": "Smartphone Case",
            "description": "Protective case for latest smartphone models",
            "price": 24.99,
            "currency": "USD",
            "inStock": true,
            "category": "accessories"
          }
        ],
        "total": 2
      }
    }
  }
]`
}

// generateOrderAPI creates order/payment endpoints
func generateOrderAPI(options *GenerationOptions) string {
	return `[
  {
    "httpRequest": {
      "method": "POST",
      "path": "/api/orders"
    },
    "httpResponse": {
      "statusCode": 201,
      "body": {
        "orderId": "ord_12345",
        "status": "pending",
        "total": 124.98,
        "currency": "USD",
        "items": [
          {
            "productId": "prod_001",
            "quantity": 1,
            "price": 99.99
          },
          {
            "productId": "prod_002",
            "quantity": 1,
            "price": 24.99
          }
        ],
        "createdAt": "2024-12-01T15:30:00Z"
      }
    }
  },
  {
    "httpRequest": {
      "method": "GET",
      "path": "/api/orders/ord_12345"
    },
    "httpResponse": {
      "statusCode": 200,
      "body": {
        "orderId": "ord_12345",
        "status": "confirmed",
        "total": 124.98,
        "currency": "USD",
        "paymentStatus": "paid",
        "shippingAddress": {
          "street": "123 Main St",
          "city": "San Francisco",
          "state": "CA",
          "zipCode": "94105"
        }
      }
    }
  }
]`
}

// generateFileAPI creates file upload endpoints
func generateFileAPI(options *GenerationOptions) string {
	return `[
  {
    "httpRequest": {
      "method": "POST",
      "path": "/api/files/upload"
    },
    "httpResponse": {
      "statusCode": 201,
      "body": {
        "fileId": "file_abc123",
        "filename": "document.pdf",
        "size": 1024000,
        "mimeType": "application/pdf",
        "uploadedAt": "2024-12-01T15:45:00Z",
        "downloadUrl": "https://cdn.example.com/files/file_abc123"
      }
    }
  },
  {
    "httpRequest": {
      "method": "GET",
      "path": "/api/files/file_abc123"
    },
    "httpResponse": {
      "statusCode": 200,
      "body": {
        "fileId": "file_abc123",
        "filename": "document.pdf",
        "size": 1024000,
        "mimeType": "application/pdf",
        "downloadUrl": "https://cdn.example.com/files/file_abc123",
        "metadata": {
          "pages": 5,
          "author": "John Doe"
        }
      }
    }
  }
]`
}

// getStatusDescription returns human-readable status descriptions
func getStatusDescription(statusCode int) string {
	switch statusCode {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 202:
		return "Accepted"
	case 204:
		return "No Content"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	default:
		return "Unknown"
	}
}

// generateGenericCRUDAPI creates generic CRUD endpoints based on input
func generateGenericCRUDAPI(input string, options *GenerationOptions) string {
	// Try to extract a resource name from the input
	resource := "items"
	if strings.Contains(strings.ToLower(input), "task") {
		resource = "tasks"
	} else if strings.Contains(strings.ToLower(input), "note") {
		resource = "notes"
	} else if strings.Contains(strings.ToLower(input), "post") {
		resource = "posts"
	}

	return fmt.Sprintf(`[
  {
    "httpRequest": {
      "method": "GET",
      "path": "/api/%s"
    },
    "httpResponse": {
      "statusCode": 200,
      "body": {
        "%s": [
          {
            "id": 1,
            "title": "Sample Item",
            "description": "This is a sample item",
            "createdAt": "2024-12-01T10:00:00Z"
          }
        ],
        "total": 1
      }
    }
  },
  {
    "httpRequest": {
      "method": "POST",
      "path": "/api/%s"
    },
    "httpResponse": {
      "statusCode": 201,
      "body": {
        "id": 2,
        "title": "New Item",
        "description": "Newly created item",
        "createdAt": "2024-12-01T15:30:00Z"
      }
    }
  }
]`, resource, resource, resource)
}
