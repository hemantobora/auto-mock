// Package models provides shared data structures used across the auto-mock application
package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// ConfigMetadata contains versioning and tracking information
type ConfigMetadata struct {
	ProjectID   string    `json:"project_id"`
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Description string    `json:"description,omitempty"`
	Provider    string    `json:"provider,omitempty"` // AI provider used (anthropic, openai, template)
	Size        int64     `json:"size,omitempty"`     // Size in bytes
}

// MockConfiguration represents a complete MockServer configuration
type MockConfiguration struct {
	Metadata     ConfigMetadata    `json:"metadata"`
	Expectations []MockExpectation `json:"expectations"`
	Settings     ConfigSettings    `json:"settings,omitempty"`
}

// ConfigSettings contains additional configuration options
type ConfigSettings struct {
	Source       string            `json:"source,omitempty"`        // ai-generated, collection-import, manual
	ImportMethod string            `json:"import_method,omitempty"` // describe, interactive, template, collection
	Tags         []string          `json:"tags,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type MockExpectation struct {
	ID           string                 `json:"id,omitempty"`
	Priority     int                    `json:"priority,omitempty"`
	HttpRequest  map[string]interface{} `json:"httpRequest"`
	HttpResponse map[string]interface{} `json:"httpResponse,omitempty"`
	HttpError    map[string]interface{} `json:"httpError,omitempty"`   // For connection errors
	HttpForward  map[string]interface{} `json:"httpForward,omitempty"` // For forwarding
	Times        *ExpectationTimes      `json:"times,omitempty"`
	TimeToLive   *TimeToLive            `json:"timeToLive,omitempty"`
}

// ExpectationTimes controls how many times an expectation should match
type ExpectationTimes struct {
	RemainingTimes int  `json:"remainingTimes,omitempty"`
	Unlimited      bool `json:"unlimited,omitempty"`
}

// TimeToLive controls how long an expectation is active
type TimeToLive struct {
	TimeUnit   string `json:"timeUnit"`   // SECONDS, MINUTES, HOURS, DAYS
	TimeToLive int    `json:"timeToLive"` // Must be integer
}

// Delay structure for MockServer
type Delay struct {
	TimeUnit string `json:"timeUnit"` // MILLISECONDS, SECONDS, MINUTES
	Value    int    `json:"value"`    // Must be integer, not string
}

// VersionInfo represents metadata about a configuration version
type VersionInfo struct {
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description,omitempty"`
	Size        int64     `json:"size,omitempty"`
}

// ProjectInfo represents metadata about a project
type ProjectInfo struct {
	ProjectID        string    `json:"project_id"`
	DisplayName      string    `json:"display_name"`
	StorageName      string    `json:"storage_name"` // Cloud-specific storage identifier
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Provider         string    `json:"provider"` // aws, gcp, azure
	HasExpectations  bool      `json:"has_expectations"`
	ExpectationCount int       `json:"expectation_count"`
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

// ValidateConfiguration validates a mock configuration
func ValidateConfiguration(config *MockConfiguration) error {
	if config == nil {
		return ValidationError{Field: "config", Message: "configuration cannot be nil"}
	}

	if config.Metadata.ProjectID == "" {
		return ValidationError{Field: "metadata.project_id", Message: "project ID is required"}
	}

	if len(config.Expectations) == 0 {
		return ValidationError{Field: "expectations", Message: "at least one expectation is required"}
	}

	for i, exp := range config.Expectations {
		if exp.HttpRequest == nil {
			return ValidationError{
				Field:   fmt.Sprintf("expectations[%d].httpRequest", i),
				Message: "HTTP request is required",
			}
		}

		if exp.HttpResponse == nil {
			return ValidationError{
				Field:   fmt.Sprintf("expectations[%d].httpResponse", i),
				Message: "HTTP response is required",
			}
		}

		// Validate request has method and path
		if method, ok := exp.HttpRequest["method"]; !ok || method == "" {
			return ValidationError{
				Field:   fmt.Sprintf("expectations[%d].httpRequest.method", i),
				Message: "HTTP method is required",
			}
		}

		if path, ok := exp.HttpRequest["path"]; !ok || path == "" {
			return ValidationError{
				Field:   fmt.Sprintf("expectations[%d].httpRequest.path", i),
				Message: "HTTP path is required",
			}
		}

		// Validate response has status code
		if statusCode, ok := exp.HttpResponse["statusCode"]; !ok || statusCode == nil {
			return ValidationError{
				Field:   fmt.Sprintf("expectations[%d].httpResponse.statusCode", i),
				Message: "HTTP status code is required",
			}
		}

		if err := ValidateMockServerExpectation(&exp); err != nil {
			return ValidationError{
				Field:   fmt.Sprintf("expectations[%d]", i),
				Message: err.Error(),
			}
		}
	}

	return nil
}

// ParseMockServerJSON converts raw MockServer JSON to our configuration format
func ParseMockServerJSON(jsonData string) (*MockConfiguration, error) {
	var expectations []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &expectations); err != nil {
		return nil, fmt.Errorf("failed to parse MockServer JSON: %w", err)
	}

	config := &MockConfiguration{
		Metadata: ConfigMetadata{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Version:   "1.0",
		},
		Expectations: make([]MockExpectation, 0, len(expectations)),
	}

	for i, exp := range expectations {
		mockExp := MockExpectation{
			ID:       fmt.Sprintf("exp_%d_%d", time.Now().Unix(), i),
			Priority: i + 1, // Higher priority for earlier expectations
		}

		if httpReq, ok := exp["httpRequest"].(map[string]interface{}); ok {
			mockExp.HttpRequest = httpReq
		}

		if httpResp, ok := exp["httpResponse"].(map[string]interface{}); ok {
			mockExp.HttpResponse = httpResp
		}

		// Check for times configuration
		if times, ok := exp["times"].(map[string]interface{}); ok {
			mockExp.Times = &ExpectationTimes{}
			if remaining, ok := times["remainingTimes"].(float64); ok {
				mockExp.Times.RemainingTimes = int(remaining)
			}
			if unlimited, ok := times["unlimited"].(bool); ok {
				mockExp.Times.Unlimited = unlimited
			}
		}

		config.Expectations = append(config.Expectations, mockExp)
	}

	return config, nil
}

// ValidateMockServerExpectation validates an expectation against MockServer schema
func ValidateMockServerExpectation(exp *MockExpectation) error {
	// Rule 1: httpRequest.name is NOT allowed
	if exp.HttpRequest != nil {
		if _, hasName := exp.HttpRequest["name"]; hasName {
			return fmt.Errorf("httpRequest.name is not allowed in MockServer schema - use it for display only, not in the actual expectation")
		}
	}

	// Rule 2: delay.value must be integer
	if exp.HttpResponse != nil {
		if delay, hasDelay := exp.HttpResponse["delay"]; hasDelay {
			delayMap, ok := delay.(map[string]interface{})
			if ok {
				if value, hasValue := delayMap["value"]; hasValue {
					// Check if it's a string (invalid) or float (needs to be int)
					switch v := value.(type) {
					case string:
						return fmt.Errorf("delay.value must be integer, got string: %s", v)
					case float64:
						// Convert to int
						delayMap["value"] = int(v)
					case int:
						// Already correct
					default:
						return fmt.Errorf("delay.value must be integer, got: %T", v)
					}
				}
			}
		}
	}

	// Rule 3: Headers must be arrays (even single values)
	if exp.HttpResponse != nil {
		if headers, hasHeaders := exp.HttpResponse["headers"]; hasHeaders {
			headerMap, ok := headers.(map[string]interface{})
			if ok {
				for key, value := range headerMap {
					// Ensure all header values are arrays
					switch v := value.(type) {
					case string:
						// Convert string to array
						headerMap[key] = []string{v}
					case []interface{}:
						// Already array, ensure all elements are strings
						strArr := make([]string, len(v))
						for i, elem := range v {
							if s, ok := elem.(string); ok {
								strArr[i] = s
							} else {
								return fmt.Errorf("header %s contains non-string value: %v", key, elem)
							}
						}
						headerMap[key] = strArr
					case []string:
						// Already correct
					default:
						return fmt.Errorf("header %s has invalid type: %T", key, v)
					}
				}
			}
		}

		// Rule 3b: Request headers also must be arrays or regex objects
		if headers, hasHeaders := exp.HttpRequest["headers"]; hasHeaders {
			headerMap, ok := headers.(map[string]interface{})
			if ok {
				for key, value := range headerMap {
					switch v := value.(type) {
					case string:
						// Convert to array
						headerMap[key] = []string{v}
					case []interface{}:
						// Check if it's a regex pattern
						if len(v) == 1 {
							if regexMap, ok := v[0].(map[string]interface{}); ok {
								if _, hasRegex := regexMap["regex"]; hasRegex {
									// Valid regex pattern, keep as-is
									continue
								}
							}
						}
						// Otherwise ensure all are strings
						strArr := make([]string, len(v))
						for i, elem := range v {
							if s, ok := elem.(string); ok {
								strArr[i] = s
							}
						}
						headerMap[key] = strArr
					case []string:
						// Already correct
					case map[string]interface{}:
						// Could be {"regex": "..."} format
						if _, hasRegex := v["regex"]; hasRegex {
							headerMap[key] = []map[string]interface{}{v}
						}
					}
				}
			}
		}
	}

	// Rule 4: Query string parameters must be arrays
	if exp.HttpRequest != nil {
		if params, hasParams := exp.HttpRequest["queryStringParameters"]; hasParams {
			paramMap, ok := params.(map[string]interface{})
			if ok {
				for key, value := range paramMap {
					switch v := value.(type) {
					case string:
						paramMap[key] = []string{v}
					case []interface{}:
						strArr := make([]string, len(v))
						for i, elem := range v {
							if s, ok := elem.(string); ok {
								strArr[i] = s
							} else {
								strArr[i] = fmt.Sprintf("%v", elem)
							}
						}
						paramMap[key] = strArr
					case []string:
						// Already correct
					}
				}
			}
		}
	}

	return nil
}

// SanitizeForMockServer removes/fixes fields that aren't MockServer-compatible
func SanitizeForMockServer(config *MockConfiguration) error {
	for i := range config.Expectations {
		exp := &config.Expectations[i]

		// Remove name from httpRequest (use it only for display)
		if exp.HttpRequest != nil {
			delete(exp.HttpRequest, "name")
		}

		// Validate and fix the expectation
		if err := ValidateMockServerExpectation(exp); err != nil {
			return fmt.Errorf("expectation %d (%s): %w", i, exp.ID, err)
		}
	}

	return nil
}

// ToMockServerJSON converts config to MockServer-compatible JSON
func (c *MockConfiguration) ToMockServerJSON() (string, error) {
	// Create a copy to avoid modifying original
	configCopy := *c

	// Sanitize before converting
	if err := SanitizeForMockServer(&configCopy); err != nil {
		return "", err
	}

	// Convert to JSON
	jsonBytes, err := json.MarshalIndent(configCopy.Expectations, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal expectations: %w", err)
	}

	return string(jsonBytes), nil
}

func (config *MockConfiguration) GetProjectID() string {
	return config.Metadata.ProjectID
}
