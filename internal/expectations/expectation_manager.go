package expectations

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

// ExpectationManager handles CRUD operations on mock expectations
type ExpectationManager struct {
	projectName string
}

// APIExpectation represents a single API expectation with metadata
type APIExpectation struct {
	ID          string                 `json:"id"`
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Description string                 `json:"description"`
	Raw         map[string]interface{} `json:"raw"`
}

// NewExpectationManager creates a new expectation manager
func NewExpectationManager(projectName string) (*ExpectationManager, error) {
	return &ExpectationManager{
		projectName: projectName,
	}, nil
}

// ViewExpectations displays expectations and allows viewing them individually or all together
func (em *ExpectationManager) ViewExpectations(config *models.MockConfiguration) error {
	fmt.Println("\n👁️  VIEW EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if config == nil || len(config.Expectations) == 0 {
		fmt.Println("📫 No expectations found for this project.")
		return nil
	}

	expectations := config.Expectations

	// If only one expectation, directly show it
	if len(expectations) == 1 {
		fmt.Printf("🔍 Found 1 expectation\n\n")
		return displayFullConfiguration(config)
	}

	// Multiple expectations - show each as an option + "view all"
	fmt.Printf("🔍 Found %d expectations\n\n", len(expectations))

	for {
		// Build options list
		apiList := buildAPIList(expectations)
		options := make([]string, 0, len(apiList)+2)

		// Add each expectation as an option
		for _, api := range apiList {
			options = append(options, api)
		}

		// Add "view all" and "back" options
		options = append(options, "📜 View All - Show complete configuration file")
		options = append(options, "🔙 Back - Return to main menu")

		var selected string
		if err := survey.AskOne(&survey.Select{
			Message: "Select expectation to view:",
			Options: options,
		}, &selected); err != nil {
			return err
		}

		// Handle "View All"
		if strings.Contains(selected, "View All") {
			if err := displayFullConfiguration(config); err != nil {
				return err
			}
			continue
		}

		// Handle "Back"
		if strings.Contains(selected, "Back") {
			return nil
		}

		// Find and display selected expectation
		for i, api := range apiList {
			if api == selected {
				if err := displaySingleExpectation(&expectations[i]); err != nil {
					return err
				}
				break
			}
		}
	}
}

// displaySingleExpectation displays a single expectation in detail
func displaySingleExpectation(expectation *models.MockExpectation) error {
	fmt.Println("\n📝 EXPECTATION DETAILS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Convert to JSON for display
	jsonBytes, err := json.MarshalIndent(expectation, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format expectation: %w", err)
	}

	fmt.Printf("\n%s\n\n", string(jsonBytes))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Get name from httpRequest if available
	name := fmt.Sprintf("%s %s", getMethod(expectation), getPath(expectation))
	if expectation.ID != "" {
		name = expectation.ID
	}
	fmt.Printf("🏷️  Name: %s\n", name)
	fmt.Printf("🔗 Method: %s %s\n", getMethod(expectation), getPath(expectation))

	// Get status code
	statusCode := expectation.HttpResponse["statusCode"]
	fmt.Printf("📊 Status: %s\n", statusCode)

	return nil
}

// displayFullConfiguration displays the complete configuration file
func displayFullConfiguration(config *models.MockConfiguration) error {

	// Convert to MockServer JSON
	mockServerJSON, err := config.ToMockServerJSON()
	if err != nil {
		return fmt.Errorf("failed to convert to MockServer JSON: %w", err)
	}

	fmt.Println("\n📝 COMPLETE CONFIGURATION FILE")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("\n%s\n\n", mockServerJSON)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📊 Total Expectations: %d\n", len(config.Expectations))
	fmt.Printf("💾 Configuration Size: %d bytes\n", len(mockServerJSON))

	return nil
}

// EditExpectations handles the interactive editing of expectations
// It takes a config, allows user to edit it, and returns the modified config
// This function does NOT interact with storage - that's the caller's responsibility
func (em *ExpectationManager) EditExpectations(config *models.MockConfiguration) (*models.MockConfiguration, error) {
	fmt.Println("\n✏️  EDIT EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if config == nil || len(config.Expectations) == 0 {
		fmt.Println("📭 No expectations found for this project.")
		return nil, nil
	}

	expectations := config.Expectations

	for {
		// Handle single expectation case
		if len(expectations) == 1 {
			fmt.Printf("🔍 Found 1 expectation: %s %s\n",
				expectations[0].HttpRequest["method"].(string),
				expectations[0].HttpRequest["path"].(string))

			var confirmEdit bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Edit this expectation?",
				Default: true,
			}, &confirmEdit); err != nil {
				return nil, err
			}

			if !confirmEdit {
				fmt.Println("✅ Edit cancelled.")
				return nil, nil
			}

			// Edit the single expectation
			if err := editSingleExpectation(&expectations[0]); err != nil {
				return nil, fmt.Errorf("edit failed: %w", err)
			}

			// Return modified config
			config.Expectations = expectations
			return config, nil
		}

		// Handle multiple expectations
		apiList := buildAPIList(expectations)
		apiList = append(apiList, "🔙 Finish editing and save changes")

		var selectedAPI string
		if err := survey.AskOne(&survey.Select{
			Message: "Select API to edit:",
			Options: apiList,
		}, &selectedAPI); err != nil {
			return nil, err
		}

		if strings.Contains(selectedAPI, "Finish editing") {
			break
		}

		// Find selected expectation
		selectedIndex := findExpectationIndex(apiList, selectedAPI)
		if selectedIndex == -1 {
			continue
		}

		// Edit the selected expectation
		if err := editSingleExpectation(&expectations[selectedIndex]); err != nil {
			fmt.Printf("❌ Edit failed: %v\n", err)
			continue
		}

		// Ask if user wants to edit more
		var editMore bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Edit another expectation?",
			Default: false,
		}, &editMore); err != nil {
			return nil, err
		}

		if !editMore {
			break
		}
	}

	// Return modified config
	config.Expectations = expectations
	return config, nil
}

// buildAPIList creates display strings for each expectation
func buildAPIList(expectations []models.MockExpectation) []string {
	var apiList []string
	for _, exp := range expectations {
		// Get status code for display
		statusCode := "200"
		if httpResp, ok := exp.HttpResponse["statusCode"].(int); ok {
			statusCode = fmt.Sprintf("%d", httpResp)
		} else if httpResp, ok := exp.HttpResponse["statusCode"].(float64); ok {
			statusCode = fmt.Sprintf("%.0f", httpResp)
		}

		// Get method and path
		method := "?"
		path := "?"
		if m, ok := exp.HttpRequest["method"].(string); ok {
			method = m
		}
		if p, ok := exp.HttpRequest["path"].(string); ok {
			path = p
		}

		// Get query parameters for display
		queryInfo := ""
		if params, ok := exp.HttpRequest["queryStringParameters"].(map[string]interface{}); ok && len(params) > 0 {
			var queryParts []string
			for key, val := range params {
				if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
					queryParts = append(queryParts, fmt.Sprintf("%s=%v", key, arr[0]))
				} else if strArr, ok := val.([]string); ok && len(strArr) > 0 {
					queryParts = append(queryParts, fmt.Sprintf("%s=%s", key, strArr[0]))
				} else {
					queryParts = append(queryParts, fmt.Sprintf("%s=%v", key, val))
				}
			}
			if len(queryParts) > 0 {
				queryInfo = fmt.Sprintf(" [?%s]", strings.Join(queryParts, "&"))
			}
		}

		// Build display string
		var displayName string
		if exp.ID != "" && queryInfo != "" {
			// Name + query params (e.g., "123 GET /api/v1/mobile [?type=New] (200)")
			displayName = fmt.Sprintf("%s %s %s%s (%s)", exp.ID, method, path, queryInfo, statusCode)
		} else if exp.ID != "" {
			// Just name (e.g., "123 GET /api/v1/mobile (200)")
			displayName = fmt.Sprintf("%s %s %s (%s)", exp.ID, method, path, statusCode)
		} else {
			// Method + path + query params (e.g., "GET /api/v1/mobile [?type=New] (200)")
			displayName = fmt.Sprintf("%s %s%s (%s)", method, path, queryInfo, statusCode)
		}

		apiList = append(apiList, displayName)
	}
	return apiList
}

// findExpectationIndices finds the indices of selected expectations
func findExpectationIndices(apiList []string, selectedAPIs []string) []int {
	var indices []int
	for i, api := range apiList {
		for _, selected := range selectedAPIs {
			if api == selected {
				indices = append(indices, i)
				break
			}
		}
	}
	return indices
}

// DownloadExpectations downloads the entire expectations file
func (em *ExpectationManager) DownloadExpectations(config *models.MockConfiguration) error {
	fmt.Println("\n💾 DOWNLOAD EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Convert to MockServer JSON
	mockServerJSON, err := config.ToMockServerJSON()
	if err != nil {
		return fmt.Errorf("failed to convert to MockServer JSON: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("%s-expectations.json", em.projectName)

	// Write to file
	if err := os.WriteFile(filename, []byte(mockServerJSON), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("\n✅ Expectations downloaded successfully!\n")
	fmt.Printf("📁 File: %s\n", filename)
	fmt.Printf("📊 Expectations: %d\n", len(config.Expectations))
	fmt.Printf("💾 Size: %d bytes\n", len(mockServerJSON))
	fmt.Printf("\n💡 You can now use this file with MockServer:\n")
	fmt.Printf("   curl -X PUT http://localhost:1080/mockserver/expectation -d @%s\n", filename)

	return nil
}

// ReplaceExpectations generates new expectations with warning
func (em *ExpectationManager) ReplaceExpectationsPrompt() error {
	fmt.Println("\n🔄 REPLACE EXPECTATIONS WARNING")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📦 Project: %s\n", em.projectName)
	fmt.Println("⚠️  This will replace ALL existing expectations")
	fmt.Println("💾 Previous version will be saved in version history")

	var confirmReplace bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Continue with replacing expectations?",
		Default: false,
	}, &confirmReplace); err != nil {
		return err
	}

	if !confirmReplace {
		fmt.Println("✅ Replace operation cancelled.")
		return nil
	}

	fmt.Println("🚀 Proceeding with new expectation generation...")
	return nil // Return to main generation flow
}

// JSON Templates

func getJSONTemplate(templateType string) string {
	switch templateType {
	case "success":
		return `{
  "success": true,
  "message": "Operation completed successfully",
  "timestamp": "${timestamp}",
  "data": {
    "id": "${uuid}"
  }
}`
	case "error":
		return `{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request parameters",
    "details": []
  },
  "timestamp": "${timestamp}"
}`
	case "list":
		return `{
  "data": [
    {
      "id": "${uuid}",
      "name": "Item 1"
    },
    {
      "id": "${uuid}",
      "name": "Item 2"
    }
  ],
  "pagination": {
    "total": 2,
    "page": 1,
    "limit": 10
  }
}`
	case "user":
		return `{
  "id": "${uuid}",
  "email": "user@example.com",
  "name": "John Doe",
  "role": "user",
  "createdAt": "${timestamp}",
  "profile": {
    "avatar": "https://example.com/avatar.jpg",
    "bio": "Sample user profile"
  }
}`
	case "product":
		return `{
  "id": "${uuid}",
  "name": "Sample Product",
  "description": "A sample product description",
  "price": {
    "amount": 99.99,
    "currency": "USD"
  },
  "category": "electronics",
  "inStock": true,
  "createdAt": "${timestamp}"
}`
	default:
		return `{
  "message": "Hello World",
  "timestamp": "${timestamp}"
}`
	}
}

// editSingleExpectation handles editing a single expectation
func editSingleExpectation(expectation *models.MockExpectation) error {
	method := getMethod(expectation)
	path := getPath(expectation)

	fmt.Printf("\n📝 Editing: %s %s\n", method, path)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for {
		var editOption string
		if err := survey.AskOne(&survey.Select{
			Message: "What would you like to edit?",
			Options: []string{
				"method - HTTP Method",
				"path - Request Path",
				"status - Response Status Code",
				"body - Response Body",
				"headers - Response Headers",
				"query - Query Parameters",
				"view - View Current Configuration",
				"done - Finish Editing",
			},
		}, &editOption); err != nil {
			return err
		}

		editOption = strings.Split(editOption, " ")[0]

		switch editOption {
		case "method":
			if err := editMethod(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "path":
			if err := editPath(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "status":
			if err := editStatusCode(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "body":
			if err := editResponseBody(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "headers":
			if err := editHeaders(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "query":
			if err := editQueryParams(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "view":
			viewCurrentConfig(expectation)
		case "done":
			return nil
		}
	}
}

// Helper functions for getting data from expectations

func getMethod(expectation *models.MockExpectation) string {
	if method, ok := expectation.HttpRequest["method"].(string); ok {
		return method
	}
	return "?"
}

func getPath(expectation *models.MockExpectation) string {
	if path, ok := expectation.HttpRequest["path"].(string); ok {
		return path
	}
	return "?"
}

// Edit helper methods

func editMethod(expectation *models.MockExpectation) error {
	currentMethod := getMethod(expectation)

	var newMethod string
	if err := survey.AskOne(&survey.Select{
		Message: "Select HTTP method:",
		Options: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"},
		Default: currentMethod,
	}, &newMethod); err != nil {
		return err
	}

	expectation.HttpRequest["method"] = newMethod
	fmt.Printf("✅ Updated method to %s\n", newMethod)
	return nil
}

func editPath(expectation *models.MockExpectation) error {
	currentPath := getPath(expectation)

	var newPath string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter request path:",
		Default: currentPath,
		Help:    "Example: /api/v1/users/{id}",
	}, &newPath); err != nil {
		return err
	}

	if strings.TrimSpace(newPath) == "" {
		return fmt.Errorf("path cannot be empty")
	}

	expectation.HttpRequest["path"] = newPath
	fmt.Printf("✅ Updated path to %s\n", newPath)
	return nil
}

func editStatusCode(expectation *models.MockExpectation) error {
	currentStatus := 200
	if status, ok := expectation.HttpResponse["statusCode"].(int); ok {
		currentStatus = status
	} else if status, ok := expectation.HttpResponse["statusCode"].(float64); ok {
		currentStatus = int(status)
	}

	var statusCode string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter status code:",
		Default: fmt.Sprintf("%d", currentStatus),
		Help:    "Example: 200, 404, 500",
	}, &statusCode); err != nil {
		return err
	}

	newStatus := 200
	if _, err := fmt.Sscanf(statusCode, "%d", &newStatus); err != nil {
		return fmt.Errorf("invalid status code: %s", statusCode)
	}

	if newStatus < 100 || newStatus > 599 {
		return fmt.Errorf("status code must be between 100-599")
	}

	expectation.HttpResponse["statusCode"] = newStatus
	fmt.Printf("✅ Updated status code to %d\n", newStatus)
	return nil
}

func editResponseBody(expectation *models.MockExpectation) error {
	currentBody := ""
	if body, ok := expectation.HttpResponse["body"].(string); ok {
		currentBody = body
	}

	var editChoice string
	if err := survey.AskOne(&survey.Select{
		Message: "How would you like to edit the response body?",
		Options: []string{
			"text - Edit as plain text",
			"json - Edit as JSON",
			"template - Use JSON template",
			"view - View current body",
		},
	}, &editChoice); err != nil {
		return err
	}

	editChoice = strings.Split(editChoice, " ")[0]

	switch editChoice {
	case "view":
		fmt.Printf("\nCurrent response body:\n%s\n\n", currentBody)
		return nil
	case "text":
		return editBodyAsText(expectation, currentBody)
	case "json":
		return editBodyAsJSON(expectation, currentBody)
	case "template":
		return editBodyWithTemplate(expectation)
	}
	return nil
}

func editBodyAsText(expectation *models.MockExpectation, currentBody string) error {
	var newBody string
	if err := survey.AskOne(&survey.Multiline{
		Message: "Enter response body:",
		Default: currentBody,
		Help:    "Enter the raw response body content",
	}, &newBody); err != nil {
		return err
	}

	expectation.HttpResponse["body"] = newBody
	fmt.Printf("✅ Updated response body\n")
	return nil
}

func editBodyAsJSON(expectation *models.MockExpectation, currentBody string) error {
	// Try to pretty-print current JSON
	prettyBody := currentBody
	var jsonData interface{}
	if err := json.Unmarshal([]byte(currentBody), &jsonData); err == nil {
		if prettyBytes, err := json.MarshalIndent(jsonData, "", "  "); err == nil {
			prettyBody = string(prettyBytes)
		}
	}

	var newBody string
	if err := survey.AskOne(&survey.Multiline{
		Message: "Enter JSON response body:",
		Default: prettyBody,
		Help:    "Enter valid JSON. Will be validated before saving.",
	}, &newBody); err != nil {
		return err
	}

	// Validate JSON
	var testData interface{}
	if err := json.Unmarshal([]byte(newBody), &testData); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	expectation.HttpResponse["body"] = newBody
	fmt.Printf("✅ Updated JSON response body\n")
	return nil
}

func editBodyWithTemplate(expectation *models.MockExpectation) error {
	var template string
	if err := survey.AskOne(&survey.Select{
		Message: "Select JSON template:",
		Options: []string{
			"success - Success response",
			"error - Error response",
			"list - List/array response",
			"user - User object",
			"product - Product object",
		},
	}, &template); err != nil {
		return err
	}

	templateBody := getJSONTemplate(strings.Split(template, " ")[0])
	expectation.HttpResponse["body"] = templateBody
	fmt.Printf("✅ Applied %s template\n", strings.Split(template, " ")[0])
	return nil
}

func editHeaders(expectation *models.MockExpectation) error {
	currentHeaders := make(map[string]string)
	if headers, ok := expectation.HttpResponse["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if strVal, ok := v.(string); ok {
				currentHeaders[k] = strVal
			}
		}
	}

	for {
		var action string
		options := []string{"add - Add new header", "view - View current headers"}

		for key := range currentHeaders {
			options = append(options, fmt.Sprintf("edit:%s - Edit %s", key, key))
			options = append(options, fmt.Sprintf("delete:%s - Delete %s", key, key))
		}

		options = append(options, "done - Finish editing headers")

		if err := survey.AskOne(&survey.Select{
			Message: "Header actions:",
			Options: options,
		}, &action); err != nil {
			return err
		}

		actionParts := strings.Split(action, " ")
		switch actionParts[0] {
		case "add":
			if err := addHeader(expectation, currentHeaders); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "view":
			viewHeaders(currentHeaders)
		case "done":
			return nil
		default:
			if strings.Contains(actionParts[0], ":") {
				parts := strings.Split(actionParts[0], ":")
				if len(parts) == 2 {
					headerAction := parts[0]
					headerKey := parts[1]

					if headerAction == "edit" {
						if err := editHeaderValue(expectation, currentHeaders, headerKey); err != nil {
							fmt.Printf("❌ Error: %v\n", err)
						}
					} else if headerAction == "delete" {
						delete(currentHeaders, headerKey)
						updateHeaders(expectation, currentHeaders)
						fmt.Printf("✅ Deleted header %s\n", headerKey)
					}
				}
			}
		}
	}
}

func addHeader(expectation *models.MockExpectation, currentHeaders map[string]string) error {
	var headerName, headerValue string

	if err := survey.AskOne(&survey.Input{
		Message: "Header name:",
		Help:    "Example: Content-Type, Authorization",
	}, &headerName); err != nil {
		return err
	}

	if strings.TrimSpace(headerName) == "" {
		return fmt.Errorf("header name cannot be empty")
	}

	if err := survey.AskOne(&survey.Input{
		Message: "Header value:",
		Help:    "Example: application/json, Bearer token",
	}, &headerValue); err != nil {
		return err
	}

	currentHeaders[headerName] = headerValue
	updateHeaders(expectation, currentHeaders)
	fmt.Printf("✅ Added header %s\n", headerName)
	return nil
}

func editHeaderValue(expectation *models.MockExpectation, currentHeaders map[string]string, headerKey string) error {
	currentValue := currentHeaders[headerKey]

	var newValue string
	if err := survey.AskOne(&survey.Input{
		Message: fmt.Sprintf("New value for %s:", headerKey),
		Default: currentValue,
	}, &newValue); err != nil {
		return err
	}

	currentHeaders[headerKey] = newValue
	updateHeaders(expectation, currentHeaders)
	fmt.Printf("✅ Updated header %s\n", headerKey)
	return nil
}

func updateHeaders(expectation *models.MockExpectation, headers map[string]string) {
	headerMap := make(map[string]interface{})
	for k, v := range headers {
		headerMap[k] = v
	}
	expectation.HttpResponse["headers"] = headerMap
}

func viewHeaders(headers map[string]string) {
	fmt.Println("\nCurrent headers:")
	if len(headers) == 0 {
		fmt.Println("  (no headers set)")
	} else {
		for k, v := range headers {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}
	fmt.Println()
}

func editQueryParams(expectation *models.MockExpectation) error {
	currentParams := make(map[string]interface{})
	if params, ok := expectation.HttpRequest["queryStringParameters"].(map[string]interface{}); ok {
		currentParams = params
	}

	for {
		var action string
		options := []string{"add - Add parameter", "view - View current parameters"}

		for key := range currentParams {
			options = append(options, fmt.Sprintf("edit:%s - Edit %s", key, key))
			options = append(options, fmt.Sprintf("delete:%s - Delete %s", key, key))
		}

		options = append(options, "done - Finish editing parameters")

		if err := survey.AskOne(&survey.Select{
			Message: "Query parameter actions:",
			Options: options,
		}, &action); err != nil {
			return err
		}

		actionParts := strings.Split(action, " ")
		switch actionParts[0] {
		case "add":
			if err := addQueryParam(expectation, currentParams); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "view":
			viewQueryParams(currentParams)
		case "done":
			return nil
		default:
			if strings.Contains(actionParts[0], ":") {
				parts := strings.Split(actionParts[0], ":")
				if len(parts) == 2 {
					paramAction := parts[0]
					paramKey := parts[1]

					if paramAction == "edit" {
						if err := editQueryParamValue(expectation, currentParams, paramKey); err != nil {
							fmt.Printf("❌ Error: %v\n", err)
						}
					} else if paramAction == "delete" {
						delete(currentParams, paramKey)
						updateQueryParams(expectation, currentParams)
						fmt.Printf("✅ Deleted parameter %s\n", paramKey)
					}
				}
			}
		}
	}
}

func addQueryParam(expectation *models.MockExpectation, currentParams map[string]interface{}) error {
	var paramName, paramValue string

	if err := survey.AskOne(&survey.Input{
		Message: "Parameter name:",
		Help:    "Example: id, type, limit",
	}, &paramName); err != nil {
		return err
	}

	if strings.TrimSpace(paramName) == "" {
		return fmt.Errorf("parameter name cannot be empty")
	}

	if err := survey.AskOne(&survey.Input{
		Message: "Parameter value or pattern:",
		Help:    "Example: 123, NEW, [0-9]+",
	}, &paramValue); err != nil {
		return err
	}

	currentParams[paramName] = []string{paramValue}
	updateQueryParams(expectation, currentParams)
	fmt.Printf("✅ Added parameter %s\n", paramName)
	return nil
}

func editQueryParamValue(expectation *models.MockExpectation, currentParams map[string]interface{}, paramKey string) error {
	currentValue := ""
	if val, ok := currentParams[paramKey]; ok {
		if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
			if str, ok := arr[0].(string); ok {
				currentValue = str
			}
		} else if strVal, ok := val.([]string); ok && len(strVal) > 0 {
			currentValue = strVal[0]
		}
	}

	var newValue string
	if err := survey.AskOne(&survey.Input{
		Message: fmt.Sprintf("New value for %s:", paramKey),
		Default: currentValue,
	}, &newValue); err != nil {
		return err
	}

	currentParams[paramKey] = []string{newValue}
	updateQueryParams(expectation, currentParams)
	fmt.Printf("✅ Updated parameter %s\n", paramKey)
	return nil
}

func updateQueryParams(expectation *models.MockExpectation, params map[string]interface{}) {
	expectation.HttpRequest["queryStringParameters"] = params
}

func viewQueryParams(params map[string]interface{}) {
	fmt.Println("\nCurrent query parameters:")
	if len(params) == 0 {
		fmt.Println("  (no parameters set)")
	} else {
		for k, v := range params {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}
	fmt.Println()
}

func viewCurrentConfig(expectation *models.MockExpectation) {
	fmt.Println("\n📋 Current Configuration:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Create a map to display the full expectation
	display := map[string]interface{}{
		"httpRequest":  expectation.HttpRequest,
		"httpResponse": expectation.HttpResponse,
	}
	if expectation.Times != nil {
		display["times"] = expectation.Times
	}
	if expectation.Priority > 0 {
		display["priority"] = expectation.Priority
	}

	jsonBytes, _ := json.MarshalIndent(display, "", "  ")
	fmt.Printf("%s\n\n", string(jsonBytes))
}

// findExpectationIndex finds the index of a selected expectation from the display list
func findExpectationIndex(apiList []string, selected string) int {
	for i, api := range apiList {
		if api == selected {
			return i
		}
	}
	return -1
}

// DeleteProject deletes entire project with confirmation
func (em *ExpectationManager) DeleteProjectPrompt() error {
	fmt.Println("\n⚠️  PROJECT DELETION WARNING")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📦 Project: %s\n", em.projectName)
	fmt.Println("🗑️  This will permanently delete:")
	fmt.Println("   • All mock expectations")
	fmt.Println("   • All version history")
	fmt.Println("   • S3 bucket and contents")
	fmt.Println("   • Any running infrastructure (when implemented)")
	fmt.Println("\n❌ THIS CANNOT BE UNDONE!")

	var confirmDelete bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Are you absolutely sure you want to delete this project?",
		Default: false,
	}, &confirmDelete); err != nil {
		return err
	}

	if !confirmDelete {
		fmt.Println("✅ Project deletion cancelled.")
		return nil
	}

	// Double confirmation for safety
	var finalConfirm string
	if err := survey.AskOne(&survey.Input{
		Message: fmt.Sprintf("Type '%s' to confirm deletion:", em.projectName),
	}, &finalConfirm); err != nil {
		return err
	}

	if finalConfirm != em.projectName {
		fmt.Println("❌ Project name doesn't match. Deletion cancelled.")
		return nil
	}
	return nil
}

// RemoveExpectations handles the UI for removing expectations
// Returns the list of expectation IDs/indices that user wants to remove
func (em *ExpectationManager) RemoveExpectations(config *models.MockConfiguration) ([]int, error) {
	fmt.Println("\n🗑️  REMOVE EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if config == nil || len(config.Expectations) == 0 {
		fmt.Println("📭 No expectations found for this project.")
		return nil, nil
	}

	// Build API list for multi-select
	apiList := buildAPIList(config.Expectations)

	var selectedAPIs []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select expectations to remove:",
		Options: apiList,
	}, &selectedAPIs); err != nil {
		return nil, err
	}

	if len(selectedAPIs) == 0 {
		fmt.Println("✅ No expectations selected for removal.")
		return nil, nil
	}

	// Confirm removal
	fmt.Printf("\n⚠️  You are about to remove %d expectation(s):\n", len(selectedAPIs))
	for _, api := range selectedAPIs {
		fmt.Printf("   • %s\n", api)
	}

	var confirmRemoval bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Continue with removal?",
		Default: false,
	}, &confirmRemoval); err != nil {
		return nil, err
	}

	if !confirmRemoval {
		fmt.Println("✅ Removal cancelled.")
		return nil, nil
	}

	// Find indices of selected expectations
	indices := findExpectationIndices(apiList, selectedAPIs)

	// Check if all expectations are being removed
	if len(indices) == len(config.Expectations) {
		return handleRemoveAllExpectations()
	}

	return indices, nil
}

// handleRemoveAllExpectations handles the special case when all expectations are removed
func handleRemoveAllExpectations() ([]int, error) {
	fmt.Println("\n🗑️ ALL EXPECTATIONS WILL BE REMOVED")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("⚠️  This will make the project empty but keep it active:")
	fmt.Println("   • Clear all mock expectations")
	fmt.Println("   • Tear down any running infrastructure")
	fmt.Println("   • Project can be reused later")

	var confirmClear bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Continue clearing all expectations?",
		Default: false,
	}, &confirmClear); err != nil {
		return nil, err
	}

	if !confirmClear {
		fmt.Println("✅ Remove operation cancelled.")
		return nil, nil
	}

	// Return special marker for "remove all"
	return []int{-1}, nil // -1 indicates "remove all"
}
