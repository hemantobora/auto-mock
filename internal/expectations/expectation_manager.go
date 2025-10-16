package expectations

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/builders"
	"github.com/hemantobora/auto-mock/internal/models"
)

// ExpectationManager handles CRUD operations on mock expectations
type ExpectationManager struct {
	projectName string
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

	if len(expectations) == 1 {
		fmt.Printf("🔍 Found 1 expectation\n\n")
		return displayFullConfiguration(config)
	}

	fmt.Printf("🔍 Found %d expectations\n\n", len(expectations))

	for {
		apiList := buildAPIList(expectations)
		options := make([]string, 0, len(apiList)+2)

		for _, api := range apiList {
			options = append(options, api)
		}

		options = append(options, "📜 View All - Show complete configuration file")
		options = append(options, "🔙 Back - Return to main menu")

		var selected string
		if err := survey.AskOne(&survey.Select{
			Message: "Select expectation to view:",
			Options: options,
		}, &selected); err != nil {
			return err
		}

		if strings.Contains(selected, "View All") {
			if err := displayFullConfiguration(config); err != nil {
				return err
			}
			continue
		}

		if strings.Contains(selected, "Back") {
			return nil
		}

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

func displaySingleExpectation(expectation *models.MockExpectation) error {
	fmt.Println("\n📝 EXPECTATION DETAILS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	jsonBytes, err := json.MarshalIndent(expectation, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format expectation: %w", err)
	}

	fmt.Printf("\n%s\n\n", string(jsonBytes))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	name := expectation.Description
	if name == "" {
		name = fmt.Sprintf("%s %s", expectation.HttpRequest.Method, expectation.HttpRequest.Path)
	}
	fmt.Printf("🏷️  Name: %s\n", name)
	fmt.Printf("🔗 Method: %s %s\n", expectation.HttpRequest.Method, expectation.HttpRequest.Path)
	fmt.Printf("📊 Status: %d\n", expectation.HttpResponse.StatusCode)

	return nil
}

func displayFullConfiguration(config *models.MockConfiguration) error {
	mockServerJSON := models.ExpectationsToMockServerJSON(config.Expectations)

	fmt.Println("\n📝 COMPLETE CONFIGURATION FILE")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("\n%s\n\n", mockServerJSON)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📊 Total Expectations: %d\n", len(config.Expectations))
	fmt.Printf("💾 Configuration Size: %d bytes\n", len(mockServerJSON))

	return nil
}

func (em *ExpectationManager) EditExpectations(config *models.MockConfiguration) (*models.MockConfiguration, error) {
	fmt.Println("\n✏️  EDIT EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if config == nil || len(config.Expectations) == 0 {
		fmt.Println("📭 No expectations found for this project.")
		return nil, nil
	}

	expectations := config.Expectations

	for {
		if len(expectations) == 1 {
			fmt.Printf("🔍 Found 1 expectation: %s %s\n", expectations[0].HttpRequest.Method, expectations[0].HttpRequest.Path)

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

			if err := editSingleExpectation(&expectations[0]); err != nil {
				return nil, fmt.Errorf("edit failed: %w", err)
			}

			config.Expectations = expectations
			return config, nil
		}

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

		selectedIndex := findExpectationIndex(apiList, selectedAPI)
		if selectedIndex == -1 {
			continue
		}

		if err := editSingleExpectation(&expectations[selectedIndex]); err != nil {
			fmt.Printf("❌ Edit failed: %v\n", err)
			continue
		}

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

	config.Expectations = expectations
	return config, nil
}

func buildAPIList(expectations []models.MockExpectation) []string {
	var apiList []string
	for _, exp := range expectations {
		method := exp.HttpRequest.Method
		path := exp.HttpRequest.Path
		statusCode := exp.HttpResponse.StatusCode
		name := exp.Description

		queryInfo := ""
		if len(exp.HttpRequest.QueryStringParameters) > 0 {
			var queryParts []string
			for key, val := range exp.HttpRequest.QueryStringParameters {
				queryParts = append(queryParts, fmt.Sprintf("%s=%s", key, val))
			}
			if len(queryParts) > 0 {
				queryInfo = fmt.Sprintf(" [?%s]", strings.Join(queryParts, "&"))
			}
		}

		var displayName string
		if name != "" && queryInfo != "" {
			displayName = fmt.Sprintf("%s %s %s%s (%d)", name, method, path, queryInfo, statusCode)
		} else if name != "" {
			displayName = fmt.Sprintf("%s %s %s (%d)", name, method, path, statusCode)
		} else {
			displayName = fmt.Sprintf("%s %s%s (%d)", method, path, queryInfo, statusCode)
		}

		apiList = append(apiList, displayName)
	}
	return apiList
}

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

func findExpectationIndex(apiList []string, selected string) int {
	for i, api := range apiList {
		if api == selected {
			return i
		}
	}
	return -1
}

func (em *ExpectationManager) DownloadExpectations(config *models.MockConfiguration) error {
	fmt.Println("\n💾 DOWNLOAD EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	mockServerJSON := models.ExpectationsToMockServerJSON(config.Expectations)
	filename := fmt.Sprintf("%s-expectations.json", em.projectName)

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
		return fmt.Errorf("replace cancelled by user")
	}

	fmt.Println("🚀 Proceeding with new expectation generation...")
	return nil
}

func (em *ExpectationManager) RemoveExpectations(config *models.MockConfiguration) ([]int, error) {
	fmt.Println("\n🗑️  REMOVE EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if config == nil || len(config.Expectations) == 0 {
		fmt.Println("📭 No expectations found for this project.")
		return nil, nil
	}

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

	indices := findExpectationIndices(apiList, selectedAPIs)

	if len(indices) == len(config.Expectations) {
		return handleRemoveAllExpectations()
	}

	return indices, nil
}

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

	return []int{-1}, nil
}

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
		return fmt.Errorf("deletion cancelled by user")
	}

	var finalConfirm string
	if err := survey.AskOne(&survey.Input{
		Message: fmt.Sprintf("Type '%s' to confirm deletion:", em.projectName),
	}, &finalConfirm); err != nil {
		return err
	}

	if finalConfirm != em.projectName {
		fmt.Println("❌ Project name doesn't match. Deletion cancelled.")
		return fmt.Errorf("project name mismatch")
	}

	return nil
}

func editSingleExpectation(expectation *models.MockExpectation) error {
	fmt.Printf("\n📝 Editing: %s %s\n", expectation.HttpRequest.Method, expectation.HttpRequest.Path)
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
				"priority - Set Expectation Priority",
				"times - Set Expectation Times",
				"done - Finish Editing",
			},
		}, &editOption); err != nil {
			return err
		}

		editOption = strings.Split(editOption, " ")[0]

		switch editOption {
		case "method":
			editMethod(expectation)
		case "path":
			editPath(expectation)
		case "status":
			editStatusCode(expectation)
		case "body":
			editResponseBody(expectation)
		case "headers":
			editResponseHeaders(expectation)
		case "query":
			editQueryParams(expectation)
		case "view":
			viewCurrentConfig(expectation)
		case "priority":
			editPriority(expectation)
		case "times":
			editTimes(expectation)
		case "done":
			return nil
		}
	}
}

func editPriority(expectation *models.MockExpectation) {
	var priority int
	if err := survey.AskOne(&survey.Input{
		Message: "Enter expectation priority (lower number = higher priority):",
		Default: fmt.Sprintf("%d", expectation.Priority),
		Help:    "Example: 1, 5, 10",
	}, &priority); err == nil && priority >= 0 {
		expectation.Priority = priority
		fmt.Printf("✅ Updated priority to %d\n", priority)
	}
}

func editTimes(expectation *models.MockExpectation) {
	var times int
	if err := survey.AskOne(&survey.Input{
		Message: "Enter number of times this expectation should be matched (0 = unlimited):",
		Default: fmt.Sprintf("%d", expectation.Times),
		Help:    "Example: 0, 1, 5",
	}, &times); err == nil && times >= 0 {
		if times == 0 {
			expectation.Times = &models.Times{RemainingTimes: times, Unlimited: true}
			fmt.Printf("✅ Updated times to unlimited (0)\n")
			return
		}
		expectation.Times = &models.Times{RemainingTimes: times, Unlimited: false}
		fmt.Printf("✅ Updated times to %d\n", times)
	}
}

func editMethod(expectation *models.MockExpectation) {
	var newMethod string
	if err := survey.AskOne(&survey.Select{
		Message: "Select HTTP method:",
		Options: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"},
		Default: expectation.HttpRequest.Method,
	}, &newMethod); err == nil {
		expectation.HttpRequest.Method = newMethod
		fmt.Printf("✅ Updated method to %s\n", newMethod)
	}
}

func editPath(expectation *models.MockExpectation) {
	var newPath string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter request path:",
		Default: expectation.HttpRequest.Path,
		Help:    "Example: /api/v1/users/{id}",
	}, &newPath); err == nil && strings.TrimSpace(newPath) != "" {
		expectation.HttpRequest.Path = newPath
		mc := &builders.MockConfigurator{}
		mc.CollectPathMatchingStrategy(0, expectation)
		fmt.Printf("✅ Updated path to %s\n", newPath)
	}
}

func editStatusCode(expectation *models.MockExpectation) {
	var statusCode string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter status code:",
		Default: fmt.Sprintf("%d", expectation.HttpResponse.StatusCode),
		Help:    "Example: 200, 404, 500",
	}, &statusCode); err == nil {
		var newStatus int
		if _, err := fmt.Sscanf(statusCode, "%d", &newStatus); err == nil && newStatus >= 100 && newStatus <= 599 {
			expectation.HttpResponse.StatusCode = newStatus
			fmt.Printf("✅ Updated status code to %d\n", newStatus)
		}
	}
}

func editResponseBody(expectation *models.MockExpectation) {
	currentBody := ""
	if body, ok := expectation.HttpResponse.Body.(string); ok {
		currentBody = body
	}
	var editChoice string
	if err := survey.AskOne(&survey.Select{
		Message: "How would you like to edit the response body?",
		Options: []string{"text - Edit as plain text", "json - Edit as JSON", "template - Use JSON template", "view - View current body"},
	}, &editChoice); err == nil {
		editChoice = strings.Split(editChoice, " ")[0]
		switch editChoice {
		case "view":
			fmt.Printf("\nCurrent response body:\n%s\n\n", currentBody)
		case "text":
			editBodyAsText(expectation, currentBody)
		case "json":
			editBodyAsJSON(expectation, currentBody)
		}
	}
}

func editBodyAsText(expectation *models.MockExpectation, currentBody string) {
	var newBody string
	if err := survey.AskOne(&survey.Multiline{Message: "Enter response body:", Default: currentBody}, &newBody); err == nil {
		expectation.HttpResponse.Body = newBody
		fmt.Println("✅ Updated response body")
	}
}

func editBodyAsJSON(expectation *models.MockExpectation, currentBody string) {
	prettyBody := currentBody
	var jsonData interface{}
	if json.Unmarshal([]byte(currentBody), &jsonData) == nil {
		if prettyBytes, err := json.MarshalIndent(jsonData, "", "  "); err == nil {
			prettyBody = string(prettyBytes)
		}
	}
	var newBody string
	if err := survey.AskOne(&survey.Multiline{Message: "Enter JSON response body:", Default: prettyBody}, &newBody); err == nil {
		if json.Unmarshal([]byte(newBody), &jsonData) == nil {
			expectation.HttpResponse.Body = newBody
			fmt.Println("✅ Updated JSON response body")
		} else {
			fmt.Println("❌ Invalid JSON")
		}
	}
}

func editResponseHeaders(expectation *models.MockExpectation) {
	if expectation.HttpResponse.Headers == nil {
		expectation.HttpResponse.Headers = make(map[string][]string)
	}
	for {
		var action string
		options := []string{"add - Add new header", "view - View current headers"}
		for key := range expectation.HttpResponse.Headers {
			options = append(options, fmt.Sprintf("edit:%s - Edit %s", key, key))
			options = append(options, fmt.Sprintf("delete:%s - Delete %s", key, key))
		}
		options = append(options, "done - Finish editing headers")
		if err := survey.AskOne(&survey.Select{Message: "Header actions:", Options: options}, &action); err != nil {
			return
		}
		actionParts := strings.Split(action, " ")
		if actionParts[0] == "add" {
			addResponseHeader(expectation)
		} else if actionParts[0] == "view" {
			viewResponseHeaders(expectation.HttpResponse.Headers)
		} else if actionParts[0] == "done" {
			return
		} else if strings.Contains(actionParts[0], ":") {
			parts := strings.Split(actionParts[0], ":")
			if len(parts) == 2 {
				if parts[0] == "edit" {
					editResponseHeaderValue(expectation, parts[1])
				} else if parts[0] == "delete" {
					delete(expectation.HttpResponse.Headers, parts[1])
					fmt.Printf("✅ Deleted header %s\n", parts[1])
				}
			}
		}
	}
}

func addResponseHeader(expectation *models.MockExpectation) {
	var headerName, headerValue string
	if err := survey.AskOne(&survey.Input{Message: "Header name:"}, &headerName); err == nil && strings.TrimSpace(headerName) != "" {
		if err := survey.AskOne(&survey.Input{Message: "Header value:"}, &headerValue); err == nil {
			expectation.HttpResponse.Headers[headerName] = []string{headerValue}
			fmt.Printf("✅ Added header %s\n", headerName)
		}
	}
}

func editResponseHeaderValue(expectation *models.MockExpectation, headerKey string) {
	var newValue string
	defaultValue := strings.Join(expectation.HttpResponse.Headers[headerKey], ", ")
	if err := survey.AskOne(&survey.Input{Message: fmt.Sprintf("New value for %s:", headerKey), Default: defaultValue}, &newValue); err == nil {
		expectation.HttpResponse.Headers[headerKey] = []string{newValue}
		fmt.Printf("✅ Updated header %s\n", headerKey)
	}
}

func viewResponseHeaders(headers map[string][]string) {
	fmt.Println("\nCurrent response headers:")
	if len(headers) == 0 {
		fmt.Println("  (no headers set)")
	} else {
		for k, v := range headers {
			fmt.Printf("  %s: %s\n", k, strings.Join(v, ", "))
		}
	}
	fmt.Println()
}

func editQueryParams(expectation *models.MockExpectation) {
	if expectation.HttpRequest.QueryStringParameters == nil {
		expectation.HttpRequest.QueryStringParameters = make(map[string][]string)
	}
	for {
		var action string
		options := []string{"add - Add parameter", "view - View current parameters"}
		for key := range expectation.HttpRequest.QueryStringParameters {
			options = append(options, fmt.Sprintf("edit:%s - Edit %s", key, key))
			options = append(options, fmt.Sprintf("delete:%s - Delete %s", key, key))
		}
		options = append(options, "done - Finish editing parameters")
		if err := survey.AskOne(&survey.Select{Message: "Query parameter actions:", Options: options}, &action); err != nil {
			return
		}
		actionParts := strings.Split(action, " ")
		if actionParts[0] == "add" {
			addQueryParam(expectation)
		} else if actionParts[0] == "view" {
			viewQueryParams(expectation.HttpRequest.QueryStringParameters)
		} else if actionParts[0] == "done" {
			return
		} else if strings.Contains(actionParts[0], ":") {
			parts := strings.Split(actionParts[0], ":")
			if len(parts) == 2 {
				if parts[0] == "edit" {
					editQueryParamValue(expectation, parts[1])
				} else if parts[0] == "delete" {
					delete(expectation.HttpRequest.QueryStringParameters, parts[1])
					fmt.Printf("✅ Deleted parameter %s\n", parts[1])
				}
			}
		}
	}
}

func addQueryParam(expectation *models.MockExpectation) {
	var paramName, paramValue string
	if err := survey.AskOne(&survey.Input{Message: "Parameter name:"}, &paramName); err == nil && strings.TrimSpace(paramName) != "" {
		if err := survey.AskOne(&survey.Input{Message: "Parameter value or pattern:"}, &paramValue); err == nil {
			expectation.HttpRequest.QueryStringParameters[paramName] = []string{paramValue}
			fmt.Printf("✅ Added parameter %s\n", paramName)
		}
	}
}

func editQueryParamValue(expectation *models.MockExpectation, paramKey string) {
	var newValue string
	defaultValue := strings.Join(expectation.HttpRequest.QueryStringParameters[paramKey], ", ")
	if err := survey.AskOne(&survey.Input{Message: fmt.Sprintf("New value for %s:", paramKey), Default: defaultValue}, &newValue); err == nil {
		expectation.HttpRequest.QueryStringParameters[paramKey] = []string{newValue}
		fmt.Printf("✅ Updated parameter %s\n", paramKey)
	}
}

func viewQueryParams(params map[string][]string) {
	fmt.Println("\nCurrent query parameters:")
	if len(params) == 0 {
		fmt.Println("  (no parameters set)")
	} else {
		for k, v := range params {
			fmt.Printf("  %s: %s\n", k, strings.Join(v, ", "))
		}
	}
	fmt.Println()
}

func viewCurrentConfig(expectation *models.MockExpectation) {
	fmt.Println("\n📋 Current Configuration:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	jsonBytes, _ := json.MarshalIndent(expectation, "", "  ")
	fmt.Printf("%s\n\n", string(jsonBytes))
}
