// internal/terraform/display.go
package terraform

import (
	"fmt"
	"strings"
	"time"
)

// DisplayDeploymentResults shows a formatted summary of the infrastructure deployment
func DisplayDeploymentResults(outputs *InfrastructureOutputs, projectName string) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("  INFRASTRUCTURE DEPLOYMENT SUCCESSFUL")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Project Information
	fmt.Println("PROJECT DETAILS:")
	fmt.Printf("  Name:        %s\n", projectName)

	if _, ok := outputs.InfrastructureSummary["project"].(string); ok {
		fmt.Printf("  Region:      %s\n", outputs.InfrastructureSummary["region"])
	}
	fmt.Println()

	// Endpoints
	fmt.Println("ENDPOINTS:")
	fmt.Printf("  MockServer API:   %s\n", outputs.MockServerURL)
	fmt.Printf("  Dashboard:        %s\n", outputs.DashboardURL)
	fmt.Println()

	// Infrastructure Summary
	if compute, ok := outputs.InfrastructureSummary["compute"].(map[string]interface{}); ok {
		fmt.Println("COMPUTE RESOURCES:")
		fmt.Printf("  ECS Cluster:      %s\n", compute["cluster"])
		fmt.Printf("  ECS Service:      %s\n", compute["service"])
		fmt.Printf("  Instance Size:    %s\n", compute["instance_size"])
		fmt.Printf("  Task Count:       %v (min: %v, max: %v)\n",
			compute["current_tasks"], compute["min_tasks"], compute["max_tasks"])
		fmt.Println()
	}

	// Storage
	fmt.Println("STORAGE:")
	fmt.Printf("  Config Bucket:    %s\n", outputs.ConfigBucket)
	fmt.Println()

	// Quick Start Commands
	fmt.Println("QUICK START:")
	fmt.Println("  Health Check:")
	if cmd, ok := outputs.CLICommands["health_check"]; ok {
		fmt.Printf("    %s\n", cmd)
	}
	fmt.Println()
	fmt.Println("  View Expectations:")
	if cmd, ok := outputs.CLICommands["list_expectations"]; ok {
		fmt.Printf("    %s\n", cmd)
	}
	fmt.Println()

	// Management Commands
	fmt.Println("MANAGEMENT:")
	if integration, ok := outputs.IntegrationSummary["upload_command"].(string); ok {
		fmt.Println("  Upload new expectations:")
		fmt.Printf("    %s\n", integration)
		fmt.Println()
	}

	if cmd, ok := outputs.CLICommands["view_logs"]; ok {
		fmt.Println("  View logs:")
		fmt.Printf("    %s\n", cmd)
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
}

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

// DisplayStatusInfo shows current infrastructure status
func DisplayStatusInfo(outputs *InfrastructureOutputs) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("  INFRASTRUCTURE STATUS")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Basic info
	if summary, ok := outputs.InfrastructureSummary["compute"].(map[string]interface{}); ok {
		fmt.Printf("Cluster:      %s\n", summary["cluster"])
		fmt.Printf("Service:      %s\n", summary["service"])
		fmt.Printf("Tasks:        %v/%v running\n", summary["current_tasks"], summary["max_tasks"])
	}

	fmt.Printf("API Endpoint: %s\n", outputs.MockServerURL)
	fmt.Printf("Dashboard:    %s\n", outputs.DashboardURL)

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
}
