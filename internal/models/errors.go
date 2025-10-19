package models

import "fmt"

// CollectionParsingError represents errors during collection file parsing
type CollectionParsingError struct {
	CollectionType string
	FilePath       string
	Line           int
	Cause          error
}

func (e *CollectionParsingError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("failed to parse %s collection at line %d in file '%s': %v",
			e.CollectionType, e.Line, e.FilePath, e.Cause)
	}
	return fmt.Sprintf("failed to parse %s collection in file '%s': %v",
		e.CollectionType, e.FilePath, e.Cause)
}

func (e *CollectionParsingError) Unwrap() error {
	return e.Cause
}

// APIExecutionError represents errors during API execution in collection processing
type APIExecutionError struct {
	APIName    string
	Method     string
	URL        string
	StatusCode int
	Cause      error
}

func (e *APIExecutionError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("API execution failed: %s %s %s (status %d): %v",
			e.APIName, e.Method, e.URL, e.StatusCode, e.Cause)
	}
	return fmt.Sprintf("API execution failed: %s %s %s: %v",
		e.APIName, e.Method, e.URL, e.Cause)
}

func (e *APIExecutionError) Unwrap() error {
	return e.Cause
}

// VariableResolutionError represents errors during variable resolution
type VariableResolutionError struct {
	VariableName string
	Source       string // "environment", "pre-script", "user-input", etc.
	Cause        error
}

func (e *VariableResolutionError) Error() string {
	return fmt.Sprintf("failed to resolve variable '%s' from %s: %v",
		e.VariableName, e.Source, e.Cause)
}

func (e *VariableResolutionError) Unwrap() error {
	return e.Cause
}

// ScriptExecutionError represents errors during pre/post script execution
type ScriptExecutionError struct {
	ScriptType string // "pre-request" or "post-response"
	APIName    string
	ScriptLine int
	Cause      error
}

func (e *ScriptExecutionError) Error() string {
	if e.ScriptLine > 0 {
		return fmt.Sprintf("%s script execution failed for '%s' at line %d: %v",
			e.ScriptType, e.APIName, e.ScriptLine, e.Cause)
	}
	return fmt.Sprintf("%s script execution failed for '%s': %v",
		e.ScriptType, e.APIName, e.Cause)
}

func (e *ScriptExecutionError) Unwrap() error {
	return e.Cause
}

// ConfigValidationError represents configuration validation errors (already exists in config.go)
// We'll keep the existing ValidationError type and not duplicate it

// ProviderError represents cloud provider operation errors
type ProviderError struct {
	Provider  string // "aws", "gcp", "azure"
	Operation string // "init", "deploy", "destroy", etc.
	Resource  string // bucket name, project name, etc.
	Cause     error
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s provider error during %s operation on resource '%s': %v",
		e.Provider, e.Operation, e.Resource, e.Cause)
}

func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// DeploymentError represents infrastructure deployment errors
type DeploymentError struct {
	ProjectName string
	Phase       string // "init", "plan", "apply", "destroy"
	Cause       error
}

func (e *DeploymentError) Error() string {
	return fmt.Sprintf("deployment error for project '%s' during %s phase: %v",
		e.ProjectName, e.Phase, e.Cause)
}

func (e *DeploymentError) Unwrap() error {
	return e.Cause
}

// AIGenerationError represents AI provider generation errors
type AIGenerationError struct {
	Provider string // "anthropic", "openai", "template"
	Input    string // truncated input for context
	Cause    error
}

func (e *AIGenerationError) Error() string {
	truncatedInput := e.Input
	if len(truncatedInput) > 100 {
		truncatedInput = truncatedInput[:100] + "..."
	}
	return fmt.Sprintf("AI generation failed using %s provider for input '%s': %v",
		e.Provider, truncatedInput, e.Cause)
}

func (e *AIGenerationError) Unwrap() error {
	return e.Cause
}

// ExpectationBuildError represents errors during mock expectation building
type ExpectationBuildError struct {
	ExpectationType string // "REST", "GraphQL", "WebSocket"
	Step            string // "API Details", "Response Definition", etc.
	Field           string // specific field that caused the error
	Cause           error
}

func (e *ExpectationBuildError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s expectation build failed at step '%s' (field: %s): %v",
			e.ExpectationType, e.Step, e.Field, e.Cause)
	}
	return fmt.Sprintf("%s expectation build failed at step '%s': %v",
		e.ExpectationType, e.Step, e.Cause)
}

func (e *ExpectationBuildError) Unwrap() error {
	return e.Cause
}

// JSONValidationError represents JSON validation errors
type JSONValidationError struct {
	Context string // "request body", "response body", etc.
	Content string // truncated JSON content
	Cause   error
}

func (e *JSONValidationError) Error() string {
	truncatedContent := e.Content
	if len(truncatedContent) > 100 {
		truncatedContent = truncatedContent[:100] + "..."
	}
	return fmt.Sprintf("JSON validation failed for %s: %v\nContent: %s",
		e.Context, e.Cause, truncatedContent)
}

func (e *JSONValidationError) Unwrap() error {
	return e.Cause
}

// RegexValidationError represents regex pattern validation errors
type RegexValidationError struct {
	Pattern string
	Context string // "header matching", "path matching", etc.
	Cause   error
}

func (e *RegexValidationError) Error() string {
	return fmt.Sprintf("invalid regex pattern '%s' for %s: %v",
		e.Pattern, e.Context, e.Cause)
}

func (e *RegexValidationError) Unwrap() error {
	return e.Cause
}

// InputValidationError represents user input validation errors
type InputValidationError struct {
	InputType string // "priority", "delay", "times", etc.
	Value     string
	Expected  string // description of expected format
	Cause     error
}

func (e *InputValidationError) Error() string {
	if e.Expected != "" {
		return fmt.Sprintf("invalid %s value '%s' (expected: %s): %v",
			e.InputType, e.Value, e.Expected, e.Cause)
	}
	return fmt.Sprintf("invalid %s value '%s': %v",
		e.InputType, e.Value, e.Cause)
}

func (e *InputValidationError) Unwrap() error {
	return e.Cause
}
