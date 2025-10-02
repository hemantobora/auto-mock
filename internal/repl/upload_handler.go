// internal/repl/upload_handler.go
// Handles file upload with unified deployment menu
package repl

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/builders"
)

// configureUploadedExpectationWithMenu handles file upload with deployment menu
func configureUploadedExpectationWithMenu(projectName string) (string, error) {
	fmt.Println("📤 Expectation Import")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━")

	var filePath string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter path to expectation json file:",
	}, &filePath); err != nil {
		return "", err
	}

	// Validate file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", filePath)
	}

	fmt.Printf("\n📄 Parsing expectation file: %s\n", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Unmarshal into builders.MockExpectation
	var expectations []builders.MockExpectation
	if err := json.Unmarshal(data, &expectations); err != nil {
		return "", fmt.Errorf("invalid expectation JSON format: %w", err)
	}

	if len(expectations) == 0 {
		return "", fmt.Errorf("no expectations found in file")
	}

	// Validate
	validCount := 0
	for i, exp := range expectations {
		if exp.Method != "" && exp.Path != "" && exp.StatusCode != 0 {
			validCount++
		} else {
			fmt.Printf("⚠️  Warning: Expectation %d is missing required fields\n", i+1)
		}
	}

	if validCount == 0 {
		return "", fmt.Errorf("no valid expectations found")
	}

	fmt.Printf("✅ Found %d expectation(s) in file (%d valid)\n", len(expectations), validCount)

	mockServerJSON := builders.ExpectationsToMockServerJSON(expectations)
	return mockServerJSON, nil
}
