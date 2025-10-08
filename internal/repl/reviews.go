package repl

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/state"
	"github.com/hemantobora/auto-mock/internal/utils"
)

// Handle final result
func handleFinalResult(mockServerJSON, projectName string) error {
	for {
		var action string
		if err := survey.AskOne(&survey.Select{
			Message: "What would you like to do with this configuration?",
			Options: []string{
				"save - Save the expectation file",
				"view-json - View full JSON configuration",
				"deploy - Deploy complete infrastructure (ECS + ALB)",
				"local - Start MockServer locally",
				"exit - Exit without saving",
			},
		}, &action); err != nil {
			return err
		}

		action = strings.Split(action, " ")[0]
		switch action {
		case "save":
			return saveToFile(mockServerJSON, projectName)
		case "deploy":
			// First save expectations to S3
			if err := saveToFile(mockServerJSON, projectName); err != nil {
				return fmt.Errorf("failed to save expectations: %w", err)
			}
			// Then deploy infrastructure
			return deployInfrastructureWithTerraform(projectName, "")
		case "local":
			return startLocalMockServer(mockServerJSON, projectName)
		case "view-json":
			fmt.Printf("\n📋 Full JSON Configuration:\n%s\n\n", mockServerJSON)
			continue
		case "exit":
			fmt.Println("\n⚠️  Are you sure you want to exit without saving?")
			fmt.Println("   • The uploaded expectations will not be saved")
			var confirmExit bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Exit without saving?",
				Default: false,
			}, &confirmExit); err != nil {
				return err
			}
			if confirmExit {
				return fmt.Errorf("user cancelled upload")
			}
			continue
		}
		return nil
	}
}

// Save configuration to S3
func saveToFile(mockServerJSON, projectName string) error {
	ctx := context.Background()
	cleanName := utils.ExtractUserProjectName(projectName)

	fmt.Println("\n☁️ Saving expectation file to your cloud storage")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Initialize S3 store using factory
	store, err := state.StoreForProject(ctx, projectName)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	// Parse MockServer JSON to MockConfiguration format
	mockConfig, err := state.ParseMockServerJSON(mockServerJSON)
	if err != nil {
		return fmt.Errorf("failed to parse mock server JSON: %w", err)
	}

	// Set metadata
	mockConfig.Metadata.ProjectID = cleanName
	mockConfig.Metadata.Provider = "auto-mock-cli"
	mockConfig.Metadata.Description = "Generated via interactive mock generation"
	mockConfig.Metadata.Version = fmt.Sprintf("v%d", time.Now().Unix())
	mockConfig.Metadata.CreatedAt = time.Now()
	mockConfig.Metadata.UpdatedAt = time.Now()

	// Save to S3
	if err := store.SaveConfig(ctx, cleanName, mockConfig); err != nil {
		return fmt.Errorf("failed to save to S3: %w", err)
	}

	fmt.Printf("\n✅ MockServer configuration saved to cloud storage!\n")
	fmt.Printf("📁 Project: %s\n", cleanName)
	fmt.Printf("🔗 Configuration stored for team access\n")
	return nil
}

// Start local MockServer
func startLocalMockServer(mockServerJSON, projectName string) error {
	fmt.Println("\n🚀 Local MockServer Setup")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	cleanName := utils.ExtractUserProjectName(projectName)
	configFile := fmt.Sprintf("%s-expectations.json", cleanName)

	if err := os.WriteFile(configFile, []byte(mockServerJSON), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✅ Configuration saved as: %s\n\n", configFile)
	fmt.Println("🐳 Docker commands:")
	fmt.Println("1. Start MockServer:")
	fmt.Println("   docker run -d -p 1080:1080 -p 1090:1090 mockserver/mockserver:5.15.0")
	fmt.Println("2. Load expectations:")
	fmt.Printf("   curl -X PUT http://localhost:1080/mockserver/expectation -d @%s\n", configFile)
	fmt.Println("3. Access your API: http://localhost:1080")
	fmt.Println("4. View dashboard: http://localhost:1080/mockserver/dashboard")
	return nil
}
