package builders

import (
	"encoding/json"
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

	// Mandatory: API Details
	if err := collectRESTAPIDetails(&expectation); err != nil {
		return expectation, &models.ExpectationBuildError{
			ExpectationType: "REST",
			Step:            "API Details",
			Cause:           err,
		}
	}

	// Mandatory: Response Definition
	if err := collectResponseDefinition(&expectation); err != nil {
		return expectation, &models.ExpectationBuildError{
			ExpectationType: "REST",
			Step:            "Response Definition",
			Cause:           err,
		}
	}

	// Optional steps — user selects which to configure (space to toggle, enter to continue)
	const (
		optQueryParams    = "Query parameter matching"
		optPathStrategy   = "Path matching strategy"
		optRequestHeaders = "Request header matching"
		optRespHeaders    = "Response headers"
		optAdvanced       = "Advanced features (delay, limits, priority, connection)"
	)
	var selected []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Configure additional options (space to select, enter to skip):",
		Options: []string{optQueryParams, optPathStrategy, optRequestHeaders, optRespHeaders, optAdvanced},
	}, &selected); err != nil {
		return expectation, err
	}

	has := func(opt string) bool {
		for _, s := range selected {
			if s == opt {
				return true
			}
		}
		return false
	}

	if has(optQueryParams) {
		if err := mock_configurator.CollectQueryParameterMatching(&expectation); err != nil {
			return expectation, &models.ExpectationBuildError{
				ExpectationType: "REST",
				Step:            "Query Parameter Matching",
				Cause:           err,
			}
		}
	}
	if has(optPathStrategy) {
		if err := mock_configurator.CollectPathMatchingStrategy(&expectation); err != nil {
			return expectation, &models.ExpectationBuildError{
				ExpectationType: "REST",
				Step:            "Path Matching Strategy",
				Cause:           err,
			}
		}
	}
	if has(optRequestHeaders) {
		if err := mock_configurator.CollectRequestHeaderMatching(&expectation); err != nil {
			return expectation, &models.ExpectationBuildError{
				ExpectationType: "REST",
				Step:            "Request Header Matching",
				Cause:           err,
			}
		}
	}
	if has(optRespHeaders) {
		if err := mock_configurator.CollectResponseHeader(&expectation); err != nil {
			return expectation, &models.ExpectationBuildError{
				ExpectationType: "REST",
				Step:            "Response Header",
				Cause:           err,
			}
		}
	}
	if has(optAdvanced) {
		if err := mock_configurator.CollectAdvancedFeatures(&expectation); err != nil {
			return expectation, &models.ExpectationBuildError{
				ExpectationType: "REST",
				Step:            "Advanced Features",
				Cause:           err,
			}
		}
	}

	// Mandatory: Review and Confirm
	if err := reviewAndConfirm(&expectation); err != nil {
		return expectation, &models.ExpectationBuildError{
			ExpectationType: "REST",
			Step:            "Review and Confirm",
			Cause:           err,
		}
	}

	return expectation, nil
}

// Step 1: Collect API Details (Method, Path, Request Body)
func collectRESTAPIDetails(expectation *MockExpectation) error {
	fmt.Printf("\n📋 API Details\n")
	var mock_configurator MockConfigurator

	expectation.HttpRequest = &models.HttpRequest{
		Headers:               []models.NameValues{},
		QueryStringParameters: []models.NameValues{},
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
		Message: "Enter the API path. Matching criteria will be asked later:",
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
			for name, value := range detectedParams {
				SetNameValues(&expectation.HttpRequest.QueryStringParameters, name, value)
			}
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
func collectResponseDefinition(expectation *MockExpectation) error {
	fmt.Printf("\n📤 Response Definition\n")

	expectation.HttpResponse = &models.HttpResponse{
		Headers: []models.NameValues{},
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
		for {
			var responseJSON string
			if err := survey.AskOne(&survey.Multiline{
				Message: "Enter the response body JSON:",
				Help:    "Paste your JSON response here. Leave empty for no body.",
			}, &responseJSON); err != nil {
				return err
			}

			responseJSON = strings.TrimSpace(responseJSON)
			responseJSON = strings.TrimPrefix(responseJSON, "\xEF\xBB\xBF") // strip UTF-8 BOM
			if responseJSON == "" {
				// Empty response
				expectation.HttpResponse.Body = ""
				expectation.HttpResponse.StatusCode = 204 // No Content
				fmt.Println("ℹ️  Empty response body - status code changed to 204")
				return nil
			}

			// Validate JSON
			var temp interface{}
			if err := json.Unmarshal([]byte(responseJSON), &temp); err != nil {
				fmt.Printf("❌ Invalid JSON: %v\n", err)
				var retry bool
				if askErr := survey.AskOne(&survey.Confirm{Message: "Try again?", Default: true}, &retry); askErr != nil {
					return askErr
				}
				if !retry {
					return &models.JSONValidationError{
						Context: "JSON validation",
						Content: responseJSON,
						Cause:   err,
					}
				}
				continue
			}

			expectation.HttpResponse.Body = map[string]any{
				"type": "JSON",
				"json": temp,
			}
			fmt.Println("✅ Response body JSON configured")
			return nil
		}

	default:
		return fmt.Errorf("unsupported body input method: %s", bodyChoice)
	}

	return nil
}

// Step 8: Review and Confirm
func reviewAndConfirm(expectation *MockExpectation) error {
	fmt.Printf("\n🔄 Review and Confirm\n")

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
