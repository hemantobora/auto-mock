package builders

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

// BuildRESTExpectationWithContext builds a REST expectation with context of existing expectations
func BuildRESTExpectationWithContext() (MockExpectation, error) {
	var expectation MockExpectation
	var mock_configurator MockConfigurator

	fmt.Println("🚀 Starting Enhanced 7-Step REST Expectation Builder")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	steps := []struct {
		name string
		fn   func(step int, exp *MockExpectation) error
	}{
		{"API Details", collectRESTAPIDetails},
		{"Query Parameter Matching", mock_configurator.CollectQueryParameterMatching},
		{"Path Matching Strategy", mock_configurator.CollectPathMatchingStrategy},
		{"Request Header Matching", mock_configurator.CollectRequestHeaderMatching},
		{"Response Definition", collectResponseDefinition},
		{"Response Header", mock_configurator.CollectResponseHeader},
		{"Advanced Features", mock_configurator.CollectAdvancedFeatures},
		{"Review and Confirm", reviewAndConfirm},
	}

	for i, step := range steps {
		if err := step.fn(i+1, &expectation); err != nil {
			return expectation, &models.ExpectationBuildError{
				ExpectationType: "REST",
				Step:            step.name,
				Cause:           err,
			}
		}
	}

	return expectation, nil
}

// Step 1: Collect API Details (Method, Path, Request Body)
func collectRESTAPIDetails(step int, expectation *MockExpectation) error {
	fmt.Printf("\n📋 Step %d: API Details\n", step)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━")
	var mock_configurator MockConfigurator

	expectation.HttpRequest = &models.HttpRequest{
		Headers:               make(map[string][]any),
		QueryStringParameters: make(map[string][]string),
	}

	// HTTP Method selection
	var method string
	if err := survey.AskOne(&survey.Select{
		Message: "Select HTTP method:",
		Options: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		Default: "GET",
	}, &method); err != nil {
		return err
	}
	expectation.HttpRequest.Method = method

	// Path collection
	var path string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter the API path:",
		Help:    "Use {param} for path parameters, e.g., /api/users/{id}",
		Default: "/api/users/{id}",
	}, &path); err != nil {
		return err
	}

	// Smart query parameter detection and path cleaning
	cleanPath, detectedParams := mock_configurator.ParsePathAndQueryParams(path)
	expectation.HttpRequest.Path = cleanPath

	// Show detected query parameters
	if len(detectedParams) > 0 {
		fmt.Printf("\n💡 Query parameters detected in path:\n")
		for name, value := range detectedParams {
			fmt.Printf("   %s=%s\n", name, value)
		}

		var useDetected bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Auto-configure these query parameters for matching?",
			Default: true,
		}, &useDetected); err != nil {
			return err
		}

		if useDetected {
			expectation.HttpRequest.QueryStringParameters = detectedParams
			fmt.Printf("✅ Pre-configured %d query parameters\n", len(detectedParams))
		}
	}

	// Request body for methods that typically have bodies
	if method == "POST" || method == "PUT" || method == "PATCH" {
		if err := mock_configurator.CollectRequestBody(expectation, ""); err != nil {
			return err
		}
	}

	fmt.Printf("✅ API Details: %s %s\n", expectation.HttpRequest.Method, expectation.HttpRequest.Path)
	return nil
}

// Step 5: Response Definition
func collectResponseDefinition(step int, expectation *MockExpectation) error {
	fmt.Printf("\n📤 Step %d: Response Definition\n", step)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	expectation.HttpResponse = &models.HttpResponse{
		Headers: make(map[string][]string),
	}

	// Status code selection (hierarchical)
	if err := collectStatusCode(expectation); err != nil {
		return err
	}

	// Response body
	if expectation.HttpResponse.StatusCode == 204 {
		// No body for 204
		expectation.HttpResponse.Body = ""
		fmt.Println("ℹ️  204 No Content - no response body configured")
		return nil
	} else {
		if err := collectResponseBody(expectation); err != nil {
			return err
		}
	}

	fmt.Printf("✅ Response: %d with body configured\n", expectation.HttpResponse.StatusCode)
	return nil
}

// collectStatusCode collects HTTP status code using hierarchical selection
func collectStatusCode(expectation *MockExpectation) error {
	fmt.Println("\n🔢 Status Code Selection")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━")

	statusCodes := CommonStatusCodes()

	// Step 1: Choose category
	var categories []string
	for category := range statusCodes {
		categories = append(categories, category)
	}

	var selectedCategory string
	if err := survey.AskOne(&survey.Select{
		Message: "Select status code category:",
		Options: categories,
		Default: "2xx Success",
	}, &selectedCategory); err != nil {
		return err
	}

	// Step 2: Choose specific code
	codes := statusCodes[selectedCategory]
	var codeOptions []string
	for _, code := range codes {
		codeOptions = append(codeOptions, fmt.Sprintf("%d - %s", code.Code, code.Description))
	}

	var selectedCode string
	if err := survey.AskOne(&survey.Select{
		Message: "Select specific status code:",
		Options: codeOptions,
	}, &selectedCode); err != nil {
		return err
	}

	// Parse status code
	codeStr := strings.Split(selectedCode, " - ")[0]
	statusCode, err := strconv.Atoi(codeStr)
	if err != nil {
		return &models.InputValidationError{
			InputType: "status code",
			Value:     codeStr,
			Expected:  "valid HTTP status code",
			Cause:     err,
		}
	}

	expectation.HttpResponse.StatusCode = statusCode
	return nil
}

// collectResponseBody collects the response body
func collectResponseBody(expectation *MockExpectation) error {
	fmt.Println("\n📄 Response Body")
	fmt.Println("━━━━━━━━━━━━━━━━━━━")

	var bodyChoice string
	if err := survey.AskOne(&survey.Select{
		Message: "How do you want to provide the response body?",
		Options: []string{
			"template - Generate from template",
			"json - Type/paste JSON directly",
		},
		Default: "json - Type/paste JSON directly",
	}, &bodyChoice); err != nil {
		return err
	}

	bodyChoice = strings.Split(bodyChoice, " ")[0]

	switch bodyChoice {
	case "template":
		if err := GenerateResponseTemplate(expectation); err != nil {
			return err
		}

	case "json":
		var responseJSON string
		if err := survey.AskOne(&survey.Multiline{
			Message: "Enter the response body JSON:",
			Help:    "Paste your JSON response here. Leave empty for no body.",
		}, &responseJSON); err != nil {
			return err
		}

		responseJSON = strings.TrimSpace(responseJSON)
		if responseJSON == "" {
			// Empty response
			expectation.HttpResponse.Body = ""
			expectation.HttpResponse.StatusCode = 204 // No Content
			fmt.Println("ℹ️  Empty response body - status code changed to 204")
			return nil
		}

		// Validate JSON
		if err := ValidateJSON(responseJSON); err != nil {
			fmt.Printf("⚠️  JSON validation failed: %v\n", err)
			return &models.JSONValidationError{
				Context: "response body",
				Content: responseJSON,
				Cause:   err,
			}
		}

		// Format and store JSON
		formattedJSON, _ := FormatJSON(responseJSON)
		expectation.HttpResponse.Body = map[string]any{
			"type": "JSON",
			"json": formattedJSON,
		}
		fmt.Println("✅ Response body JSON configured")

	default:
		return fmt.Errorf("unsupported body input method: %s", bodyChoice)
	}

	return nil
}

// Step 8: Review and Confirm
func reviewAndConfirm(step int, expectation *MockExpectation) error {
	fmt.Printf("\n🔄 Step %d: Review and Confirm\n", step)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Display summary
	fmt.Printf("\n📋 Expectation Summary:\n")
	if expectation.Description != "" {
		fmt.Printf("   Description: %s\n", expectation.Description)
	}
	fmt.Printf("   Method: %s\n", expectation.HttpRequest.Method)
	fmt.Printf("   Path: %s\n", expectation.HttpRequest.Path)
	fmt.Printf("   Status Code: %d\n", expectation.HttpResponse.StatusCode)

	if len(expectation.HttpRequest.QueryStringParameters) > 0 {
		fmt.Printf("   Query Parameters: %d\n", len(expectation.HttpRequest.QueryStringParameters))
	}
	if len(expectation.HttpRequest.Headers) > 0 {
		fmt.Printf("   Request Headers: %d\n", len(expectation.HttpRequest.Headers))
	}
	if expectation.HttpRequest.Body != nil {
		fmt.Printf("   Request Body: Configured\n")
	}

	var confirm bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Create this expectation?",
		Default: true,
	}, &confirm); err != nil {
		return err
	}

	if !confirm {
		fmt.Println("\nℹ️  Expectation creation cancelled")
		fmt.Println("🔄 You can start over or exit")
		return fmt.Errorf("expectation creation cancelled by user")
	}

	fmt.Printf("\n✅ REST Expectation Created: %s\n", expectation.Description)
	return nil
}
