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

		options = append(options, apiList...)

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
			for _, nv := range exp.HttpRequest.QueryStringParameters {
				queryParts = append(queryParts, fmt.Sprintf("%s=%s", nv.Name, strings.Join(nv.Values, ",")))
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
	fmt.Println("   • Storage bucket and contents")
	fmt.Println("   • Any running infrastructure")
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
	// Determine default value: 0 for unlimited or unset, otherwise current remaining times
	defaultTimes := 0
	if expectation.Times != nil && !expectation.Times.Unlimited {
		defaultTimes = expectation.Times.RemainingTimes
	}
	if err := survey.AskOne(&survey.Input{
		Message: "Enter number of times this expectation should be matched (0 = unlimited):",
		Default: fmt.Sprintf("%d", defaultTimes),
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
	mc := &builders.MockConfigurator{}
	mc.CollectPathMatchingStrategy(expectation)
	fmt.Printf("✅ Updated path to %s\n", expectation.HttpRequest.Path)
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
		Options: []string{"json - Edit as JSON", "template - Use JSON template", "view - View current body"},
	}, &editChoice); err == nil {
		editChoice = strings.Split(editChoice, " ")[0]
		switch editChoice {
		case "view":
			fmt.Printf("\nCurrent response body:\n%s\n\n", currentBody)
		case "json":
			editBodyAsJSON(expectation, currentBody)
		case "template":
			if err := builders.GenerateResponseTemplate(expectation); err != nil {
				fmt.Printf("❌ Failed to generate response template: %v\n", err)
			}
		}
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
			expectation.HttpResponse.Body = map[string]any{
				"type": "JSON",
				"json": jsonData,
			}
			fmt.Println("✅ Updated JSON response body")
		} else {
			fmt.Println("❌ Invalid JSON")
		}
	}
}

// editResponseHeaders lets users add/view/edit/delete response headers when
// HttpResponse.Headers is []NameValues.
//
// Assumptions:
//   - addResponseHeader(*models.MockExpectation) appends or updates Headers.
//   - viewResponseHeaders([]models.NameValues) pretty-prints current headers.
//   - editResponseHeaderValue(*models.MockExpectation, name string) updates the
//     Values for the first header with that name (case-insensitive).
func editResponseHeaders(expectation *models.MockExpectation) {
	// Initialize slice if nil
	if expectation.HttpResponse.Headers == nil {
		expectation.HttpResponse.Headers = []models.NameValues{}
	}
	editNameValuesList(&expectation.HttpResponse.Headers, "header")
}

// helper: find header index by name (case-insensitive)
func findNameIndex(items []models.NameValues, name string) int {
	for i, nv := range items {
		if strings.EqualFold(nv.Name, name) {
			return i
		}
	}
	return -1
}

// helper: split "a, b,  c" -> []string{"a","b","c"} (drop empties)
func parseCSVValues(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	// if user gave an empty string, preserve a single empty value
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// --- Generic NameValues editor helpers --------------------------------------

func editNameValuesList(list *[]models.NameValues, nounSingular string) {
	for {
		var action string

		options := []string{
			fmt.Sprintf("add - Add new %s", nounSingular),
			fmt.Sprintf("view - View current %ss", nounSingular),
		}
		for _, nv := range *list {
			options = append(options, fmt.Sprintf("edit:%s - Edit %s", nv.Name, nv.Name))
			options = append(options, fmt.Sprintf("delete:%s - Delete %s", nv.Name, nv.Name))
		}
		options = append(options, fmt.Sprintf("done - Finish editing %ss", nounSingular))

		if err := survey.AskOne(&survey.Select{Message: fmt.Sprintf("%s actions:", strings.Title(nounSingular)), Options: options}, &action); err != nil {
			return
		}

		token := strings.Fields(action)
		if len(token) == 0 {
			continue
		}
		actionToken := token[0]

		switch {
		case actionToken == "add":
			addNameValue(list, nounSingular)

		case actionToken == "view":
			viewNameValues(*list, nounSingular)

		case actionToken == "done":
			return

		case strings.Contains(actionToken, ":"):
			parts := strings.SplitN(actionToken, ":", 2)
			if len(parts) != 2 {
				continue
			}
			cmd, name := parts[0], parts[1]
			switch cmd {
			case "edit":
				editNameValue(list, name, nounSingular)
			case "delete":
				if idx := findNameIndex(*list, name); idx >= 0 {
					h := *list
					*list = append(h[:idx], h[idx+1:]...)
					fmt.Printf("✅ Deleted %s %s\n", nounSingular, name)
				} else {
					fmt.Printf("⚠️ %s %s not found\n", strings.Title(nounSingular), name)
				}
			}
		}
	}
}

func addNameValue(list *[]models.NameValues, noun string) {
	var name, valueCSV string
	if err := survey.AskOne(&survey.Input{Message: fmt.Sprintf("%s name:", strings.Title(noun))}, &name); err != nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if err := survey.AskOne(&survey.Input{Message: fmt.Sprintf("%s value (comma-separated for multiple):", strings.Title(noun))}, &valueCSV); err != nil {
		return
	}
	values := parseCSVValues(valueCSV)
	if *list == nil {
		*list = []models.NameValues{}
	}
	if idx := findNameIndex(*list, name); idx >= 0 {
		(*list)[idx].Values = values
		fmt.Printf("✅ Updated %s %s\n", noun, name)
		return
	}
	*list = append(*list, models.NameValues{Name: name, Values: values})
	fmt.Printf("✅ Added %s %s\n", noun, name)
}

func editNameValue(list *[]models.NameValues, name string, noun string) {
	if *list == nil {
		*list = []models.NameValues{}
	}
	idx := findNameIndex(*list, name)
	if idx < 0 {
		fmt.Printf("⚠️ %s %s not found; creating it.\n", strings.Title(noun), name)
		addNameValue(list, noun)
		return
	}
	defaultValue := strings.Join((*list)[idx].Values, ", ")
	var newVal string
	if err := survey.AskOne(&survey.Input{Message: fmt.Sprintf("New value for %s (comma-separated for multiple):", name), Default: defaultValue}, &newVal); err != nil {
		return
	}
	(*list)[idx].Values = parseCSVValues(newVal)
	fmt.Printf("✅ Updated %s %s\n", noun, name)
}

func viewNameValues(list []models.NameValues, noun string) {
	fmt.Printf("\nCurrent %ss:\n", noun)
	if len(list) == 0 {
		fmt.Println("  (none set)")
	} else {
		for _, nv := range list {
			fmt.Printf("  %s: %s\n", nv.Name, strings.Join(nv.Values, ", "))
		}
	}
	fmt.Println()
}

func editQueryParams(expectation *models.MockExpectation) {
	if expectation.HttpRequest.QueryStringParameters == nil {
		expectation.HttpRequest.QueryStringParameters = []models.NameValues{}
	}
	editNameValuesList(&expectation.HttpRequest.QueryStringParameters, "parameter")
}

// Removed legacy map-based query param editors; handled by generic NameValues editor.

func viewCurrentConfig(expectation *models.MockExpectation) {
	fmt.Println("\n📋 Current Configuration:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	jsonBytes, _ := json.MarshalIndent(expectation, "", "  ")
	fmt.Printf("%s\n\n", string(jsonBytes))
}
