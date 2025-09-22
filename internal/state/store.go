// Package state provides persistent storage for mock configurations
package state

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ConfigMetadata contains metadata about a stored configuration
type ConfigMetadata struct {
	ProjectID   string    `json:"project_id"`
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Description string    `json:"description"`
	Provider    string    `json:"provider"`
	Size        int64     `json:"size"`
}

// MockConfiguration represents a complete mock server configuration
type MockConfiguration struct {
	Metadata     ConfigMetadata         `json:"metadata"`
	Expectations []MockExpectation      `json:"expectations"`
	Settings     map[string]interface{} `json:"settings,omitempty"`
}

// MockExpectation represents a single mock expectation
type MockExpectation struct {
	ID          string                 `json:"id"`
	Priority    int                    `json:"priority"`
	HttpRequest map[string]interface{} `json:"httpRequest"`
	HttpResponse map[string]interface{} `json:"httpResponse"`
	Times       *ExpectationTimes      `json:"times,omitempty"`
}

// ExpectationTimes defines how many times an expectation should match
type ExpectationTimes struct {
	RemainingTimes int  `json:"remainingTimes,omitempty"`
	Unlimited      bool `json:"unlimited,omitempty"`
}

// Store defines the interface for configuration storage
type Store interface {
	// SaveConfig saves a mock configuration
	SaveConfig(ctx context.Context, projectID string, config *MockConfiguration) error
	
	// GetConfig retrieves a mock configuration
	GetConfig(ctx context.Context, projectID string) (*MockConfiguration, error)
	
	// GetConfigVersion retrieves a specific version of a configuration
	GetConfigVersion(ctx context.Context, projectID, version string) (*MockConfiguration, error)
	
	// ListConfigs lists all configurations with metadata
	ListConfigs(ctx context.Context) ([]ConfigMetadata, error)
	
	// DeleteConfig removes a configuration
	DeleteConfig(ctx context.Context, projectID string) error
	
	// UpdateConfig updates an existing configuration
	UpdateConfig(ctx context.Context, projectID string, config *MockConfiguration) error
	
	// GetConfigHistory retrieves version history for a project
	GetConfigHistory(ctx context.Context, projectID string) ([]ConfigMetadata, error)
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
			Priority: len(expectations) - i, // Higher priority for earlier expectations
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

// ToMockServerJSON converts configuration back to MockServer JSON format
func (config *MockConfiguration) ToMockServerJSON() (string, error) {
	expectations := make([]map[string]interface{}, 0, len(config.Expectations))
	
	for _, exp := range config.Expectations {
		mockExp := map[string]interface{}{
			"httpRequest":  exp.HttpRequest,
			"httpResponse": exp.HttpResponse,
		}
		
		if exp.Times != nil {
			times := map[string]interface{}{}
			if exp.Times.Unlimited {
				times["unlimited"] = true
			} else if exp.Times.RemainingTimes > 0 {
				times["remainingTimes"] = exp.Times.RemainingTimes
			}
			if len(times) > 0 {
				mockExp["times"] = times
			}
		}
		
		expectations = append(expectations, mockExp)
	}
	
	jsonBytes, err := json.MarshalIndent(expectations, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal to MockServer JSON: %w", err)
	}
	
	return string(jsonBytes), nil
}
