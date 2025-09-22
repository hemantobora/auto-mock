package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// AnthropicProvider implements MockServer JSON generation using Claude
type AnthropicProvider struct {
	apiKey      string
	baseURL     string
	model       string
	client      *http.Client
	maxTokens   int
	temperature float64
}

// Anthropic API request/response types
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Usage   *anthropicUsage    `json:"usage,omitempty"`
	Error   *anthropicError    `json:"error,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider() *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:      os.Getenv("ANTHROPIC_API_KEY"),
		baseURL:     getEnvWithDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com/v1/messages"),
		model:       getEnvWithDefault("ANTHROPIC_MODEL", "claude-3-5-sonnet-20241022"),
		maxTokens:   getEnvIntWithDefault("ANTHROPIC_MAX_TOKENS", 4000),
		temperature: getEnvFloatWithDefault("ANTHROPIC_TEMPERATURE", 0.1), // Lower for precise JSON
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (a *AnthropicProvider) GetName() string {
	return "anthropic"
}

func (a *AnthropicProvider) IsAvailable() bool {
	return a.apiKey != ""
}

// GenerateMockConfig generates MockServer JSON using Claude with structured specifications
func (a *AnthropicProvider) GenerateMockConfig(ctx context.Context, input string, options *GenerationOptions) (*GenerationResult, error) {
	// Build the focused system prompt for MockServer JSON generation
	systemPrompt := a.buildMockServerSystemPrompt()
	
	// The input is now a structured specification from the REPL
	userPrompt := input

	// Create API request
	apiRequest := anthropicRequest{
		Model:     a.model,
		MaxTokens: a.maxTokens,
		Messages: []anthropicMessage{
			{
				Role:    "user", 
				Content: systemPrompt + "\n\n" + userPrompt,
			},
		},
	}

	// Make API call
	startTime := time.Now()
	response, err := a.makeAPICall(ctx, apiRequest)
	if err != nil {
		return nil, fmt.Errorf("anthropic API call failed: %w", err)
	}

	if len(response.Content) == 0 {
		return nil, fmt.Errorf("empty response from Anthropic")
	}

	// Extract and validate MockServer JSON
	mockServerJSON := a.extractJSON(response.Content[0].Text)
	if err := a.validateMockServerJSON(mockServerJSON); err != nil {
		return nil, fmt.Errorf("invalid MockServer JSON generated: %w", err)
	}

	// Calculate tokens used
	tokensUsed := 0
	if response.Usage != nil {
		tokensUsed = response.Usage.InputTokens + response.Usage.OutputTokens
	}

	// Generate quality feedback
	warnings, suggestions := a.analyzeResult(mockServerJSON)

	return &GenerationResult{
		MockServerJSON: mockServerJSON,
		Provider:       "anthropic",
		TokensUsed:     tokensUsed,
		Warnings:       warnings,
		Suggestions:    suggestions,
		Timestamp:      startTime,
	}, nil
}

// buildMockServerSystemPrompt creates a focused system prompt for MockServer JSON
func (a *AnthropicProvider) buildMockServerSystemPrompt() string {
	return `You are a MockServer JSON configuration generator. Your ONLY job is to convert structured specifications into valid MockServer expectation arrays.

CRITICAL REQUIREMENTS:
1. Return ONLY valid MockServer JSON array - no explanations, no markdown, no extra text
2. Follow MockServer syntax exactly: https://www.mock-server.com/mock_server/getting_started.html
3. Use ONLY the specifications provided - do not add assumptions
4. Generate proper request matchers based on user's explicit choices

MockServer JSON Format:
[
  {
    "httpRequest": {
      "method": "GET|POST|PUT|DELETE|PATCH",
      "path": "/exact/path",
      "queryStringParameters": {"param": ["value"]},
      "headers": {"header": "pattern"},
      "body": {...}
    },
    "httpResponse": {
      "statusCode": 200,
      "headers": {
        "Content-Type": "application/json",
        "Access-Control-Allow-Origin": "*",
        "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
        "Access-Control-Allow-Headers": "Content-Type, Authorization"
      },
      "body": {...}
    }
  }
]

Request Matching Rules:
- If specification says "Query Parameters (REQUIRED for matching)" → Use queryStringParameters matcher
- If specification says "Query Parameters (documentation only)" → Do NOT include in httpRequest
- If specification says "Required Headers for matching" → Use headers matcher  
- If specification says "Path Matching: Exact" → Use exact path string
- If specification says "Path Matching: Support path parameters" → Use path with parameters
- If specification says "Request Body (REQUIRED for matching)" → Include body matcher
- If specification says "Request Body (documentation only)" → Do NOT include in httpRequest

Response Rules:
- Use EXACT status codes and response bodies from specification
- Always include CORS headers in httpResponse
- Add any custom response headers specified
- Preserve JSON structure exactly as provided

NEVER:
- Add query parameters not marked as "REQUIRED for matching"
- Generate error responses unless explicitly specified
- Make assumptions about authentication
- Add extra endpoints beyond specifications
- Modify the provided response bodies`
}

// makeAPICall makes the HTTP request to Anthropic API
func (a *AnthropicProvider) makeAPICall(ctx context.Context, req anthropicRequest) (*anthropicResponse, error) {
	// Marshal request
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("User-Agent", "AutoMock/1.0")

	// Make request
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResponse anthropicResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API errors
	if apiResponse.Error != nil {
		return nil, fmt.Errorf("API error: %s - %s", apiResponse.Error.Type, apiResponse.Error.Message)
	}

	return &apiResponse, nil
}

// extractJSON extracts and cleans JSON from Claude's response
func (a *AnthropicProvider) extractJSON(text string) string {
	// Remove any markdown formatting
	text = strings.TrimSpace(text)

	// Remove ```json and ``` if present
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
	}
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
	}
	if strings.HasSuffix(text, "```") {
		text = strings.TrimSuffix(text, "```")
	}

	// Find JSON array boundaries
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")

	if start >= 0 && end > start {
		text = text[start : end+1]
	}

	return strings.TrimSpace(text)
}

// validateMockServerJSON validates the generated JSON against MockServer requirements
func (a *AnthropicProvider) validateMockServerJSON(jsonStr string) error {
	if jsonStr == "" {
		return fmt.Errorf("empty JSON generated")
	}

	// Parse as JSON array
	var expectations []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &expectations); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}

	if len(expectations) == 0 {
		return fmt.Errorf("no expectations generated")
	}

	// Validate each expectation has required MockServer structure
	for i, exp := range expectations {
		// Check for httpRequest
		httpReq, hasRequest := exp["httpRequest"]
		if !hasRequest {
			return fmt.Errorf("expectation %d missing httpRequest", i)
		}

		// Check for httpResponse
		httpResp, hasResponse := exp["httpResponse"]
		if !hasResponse {
			return fmt.Errorf("expectation %d missing httpResponse", i)
		}

		// Validate httpRequest structure
		if reqMap, ok := httpReq.(map[string]interface{}); ok {
			if _, hasMethod := reqMap["method"]; !hasMethod {
				return fmt.Errorf("expectation %d httpRequest missing method", i)
			}
			if _, hasPath := reqMap["path"]; !hasPath {
				return fmt.Errorf("expectation %d httpRequest missing path", i)
			}
		} else {
			return fmt.Errorf("expectation %d httpRequest is not an object", i)
		}

		// Validate httpResponse structure
		if respMap, ok := httpResp.(map[string]interface{}); ok {
			if _, hasStatus := respMap["statusCode"]; !hasStatus {
				return fmt.Errorf("expectation %d httpResponse missing statusCode", i)
			}
		} else {
			return fmt.Errorf("expectation %d httpResponse is not an object", i)
		}
	}

	return nil
}

// analyzeResult provides quality feedback on the generated MockServer JSON
func (a *AnthropicProvider) analyzeResult(jsonStr string) ([]string, []string) {
	var warnings []string
	var suggestions []string

	// Parse the JSON to analyze
	var expectations []map[string]interface{}
	if json.Unmarshal([]byte(jsonStr), &expectations) != nil {
		warnings = append(warnings, "Generated JSON may have parsing issues")
		return warnings, suggestions
	}

	// Check expectation count
	expectationCount := len(expectations)
	if expectationCount == 1 {
		suggestions = append(suggestions, "Consider adding more endpoints for comprehensive API coverage")
	}

	// Analyze each expectation for quality
	hasGetEndpoint := false
	hasPostEndpoint := false
	hasCorsHeaders := false
	hasQueryMatchers := false

	for _, exp := range expectations {
		// Check HTTP methods diversity
		if httpReq, ok := exp["httpRequest"].(map[string]interface{}); ok {
			if method, ok := httpReq["method"].(string); ok {
				switch method {
				case "GET":
					hasGetEndpoint = true
				case "POST":
					hasPostEndpoint = true
				}
			}

			// Check for query parameter matchers
			if _, hasQuery := httpReq["queryStringParameters"]; hasQuery {
				hasQueryMatchers = true
			}
		}

		// Check for CORS headers
		if httpResp, ok := exp["httpResponse"].(map[string]interface{}); ok {
			if headers, ok := httpResp["headers"].(map[string]interface{}); ok {
				if _, hasCors := headers["Access-Control-Allow-Origin"]; hasCors {
					hasCorsHeaders = true
				}
			}
		}
	}

	// Generate suggestions based on analysis
	if expectationCount > 1 && !hasGetEndpoint {
		suggestions = append(suggestions, "Consider adding GET endpoints for data retrieval")
	}

	if expectationCount > 1 && !hasPostEndpoint {
		suggestions = append(suggestions, "Consider adding POST endpoints for data creation")
	}

	if !hasCorsHeaders {
		warnings = append(warnings, "Some expectations may be missing CORS headers")
	}

	if hasQueryMatchers {
		suggestions = append(suggestions, "Great! Using query parameter matchers for precise request matching")
	}

	// General quality suggestions
	if expectationCount >= 3 {
		suggestions = append(suggestions, "Good API coverage with multiple endpoints")
	}

	return warnings, suggestions
}

// Helper functions
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		// Simple int parsing without external dependencies
		if len(value) > 0 {
			// Basic validation - just return default if not a simple number
			for _, char := range value {
				if char < '0' || char > '9' {
					return defaultValue
				}
			}
			// Convert manually to avoid additional imports
			result := 0
			for _, char := range value {
				result = result*10 + int(char-'0')
			}
			return result
		}
	}
	return defaultValue
}

func getEnvFloatWithDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		// Simple validation - if it looks like a float, use it
		if strings.Contains(value, ".") && len(value) > 2 {
			// For simplicity, just return default if not a simple decimal
			return defaultValue
		}
	}
	return defaultValue
}