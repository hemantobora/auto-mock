// internal/terraform/display.go
package terraform

import (
	"fmt"
	"strings"
	"time"

	"github.com/hemantobora/auto-mock/internal/models"
)

type InfrastructureOutputs = models.InfrastructureOutputs

// DisplayDeploymentProgress shows progress during deployment
func DisplayDeploymentProgress(stage string, message string) {
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("[%s] %s: %s\n", timestamp, stage, message)
}

// DisplayDestroyConfirmation shows a confirmation prompt before destroying
func DisplayDestroyConfirmation(projectName string) {
	fmt.Println()
	fmt.Println(strings.Repeat("!", 80))
	fmt.Println("  WARNING: INFRASTRUCTURE DELETION")
	fmt.Println(strings.Repeat("!", 80))
	fmt.Println()
	fmt.Printf("You are about to PERMANENTLY DELETE all infrastructure for project: %s\n", projectName)
	fmt.Println()
	fmt.Println("This will delete:")
	fmt.Println("  - Application Service")
	fmt.Println("  - Application Load Balancer")
	fmt.Println("  - Networking Resources (if created)")
	fmt.Println("  - Storage Configuration (and all data)")
	fmt.Println("  - Application Logs")
	fmt.Println()
	fmt.Println("THIS ACTION CANNOT BE UNDONE!")
	fmt.Println()
}

// DisplayDestroyResults shows results after infrastructure destruction
func DisplayDestroyResults(projectName string, success bool) {
	fmt.Println()
	if success {
		fmt.Println(strings.Repeat("=", 80))
		fmt.Println("  INFRASTRUCTURE DESTROYED SUCCESSFULLY")
		fmt.Println(strings.Repeat("=", 80))
		fmt.Println()
		fmt.Printf("All infrastructure for project '%s' has been deleted.\n", projectName)
		fmt.Println()
	} else {
		fmt.Println(strings.Repeat("!", 80))
		fmt.Println("  INFRASTRUCTURE DESTRUCTION FAILED")
		fmt.Println(strings.Repeat("!", 80))
		fmt.Println()
		fmt.Println("Some resources may not have been deleted.")
		fmt.Println("Please check your AWS console and manually delete remaining resources.")
		fmt.Println()
	}
}

// DisplayTerraformVersion shows Terraform version info
func DisplayTerraformVersion(version string) {
	fmt.Printf("Using Terraform version: %s\n", version)
}

// DisplayValidationErrors shows validation errors in a user-friendly way
func DisplayValidationErrors(errors []string) {
	fmt.Println()
	fmt.Println(strings.Repeat("!", 80))
	fmt.Println("  VALIDATION ERRORS")
	fmt.Println(strings.Repeat("!", 80))
	fmt.Println()

	for i, err := range errors {
		fmt.Printf("%d. %s\n", i+1, err)
	}

	fmt.Println()
	fmt.Println("Please fix the above errors and try again.")
	fmt.Println()
}
