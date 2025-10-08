// internal/terraform/integration.go
package terraform

import (
	"fmt"
	"strings"

	"github.com/hemantobora/auto-mock/internal/utils"
)

// ProjectConfig holds configuration for a project deployment
type ProjectConfig struct {
	ProjectName       string
	AWSProfile        string
	DeploymentOptions *DeploymentOptions
}

// CreateProjectConfig creates a project configuration from project name and profile
func CreateProjectConfig(projectName, awsProfile string) *ProjectConfig {
	cleanName := utils.ExtractUserProjectName(projectName)

	return &ProjectConfig{
		ProjectName:       cleanName,
		AWSProfile:        awsProfile,
		DeploymentOptions: DefaultDeploymentOptions(),
	}
}

// DeployInfrastructure deploys the complete AutoMock infrastructure
func DeployInfrastructure(config *ProjectConfig) (*InfrastructureOutputs, error) {
	// Check if Terraform is installed
	if err := CheckTerraformInstalled(); err != nil {
		return nil, err
	}

	// Create Terraform manager
	manager := NewManager(config.ProjectName, config.AWSProfile)

	// Deploy infrastructure
	outputs, err := manager.Deploy(config.DeploymentOptions)
	if err != nil {
		return nil, fmt.Errorf("infrastructure deployment failed: %w", err)
	}

	// Display deployment summary
	displayDeploymentSummary(config, outputs)

	return outputs, nil
}

// DestroyInfrastructure removes the complete AutoMock infrastructure
func DestroyInfrastructure(projectName, awsProfile string) error {
	// Check if Terraform is installed
	if err := CheckTerraformInstalled(); err != nil {
		return err
	}

	// Create Terraform manager
	manager := NewManager(projectName, awsProfile)

	// Destroy infrastructure
	return manager.Destroy()
}

// displayDeploymentSummary shows a formatted summary of the deployment
func displayDeploymentSummary(config *ProjectConfig, outputs *InfrastructureOutputs) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("🎉 INFRASTRUCTURE DEPLOYMENT COMPLETE\n")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Printf("📂 Project: %s\n", config.ProjectName)
	fmt.Printf("☁️  Cloud Provider: AWS\n")

	if outputs.MockServerURL != "" {
		fmt.Printf("🔗 API Endpoint: %s\n", outputs.MockServerURL)
	}

	if outputs.DashboardURL != "" {
		fmt.Printf("📊 Dashboard: %s\n", outputs.DashboardURL)
	}

	if outputs.ConfigBucket != "" {
		fmt.Printf("🪣 Configuration Bucket: %s\n", outputs.ConfigBucket)
	}

	// Display TTL information
	if config.DeploymentOptions.EnableTTLCleanup && config.DeploymentOptions.TTLHours > 0 {
		fmt.Printf("⏰ Auto-cleanup: %d hours\n", config.DeploymentOptions.TTLHours)
	}

	fmt.Println("\n" + strings.Repeat("-", 70))
	fmt.Println("📋 NEXT STEPS:")
	fmt.Println(strings.Repeat("-", 70))

	if outputs.CLICommands != nil {
		if uploadCmd, ok := outputs.CLICommands["upload_config"]; ok {
			fmt.Printf("1. Upload your expectations:\n   %s\n", uploadCmd)
		}

		if reloadCmd, ok := outputs.CLICommands["reload_service"]; ok {
			fmt.Printf("2. Reload MockServer:\n   %s\n", reloadCmd)
		}

		if viewCmd, ok := outputs.CLICommands["view_logs"]; ok {
			fmt.Printf("3. Monitor logs:\n   %s\n", viewCmd)
		}
	}

	fmt.Println("\n💡 Your infrastructure is ready! Continue with mock configuration generation.")
	fmt.Println(strings.Repeat("=", 70) + "\n")
}

// GetProjectInfrastructureInfo retrieves information about existing infrastructure
func GetProjectInfrastructureInfo(projectName, awsProfile string) (*InfrastructureOutputs, error) {
	// Check if Terraform is installed
	if err := CheckTerraformInstalled(); err != nil {
		return nil, err
	}

	// Create Terraform manager
	manager := NewManager(projectName, awsProfile)

	// Try to get outputs from existing infrastructure
	outputs, err := manager.getOutputs()
	if err != nil {
		return nil, fmt.Errorf("failed to get infrastructure info: %w", err)
	}

	return outputs, nil
}

// ValidateInfrastructureState checks if infrastructure is properly deployed
func ValidateInfrastructureState(projectName, awsProfile string) (bool, error) {
	outputs, err := GetProjectInfrastructureInfo(projectName, awsProfile)
	if err != nil {
		return false, nil // Infrastructure doesn't exist or is broken
	}

	// Check if essential components exist
	hasAPI := outputs.MockServerURL != ""
	hasBucket := outputs.ConfigBucket != ""

	return hasAPI && hasBucket, nil
}
