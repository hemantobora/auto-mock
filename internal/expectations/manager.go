package expectations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hemantobora/auto-mock/internal/state"
	"github.com/hemantobora/auto-mock/internal/utils"
)

// ExpectationManager handles CRUD operations on mock expectations
type ExpectationManager struct {
	store       *state.S3Store
	projectName string
	cleanName   string
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
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg)
	store := state.NewS3Store(s3Client, projectName)

	return &ExpectationManager{
		store:       store,
		projectName: projectName,
		cleanName:   utils.ExtractUserProjectName(projectName),
	}, nil
}

// ViewExpectations displays expectations and allows viewing them individually or all together
func (em *ExpectationManager) ViewExpectations() error {
	fmt.Println("\n👁️  VIEW EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Load existing expectations
	expectations, err := em.loadExpectations()
	if err != nil {
		return fmt.Errorf("failed to load expectations: %w", err)
	}
	
	if len(expectations) == 0 {
		fmt.Println("📫 No expectations found for this project.")
		return nil
	}
	
	// If only one expectation, directly show it
	if len(expectations) == 1 {
		fmt.Printf("🔍 Found 1 expectation\n\n")
		return em.displayFullConfiguration()
	}
	
	// Multiple expectations - show each as an option + "view all"
	fmt.Printf("🔍 Found %d expectations\n\n", len(expectations))
	
	for {
		// Build options list with each expectation + view all option
		apiList := em.buildAPIList(expectations)
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
			if err := em.displayFullConfiguration(); err != nil {
				return err
			}
			continue // Show menu again
		}
		
		// Handle "Back"
		if strings.Contains(selected, "Back") {
			return nil
		}
		
		// Find and display selected expectation
		for i, api := range apiList {
			if api == selected {
				if err := em.displaySingleExpectation(&expectations[i]); err != nil {
					return err
				}
				break
			}
		}
	}
}

// DownloadExpectations downloads the entire expectations file
func (em *ExpectationManager) DownloadExpectations() error {
	fmt.Println("\n💾 DOWNLOAD EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	ctx := context.Background()
	
	// Get configuration from S3
	config, err := em.store.GetConfig(ctx, em.cleanName)
	if err != nil {
		return fmt.Errorf("failed to load expectations: %w", err)
	}
	
	// Convert to MockServer JSON
	mockServerJSON, err := config.ToMockServerJSON()
	if err != nil {
		return fmt.Errorf("failed to convert to MockServer JSON: %w", err)
	}
	
	// Generate filename
	filename := fmt.Sprintf("%s-expectations.json", em.cleanName)
	
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

// displaySingleExpectation displays a single expectation in detail
func (em *ExpectationManager) displaySingleExpectation(expectation *APIExpectation) error {
	fmt.Println("\n📝 EXPECTATION DETAILS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Convert to JSON for display
	jsonBytes, err := json.MarshalIndent(expectation.Raw, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format expectation: %w", err)
	}
	
	fmt.Printf("\n%s\n\n", string(jsonBytes))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Get name from httpRequest if available
	name := fmt.Sprintf("%s %s", expectation.Method, expectation.Path)
	if httpReq, ok := expectation.Raw["httpRequest"].(map[string]interface{}); ok {
		if reqName, ok := httpReq["name"].(string); ok && reqName != "" {
			name = reqName
		}
	}
	fmt.Printf("🏷️  Name: %s\n", name)
	fmt.Printf("🔗 Method: %s %s\n", expectation.Method, expectation.Path)
	
	// Get status code
	statusCode := "?"
	if httpResp, ok := expectation.Raw["httpResponse"].(map[string]interface{}); ok {
		if status, ok := httpResp["statusCode"].(int); ok {
			statusCode = fmt.Sprintf("%d", status)
		} else if status, ok := httpResp["statusCode"].(float64); ok {
			statusCode = fmt.Sprintf("%.0f", status)
		}
	}
	fmt.Printf("📊 Status: %s\n", statusCode)
	
	return nil
}

// displayFullConfiguration displays the complete configuration file
func (em *ExpectationManager) displayFullConfiguration() error {
	ctx := context.Background()
	
	// Get configuration from S3
	config, err := em.store.GetConfig(ctx, em.cleanName)
	if err != nil {
		return fmt.Errorf("failed to load expectations: %w", err)
	}
	
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

// DeleteProject deletes entire project with confirmation
func (em *ExpectationManager) DeleteProject() error {
	fmt.Println("\n⚠️  PROJECT DELETION WARNING")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📦 Project: %s\n", em.cleanName)
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
		Message: fmt.Sprintf("Type '%s' to confirm deletion:", em.cleanName),
	}, &finalConfirm); err != nil {
		return err
	}

	if finalConfirm != em.cleanName {
		fmt.Println("❌ Project name doesn't match. Deletion cancelled.")
		return nil
	}

	fmt.Println("\n🗑️  Deleting project...")

	// TODO: Tear down infrastructure when implemented
	fmt.Println("   • Infrastructure teardown (placeholder)")

	// Delete S3 bucket and contents
	ctx := context.Background()
	if err := em.store.DeleteConfig(ctx, em.cleanName); err != nil {
		return fmt.Errorf("failed to delete project data: %w", err)
	}

	fmt.Printf("✅ Project '%s' deleted successfully!\n", em.cleanName)
	return nil
}

// ReplaceExpectations generates new expectations with warning
func (em *ExpectationManager) ReplaceExpectations() error {
	fmt.Println("\n🔄 REPLACE EXPECTATIONS WARNING")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📦 Project: %s\n", em.cleanName)
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

// EditExpectations allows editing individual expectations
func (em *ExpectationManager) EditExpectations() error {
	fmt.Println("\n✏️  EDIT EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Load existing expectations
	expectations, err := em.loadExpectations()
	if err != nil {
		return fmt.Errorf("failed to load expectations: %w", err)
	}

	if len(expectations) == 0 {
		fmt.Println("📭 No expectations found for this project.")
		return nil
	}

	for {
		// Handle single expectation case
		if len(expectations) == 1 {
			fmt.Printf("🔍 Found 1 expectation: %s %s\n", expectations[0].Method, expectations[0].Path)
			var confirmEdit bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Edit this expectation?",
				Default: true,
			}, &confirmEdit); err != nil {
				return err
			}

			if !confirmEdit {
				fmt.Println("✅ Edit cancelled.")
				return nil
			}

			// Edit the single expectation
			if err := em.editSingleExpectation(&expectations[0]); err != nil {
				return fmt.Errorf("edit failed: %w", err)
			}

			// Save updated expectations
			return em.saveExpectations(expectations)
		}

		// Handle multiple expectations
		// Display expectations for selection
		apiList := em.buildAPIList(expectations)
		apiList = append(apiList, "🔙 Back to main menu")

		var selectedAPI string
		if err := survey.AskOne(&survey.Select{
			Message: "Select API to edit:",
			Options: apiList,
		}, &selectedAPI); err != nil {
			return err
		}

		if strings.Contains(selectedAPI, "Back to main menu") {
			break
		}

		// Find selected expectation
		selectedIndex := em.findExpectationIndex(apiList, selectedAPI)
		if selectedIndex == -1 {
			continue
		}

		// Edit the selected expectation
		if err := em.editSingleExpectation(&expectations[selectedIndex]); err != nil {
			fmt.Printf("❌ Edit failed: %v\n", err)
			continue
		}

		// Ask if user wants to edit more
		var editMore bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Edit another expectation?",
			Default: false,
		}, &editMore); err != nil {
			return err
		}

		if !editMore {
			break
		}
	}

	// Save updated expectations
	return em.saveExpectations(expectations)
}

// RemoveExpectations allows removing selected expectations
func (em *ExpectationManager) RemoveExpectations() error {
	fmt.Println("\n🗑️  REMOVE EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Load existing expectations
	expectations, err := em.loadExpectations()
	if err != nil {
		return fmt.Errorf("failed to load expectations: %w", err)
	}

	if len(expectations) == 0 {
		fmt.Println("📭 No expectations found for this project.")
		return nil
	}

	// Build API list for multi-select
	apiList := em.buildAPIList(expectations)

	var selectedAPIs []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select expectations to remove:",
		Options: apiList,
	}, &selectedAPIs); err != nil {
		return err
	}

	if len(selectedAPIs) == 0 {
		fmt.Println("✅ No expectations selected for removal.")
		return nil
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
		return err
	}

	if !confirmRemoval {
		fmt.Println("✅ Removal cancelled.")
		return nil
	}

	// Remove selected expectations
	filteredExpectations := em.filterExpectations(expectations, selectedAPIs)

	// Handle special case: removing all expectations (project becomes empty)
	if len(filteredExpectations) == 0 {
		fmt.Println("\n🗑️ ALL EXPECTATIONS REMOVED")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("📦 Project: %s\n", em.cleanName)
		fmt.Println("⚠️  This will make the project empty but keep it active:")
		fmt.Println("   • Clear all mock expectations")
		fmt.Println("   • Tear down any running infrastructure")
		fmt.Println("   • Project can be reused later")

		var confirmClear bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Continue clearing all expectations?",
			Default: false,
		}, &confirmClear); err != nil {
			return err
		}

		if !confirmClear {
			fmt.Println("✅ Remove operation cancelled.")
			return nil
		}

		fmt.Println("\n🔄 Clearing project...")
		fmt.Println("   • Tearing down infrastructure (placeholder)")
		fmt.Println("   • Clearing expectation file")

		// Delete the configuration (empties the project)
		ctx := context.Background()
		if err := em.store.DeleteConfig(ctx, em.cleanName); err != nil {
			return fmt.Errorf("failed to clear expectations: %w", err)
		}

		fmt.Printf("\n✅ All expectations removed successfully!\n")
		fmt.Printf("📁 Project '%s' is now empty but still exists\n", em.cleanName)
		fmt.Println("💡 You can add new expectations anytime using 'automock init'")
		return nil
	}

	// Partial removal: Save updated expectations with new version
	fmt.Printf("\n🔄 Creating new version with %d expectation(s) removed...\n", len(selectedAPIs))

	if err := em.saveExpectations(filteredExpectations); err != nil {
		return fmt.Errorf("failed to save after removal: %w", err)
	}

	// If infrastructure exists, redeploy with updated expectations
	fmt.Println("   • Checking for running infrastructure...")
	fmt.Println("   • Redeployment with updated expectations (placeholder)")

	fmt.Printf("\n✅ Successfully removed %d expectation(s)!\n", len(selectedAPIs))
	fmt.Printf("📊 Remaining expectations: %d\n", len(filteredExpectations))
	return nil
}

// Helper methods

func (em *ExpectationManager) loadExpectations() ([]APIExpectation, error) {
	ctx := context.Background()

	config, err := em.store.GetConfig(ctx, em.cleanName)
	if err != nil {
		return nil, err
	}

	var expectations []APIExpectation

	// Convert MockConfiguration expectations to APIExpectation format
	for i, exp := range config.Expectations {
		apiExp := APIExpectation{
			ID: exp.ID,
			Raw: map[string]interface{}{
				"httpRequest":  exp.HttpRequest,
				"httpResponse": exp.HttpResponse,
			},
		}

		// Add times if present
		if exp.Times != nil {
			times := map[string]interface{}{}
			if exp.Times.Unlimited {
				times["unlimited"] = true
			} else if exp.Times.RemainingTimes > 0 {
				times["remainingTimes"] = exp.Times.RemainingTimes
			}
			if len(times) > 0 {
				apiExp.Raw["times"] = times
			}
		}

		// Extract method and path for display
		if method, ok := exp.HttpRequest["method"].(string); ok {
			apiExp.Method = method
		}
		if path, ok := exp.HttpRequest["path"].(string); ok {
			apiExp.Path = path
		}

		// Extract name and description if available
		if name, ok := exp.HttpRequest["name"].(string); ok && name != "" {
			apiExp.Description = name // Use name as the main identifier
		} else if apiExp.Method == "" && apiExp.Path == "" {
			apiExp.Description = fmt.Sprintf("API %d", i+1)
		} else {
			apiExp.Description = fmt.Sprintf("%s %s", apiExp.Method, apiExp.Path)
		}

		expectations = append(expectations, apiExp)
	}

	return expectations, nil
}

func (em *ExpectationManager) buildAPIList(expectations []APIExpectation) []string {
	var apiList []string
	for _, exp := range expectations {
		// Get status code for display
		statusCode := "200"
		if httpResp, ok := exp.Raw["httpResponse"].(map[string]interface{}); ok {
			if status, ok := httpResp["statusCode"].(int); ok {
				statusCode = fmt.Sprintf("%d", status)
			} else if status, ok := httpResp["statusCode"].(float64); ok {
				statusCode = fmt.Sprintf("%.0f", status)
			}
		}

		// Get method and path
		method := exp.Method
		path := exp.Path
		if method == "" {
			method = "?"
		}
		if path == "" {
			path = "?"
		}

		// Build unique display string: METHOD path [query_params] (status)
		var displayName string

		// Get query parameters for display
		queryInfo := ""
		if httpReq, ok := exp.Raw["httpRequest"].(map[string]interface{}); ok {
			if params, ok := httpReq["queryStringParameters"].(map[string]interface{}); ok && len(params) > 0 {
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
		}

		// Use name from httpRequest if available, otherwise use method+path
		name := ""
		if httpReq, ok := exp.Raw["httpRequest"].(map[string]interface{}); ok {
			if reqName, ok := httpReq["name"].(string); ok && reqName != "" {
				name = reqName
			}
		}

		if name != "" && queryInfo != "" {
			// Name + query params (e.g., "NEW [?type=New] (200)")
			displayName = fmt.Sprintf("%s%s (%s)", name, queryInfo, statusCode)
		} else if name != "" {
			// Just name (e.g., "NEW (200)")
			displayName = fmt.Sprintf("%s (%s)", name, statusCode)
		} else {
			// Method + path + query params (e.g., "GET /api/v1/mobile [?type=New] (200)")
			displayName = fmt.Sprintf("%s %s%s (%s)", method, path, queryInfo, statusCode)
		}

		apiList = append(apiList, displayName)
	}
	return apiList
}

func (em *ExpectationManager) getDistinguishingInfo(exp APIExpectation) string {
	// Try to get meaningful distinguishing information
	var infoParts []string

	// Check for query parameters
	if httpReq, ok := exp.Raw["httpRequest"].(map[string]interface{}); ok {
		if params, ok := httpReq["queryStringParameters"].(map[string]interface{}); ok && len(params) > 0 {
			var paramNames []string
			for key := range params {
				paramNames = append(paramNames, key)
			}
			if len(paramNames) > 0 {
				infoParts = append(infoParts, fmt.Sprintf("?%s", strings.Join(paramNames, "&")))
			}
		}
	}

	// Check for specific headers
	if httpResp, ok := exp.Raw["httpResponse"].(map[string]interface{}); ok {
		if headers, ok := httpResp["headers"].(map[string]interface{}); ok {
			if contentType, ok := headers["Content-Type"]; ok {
				if ct, ok := contentType.(string); ok && ct != "application/json" {
					infoParts = append(infoParts, ct)
				}
			}
		}

		// Check response body for hints
		if body, ok := httpResp["body"].(string); ok && len(body) > 0 {
			// Try to extract meaningful info from JSON body
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(body), &jsonData); err == nil {
				if msg, ok := jsonData["message"].(string); ok && len(msg) < 50 {
					infoParts = append(infoParts, msg)
				} else if errMsg, ok := jsonData["error"].(string); ok && len(errMsg) < 50 {
					infoParts = append(infoParts, errMsg)
				} else if success, ok := jsonData["success"].(bool); ok {
					if success {
						infoParts = append(infoParts, "Success")
					} else {
						infoParts = append(infoParts, "Error")
					}
				}
			} else {
				// For non-JSON, show first few characters
				if len(body) > 30 {
					infoParts = append(infoParts, body[:30]+"...")
				} else {
					infoParts = append(infoParts, body)
				}
			}
		}
	}

	// Return joined info or empty string
	if len(infoParts) > 0 {
		return strings.Join(infoParts, " | ")
	}
	return ""
}

func (em *ExpectationManager) findExpectationIndex(apiList []string, selected string) int {
	for i, api := range apiList {
		if api == selected {
			return i
		}
	}
	return -1
}

func (em *ExpectationManager) editSingleExpectation(expectation *APIExpectation) error {
	fmt.Printf("\n📝 Editing: %s\n", expectation.Description)
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
			if err := em.editMethod(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "path":
			if err := em.editPath(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "status":
			if err := em.editStatusCode(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "body":
			if err := em.editResponseBody(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "headers":
			if err := em.editHeaders(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "query":
			if err := em.editQueryParams(expectation); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "view":
			em.viewCurrentConfig(expectation)
		case "done":
			return nil
		}
	}
}

// Edit helper methods

func (em *ExpectationManager) editMethod(expectation *APIExpectation) error {
	currentMethod := expectation.Method

	var newMethod string
	if err := survey.AskOne(&survey.Select{
		Message: "Select HTTP method:",
		Options: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"},
		Default: currentMethod,
	}, &newMethod); err != nil {
		return err
	}

	if httpReq, ok := expectation.Raw["httpRequest"].(map[string]interface{}); ok {
		httpReq["method"] = newMethod
		expectation.Method = newMethod
		expectation.Description = fmt.Sprintf("%s %s", newMethod, expectation.Path)
		fmt.Printf("✅ Updated method to %s\n", newMethod)
	}
	return nil
}

func (em *ExpectationManager) editPath(expectation *APIExpectation) error {
	currentPath := expectation.Path

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

	if httpReq, ok := expectation.Raw["httpRequest"].(map[string]interface{}); ok {
		httpReq["path"] = newPath
		expectation.Path = newPath
		expectation.Description = fmt.Sprintf("%s %s", expectation.Method, newPath)
		fmt.Printf("✅ Updated path to %s\n", newPath)
	}
	return nil
}

func (em *ExpectationManager) editStatusCode(expectation *APIExpectation) error {
	currentStatus := 200
	if httpResp, ok := expectation.Raw["httpResponse"].(map[string]interface{}); ok {
		if status, ok := httpResp["statusCode"].(int); ok {
			currentStatus = status
		} else if status, ok := httpResp["statusCode"].(float64); ok {
			currentStatus = int(status)
		}
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

	if httpResp, ok := expectation.Raw["httpResponse"].(map[string]interface{}); ok {
		httpResp["statusCode"] = newStatus
		fmt.Printf("✅ Updated status code to %d\n", newStatus)
	}
	return nil
}

func (em *ExpectationManager) editResponseBody(expectation *APIExpectation) error {
	currentBody := ""
	if httpResp, ok := expectation.Raw["httpResponse"].(map[string]interface{}); ok {
		if body, ok := httpResp["body"].(string); ok {
			currentBody = body
		}
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
		return em.editBodyAsText(expectation, currentBody)
	case "json":
		return em.editBodyAsJSON(expectation, currentBody)
	case "template":
		return em.editBodyWithTemplate(expectation)
	}
	return nil
}

func (em *ExpectationManager) editBodyAsText(expectation *APIExpectation, currentBody string) error {
	var newBody string
	if err := survey.AskOne(&survey.Multiline{
		Message: "Enter response body:",
		Default: currentBody,
		Help:    "Enter the raw response body content",
	}, &newBody); err != nil {
		return err
	}

	if httpResp, ok := expectation.Raw["httpResponse"].(map[string]interface{}); ok {
		httpResp["body"] = newBody
		fmt.Printf("✅ Updated response body\n")
	}
	return nil
}

func (em *ExpectationManager) editBodyAsJSON(expectation *APIExpectation, currentBody string) error {
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

	if httpResp, ok := expectation.Raw["httpResponse"].(map[string]interface{}); ok {
		httpResp["body"] = newBody
		fmt.Printf("✅ Updated JSON response body\n")
	}
	return nil
}

func (em *ExpectationManager) editBodyWithTemplate(expectation *APIExpectation) error {
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

	if httpResp, ok := expectation.Raw["httpResponse"].(map[string]interface{}); ok {
		httpResp["body"] = templateBody
		fmt.Printf("✅ Applied %s template\n", strings.Split(template, " ")[0])
	}
	return nil
}

func (em *ExpectationManager) editHeaders(expectation *APIExpectation) error {
	currentHeaders := make(map[string]string)
	if httpResp, ok := expectation.Raw["httpResponse"].(map[string]interface{}); ok {
		if headers, ok := httpResp["headers"].(map[string]interface{}); ok {
			for k, v := range headers {
				if strVal, ok := v.(string); ok {
					currentHeaders[k] = strVal
				}
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
			if err := em.addHeader(expectation, currentHeaders); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "view":
			em.viewHeaders(currentHeaders)
		case "done":
			return nil
		default:
			if strings.Contains(actionParts[0], ":") {
				parts := strings.Split(actionParts[0], ":")
				if len(parts) == 2 {
					headerAction := parts[0]
					headerKey := parts[1]

					if headerAction == "edit" {
						if err := em.editHeaderValue(expectation, currentHeaders, headerKey); err != nil {
							fmt.Printf("❌ Error: %v\n", err)
						}
					} else if headerAction == "delete" {
						delete(currentHeaders, headerKey)
						em.updateHeaders(expectation, currentHeaders)
						fmt.Printf("✅ Deleted header %s\n", headerKey)
					}
				}
			}
		}
	}
}

func (em *ExpectationManager) editQueryParams(expectation *APIExpectation) error {
	currentParams := make(map[string]interface{})
	if httpReq, ok := expectation.Raw["httpRequest"].(map[string]interface{}); ok {
		if params, ok := httpReq["queryStringParameters"].(map[string]interface{}); ok {
			currentParams = params
		}
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
			if err := em.addQueryParam(expectation, currentParams); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		case "view":
			em.viewQueryParams(currentParams)
		case "done":
			return nil
		default:
			if strings.Contains(actionParts[0], ":") {
				parts := strings.Split(actionParts[0], ":")
				if len(parts) == 2 {
					paramAction := parts[0]
					paramKey := parts[1]

					if paramAction == "edit" {
						if err := em.editQueryParamValue(expectation, currentParams, paramKey); err != nil {
							fmt.Printf("❌ Error: %v\n", err)
						}
					} else if paramAction == "delete" {
						delete(currentParams, paramKey)
						em.updateQueryParams(expectation, currentParams)
						fmt.Printf("✅ Deleted parameter %s\n", paramKey)
					}
				}
			}
		}
	}
}

func (em *ExpectationManager) viewCurrentConfig(expectation *APIExpectation) {
	fmt.Println("\n📋 Current Configuration:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	jsonBytes, _ := json.MarshalIndent(expectation.Raw, "", "  ")
	fmt.Printf("%s\n\n", string(jsonBytes))
}

// Helper methods

func (em *ExpectationManager) addHeader(expectation *APIExpectation, currentHeaders map[string]string) error {
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
	em.updateHeaders(expectation, currentHeaders)
	fmt.Printf("✅ Added header %s\n", headerName)
	return nil
}

func (em *ExpectationManager) editHeaderValue(expectation *APIExpectation, currentHeaders map[string]string, headerKey string) error {
	currentValue := currentHeaders[headerKey]

	var newValue string
	if err := survey.AskOne(&survey.Input{
		Message: fmt.Sprintf("New value for %s:", headerKey),
		Default: currentValue,
	}, &newValue); err != nil {
		return err
	}

	currentHeaders[headerKey] = newValue
	em.updateHeaders(expectation, currentHeaders)
	fmt.Printf("✅ Updated header %s\n", headerKey)
	return nil
}

func (em *ExpectationManager) updateHeaders(expectation *APIExpectation, headers map[string]string) {
	if httpResp, ok := expectation.Raw["httpResponse"].(map[string]interface{}); ok {
		headerMap := make(map[string]interface{})
		for k, v := range headers {
			headerMap[k] = v
		}
		httpResp["headers"] = headerMap
	}
}

func (em *ExpectationManager) viewHeaders(headers map[string]string) {
	fmt.Println("\nCurrent headers:")
	for k, v := range headers {
		fmt.Printf("  %s: %s\n", k, v)
	}
	fmt.Println()
}

func (em *ExpectationManager) addQueryParam(expectation *APIExpectation, currentParams map[string]interface{}) error {
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
	em.updateQueryParams(expectation, currentParams)
	fmt.Printf("✅ Added parameter %s\n", paramName)
	return nil
}

func (em *ExpectationManager) editQueryParamValue(expectation *APIExpectation, currentParams map[string]interface{}, paramKey string) error {
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
	em.updateQueryParams(expectation, currentParams)
	fmt.Printf("✅ Updated parameter %s\n", paramKey)
	return nil
}

func (em *ExpectationManager) updateQueryParams(expectation *APIExpectation, params map[string]interface{}) {
	if httpReq, ok := expectation.Raw["httpRequest"].(map[string]interface{}); ok {
		httpReq["queryStringParameters"] = params
	}
}

func (em *ExpectationManager) viewQueryParams(params map[string]interface{}) {
	fmt.Println("\nCurrent query parameters:")
	for k, v := range params {
		fmt.Printf("  %s: %v\n", k, v)
	}
	fmt.Println()
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

func (em *ExpectationManager) saveExpectations(expectations []APIExpectation) error {
	// Convert APIExpectation back to MockConfiguration format
	mockConfig := &state.MockConfiguration{
		Metadata: state.ConfigMetadata{
			ProjectID:   em.cleanName,
			Version:     fmt.Sprintf("v%d", time.Now().Unix()),
			UpdatedAt:   time.Now(),
			Description: "Updated via expectation manager",
			Provider:    "expectation-manager",
		},
		Expectations: make([]state.MockExpectation, 0, len(expectations)),
	}

	// Convert expectations
	for i, apiExp := range expectations {
		mockExp := state.MockExpectation{
			ID:       apiExp.ID,
			Priority: len(expectations) - i, // Higher priority for earlier expectations
		}

		// Extract httpRequest and httpResponse from Raw
		if httpReq, ok := apiExp.Raw["httpRequest"].(map[string]interface{}); ok {
			mockExp.HttpRequest = httpReq
		}
		if httpResp, ok := apiExp.Raw["httpResponse"].(map[string]interface{}); ok {
			mockExp.HttpResponse = httpResp
		}

		// Extract times if present
		if times, ok := apiExp.Raw["times"].(map[string]interface{}); ok {
			mockExp.Times = &state.ExpectationTimes{}
			if unlimited, ok := times["unlimited"].(bool); ok {
				mockExp.Times.Unlimited = unlimited
			}
			if remaining, ok := times["remainingTimes"].(float64); ok {
				mockExp.Times.RemainingTimes = int(remaining)
			} else if remaining, ok := times["remainingTimes"].(int); ok {
				mockExp.Times.RemainingTimes = remaining
			}
		}

		mockConfig.Expectations = append(mockConfig.Expectations, mockExp)
	}

	// Save using the store's UpdateConfig method
	ctx := context.Background()
	if err := em.store.UpdateConfig(ctx, em.cleanName, mockConfig); err != nil {
		return fmt.Errorf("failed to save expectations: %w", err)
	}

	fmt.Printf("💾 Expectations saved successfully!\n")
	return nil
}

func (em *ExpectationManager) filterExpectations(expectations []APIExpectation, toRemove []string) []APIExpectation {
	var filtered []APIExpectation

	// Build the same display list to match against
	displayList := em.buildAPIList(expectations)

	for i, exp := range expectations {
		shouldRemove := false
		expectedDisplay := displayList[i] // Get the corresponding display string

		for _, remove := range toRemove {
			if expectedDisplay == remove {
				shouldRemove = true
				break
			}
		}

		if !shouldRemove {
			filtered = append(filtered, exp)
		}
	}

	return filtered
}
