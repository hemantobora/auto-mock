package repl

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/builders"
)

// StartInteractiveBuilder starts the 7-step interactive mock expectation builder
func generateInteractiveWithMenu(projectName string) (string, error) {
	fmt.Println("🔧 Interactive Builder (7-Step Process)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📝 Step-by-step endpoint building with validation")
	fmt.Println()

	// Step 0: Choose API Type (REST vs GraphQL)
	apiType, err := chooseAPIType()
	if err != nil {
		return "", err
	}

	var expectations []builders.MockExpectation

	// Build expectations based on API type
	for {
		var expectation builders.MockExpectation

		switch apiType {
		case "REST":
			expectation, err = buildRESTExpectation()
		case "GraphQL":
			expectation, err = buildGraphQLExpectation()
		default:
			return "", fmt.Errorf("unsupported API type: %s", apiType)
		}

		if err != nil {
			return "", fmt.Errorf("failed to build expectation: %w", err)
		}

		expectations = append(expectations, expectation)

		// Ask if user wants to add more expectations
		var addMore bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Add another expectation?",
			Default: false,
		}, &addMore); err != nil {
			return "", err
		}

		if !addMore {
			break
		}
	}

	// Convert to MockServer JSON
	mockServerJSON := builders.ExpectationsToMockServerJSON(expectations)
	return mockServerJSON, nil
}

// chooseAPIType lets user choose between REST and GraphQL
func chooseAPIType() (string, error) {
	var apiType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select API type:",
		Options: []string{
			"REST - Traditional REST API",
			"GraphQL - GraphQL API",
		},
		Default: "REST - Traditional REST API",
	}, &apiType); err != nil {
		return "", err
	}

	return strings.Split(apiType, " ")[0], nil
}

// buildRESTExpectation builds a single REST expectation using 7-step process
func buildRESTExpectation() (builders.MockExpectation, error) {
	fmt.Println("\n📡 Creating REST Expectation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Delegate to REST builder
	return builders.BuildRESTExpectation()
}

// buildGraphQLExpectation builds a single GraphQL expectation using 7-step process
func buildGraphQLExpectation() (builders.MockExpectation, error) {
	fmt.Println("\n🔗 Creating GraphQL Expectation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Delegate to GraphQL builder
	return builders.BuildGraphQLExpectation()
}
