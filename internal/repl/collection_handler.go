// internal/repl/collection_handler.go
// Handles collection imports with unified deployment menu
package repl

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/collections"
)

// generateFromCollectionWithMenu imports from API collections with deployment menu
func generateFromCollectionWithMenu(projectName string) (string, error) {
	fmt.Println("📂 Collection Import")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Get collection type
	var collectionType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select collection type:",
		Options: []string{"postman", "bruno", "insomnia"},
	}, &collectionType); err != nil {
		return "", err
	}

	// Get file path
	var filePath string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter path to collection file:",
	}, &filePath); err != nil {
		return "", err
	}

	// Process the collection - this saves to Storage bucket
	processor, err := collections.NewCollectionProcessor(projectName, collectionType)
	if err != nil {
		return "", fmt.Errorf("failed to create collection processor: %w", err)
	}

	return processor.ProcessCollection(filePath)
}

// handleCollectionMode processes collection files with AI assistance
func HandleCollectionMode(collectionType, collectionFile, projectName string) (string, error) {
	fmt.Printf("📂 Processing %s collection for project: %s\n", collectionType, projectName)

	// Validate collection parameters
	if collectionType == "" {
		return "", fmt.Errorf("collection-type is required when using collection-file")
	}

	// Create collection processor
	processor, err := collections.NewCollectionProcessor(projectName, collectionType)
	if err != nil {
		return "", fmt.Errorf("failed to create collection processor: %w", err)
	}

	// Process the collection using the full workflow
	return processor.ProcessCollection(collectionFile)

}
