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

// TimeToLive controls how long an expectation is active
// Note: This is different from Times (in expectation.go) which controls match count
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
		// Validate that Method is set
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
		if exp.HttpRequest.Method == "" {
			return ValidationError{
				Field:   fmt.Sprintf("expectations[%d].httpRequest.method", i),
				Message: "HTTP method is required",
			}
		}
		if exp.HttpRequest.Path == "" {
			return ValidationError{
				Field:   fmt.Sprintf("expectations[%d].httpRequest.path", i),
				Message: "HTTP path is required",
			}
		}
		if exp.HttpResponse.StatusCode == 0 {
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
	var expectations []MockExpectation
	if err := json.Unmarshal([]byte(jsonData), &expectations); err != nil {
		return nil, fmt.Errorf("failed to parse MockServer JSON: %w", err)
	}

	config := &MockConfiguration{
		Metadata: ConfigMetadata{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Version:   "1.0",
		},
		Expectations: expectations,
	}

	return config, nil
}

// ToMockServerJSON converts config to MockServer-compatible JSON
func (c *MockConfiguration) ToMockServerJSON() (string, error) {
	// Use the ExpectationsToMockServerJSON function from expectation.go
	return ExpectationsToMockServerJSON(c.Expectations), nil
}

func (config *MockConfiguration) GetProjectID() string {
	return config.Metadata.ProjectID
}
