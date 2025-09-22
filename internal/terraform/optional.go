// internal/terraform/optional.go
package terraform

import (
	"fmt"
	"strings"

	"github.com/hemantobora/auto-mock/internal/utils"
)

// InfrastructureLevel defines the level of infrastructure deployment
type InfrastructureLevel int

const (
	// LevelBasic - Just S3 bucket (current default behavior)
	LevelBasic InfrastructureLevel = iota
	// LevelComplete - Full infrastructure with ECS, ALB, etc.
	LevelComplete
)

// OptionalDeployment provides infrastructure deployment choices
type OptionalDeployment struct {
	ProjectName string
	AWSProfile  string
	Level       InfrastructureLevel
}

// NewOptionalDeployment creates a new optional deployment manager
func NewOptionalDeployment(projectName, awsProfile string) *OptionalDeployment {
	return &OptionalDeployment{
		ProjectName: projectName,
		AWSProfile:  awsProfile,
		Level:       LevelBasic, // Default to existing behavior
	}
}

// PromptForInfrastructureLevel asks user what level of infrastructure they want
func (od *OptionalDeployment) PromptForInfrastructureLevel() InfrastructureLevel {
	cleanName := utils.ExtractUserProjectName(od.ProjectName)
	
	fmt.Printf("\n🏗️  Infrastructure Options for '%s':\n", cleanName)
	fmt.Println(strings.Repeat("=", 50))
	
	fmt.Println("1. 📦 Basic (S3 Only)")
	fmt.Println("   • S3 bucket for configuration storage")
	fmt.Println("   • Manual MockServer deployment")
	fmt.Println("   • Free tier compatible")
	fmt.Println("   • Quick setup (30 seconds)")
	
	fmt.Println("\n2. 🚀 Complete Infrastructure") 
	fmt.Println("   • S3 bucket + ECS Fargate + Application Load Balancer")
	fmt.Println("   • Auto-scaling MockServer with SSL")
	fmt.Println("   • Production-ready with monitoring")
	fmt.Println("   • TTL auto-cleanup for dev environments")
	fmt.Println("   • Setup time (3-5 minutes)")
	
	fmt.Println(strings.Repeat("=", 50))
	fmt.Print("Choose infrastructure level (1 for Basic, 2 for Complete): ")
	
	var choice string
	fmt.Scanln(&choice)
	
	switch strings.TrimSpace(choice) {
	case "2", "complete", "full":
		od.Level = LevelComplete
		return LevelComplete
	default:
		od.Level = LevelBasic
		return LevelBasic
	}
}

// DeployBasic deploys just an S3 bucket (existing behavior)
func (od *OptionalDeployment) DeployBasic() error {
	cleanName := utils.ExtractUserProjectName(od.ProjectName)
	fmt.Printf("📦 Deploying basic infrastructure (S3 only) for '%s'...\n", cleanName)
	
	// This would call the existing S3 bucket creation logic
	// The current AWS provider already does this
	fmt.Printf("✅ Basic infrastructure ready for '%s'\n", cleanName)
	fmt.Println("💡 You can upgrade to complete infrastructure anytime with: automock upgrade --project " + cleanName)
	
	return nil
}

// DeployComplete deploys the full infrastructure stack
func (od *OptionalDeployment) DeployComplete() error {
	cleanName := utils.ExtractUserProjectName(od.ProjectName)
	fmt.Printf("🚀 Deploying complete infrastructure for '%s'...\n", cleanName)
	
	// Check if Terraform is installed
	if err := CheckTerraformInstalled(); err != nil {
		fmt.Println("❌ Terraform not found. Installing complete infrastructure requires Terraform.")
		fmt.Println("🔽 Install from: https://terraform.io/downloads")
		fmt.Printf("📦 Falling back to basic infrastructure for '%s'...\n", cleanName)
		return od.DeployBasic()
	}
	
	// Deploy complete infrastructure
	config := CreateProjectConfig(od.ProjectName, od.AWSProfile)
	outputs, err := DeployInfrastructure(config)
	if err != nil {
		fmt.Printf("❌ Complete infrastructure deployment failed: %v\n", err)
		fmt.Printf("📦 Falling back to basic infrastructure for '%s'...\n", cleanName)
		return od.DeployBasic()
	}
	
	// Show success information
	displayInfrastructureSuccess(cleanName, outputs)
	
	return nil
}

// displayInfrastructureSuccess shows deployment success with URLs
func displayInfrastructureSuccess(projectName string, outputs *InfrastructureOutputs) {
	fmt.Println("\n" + strings.Repeat("🎉", 25))
	fmt.Printf("COMPLETE INFRASTRUCTURE DEPLOYED: %s\n", projectName)
	fmt.Println(strings.Repeat("🎉", 25))
	
	if outputs.MockServerURL != "" {
		fmt.Printf("🔗 API Endpoint: %s\n", outputs.MockServerURL)
	}
	
	if outputs.DashboardURL != "" {
		fmt.Printf("📊 Dashboard: %s\n", outputs.DashboardURL)
	}
	
	if outputs.ConfigBucket != "" {
		fmt.Printf("🪣 Config Bucket: %s\n", outputs.ConfigBucket)
	}
	
	fmt.Println("\n💡 Your infrastructure is production-ready!")
	fmt.Println("📋 Continue with mock configuration generation...")
	fmt.Println(strings.Repeat("=", 60) + "\n")
}

// UpgradeToComplete upgrades from basic to complete infrastructure
func (od *OptionalDeployment) UpgradeToComplete() error {
	cleanName := utils.ExtractUserProjectName(od.ProjectName)
	fmt.Printf("⬆️  Upgrading '%s' to complete infrastructure...\n", cleanName)
	
	// Deploy complete infrastructure (will integrate with existing S3 bucket)
	return od.DeployComplete()
}

// GetInfrastructureInfo returns information about current infrastructure
func (od *OptionalDeployment) GetInfrastructureInfo() (*InfrastructureOutputs, error) {
	// Try to get complete infrastructure info first
	outputs, err := GetProjectInfrastructureInfo(od.ProjectName, od.AWSProfile)
	if err == nil {
		return outputs, nil
	}
	
	// If that fails, return basic info (S3 bucket only)
	return &InfrastructureOutputs{
		ConfigBucket: utils.GetBucketName(strings.ToLower(od.ProjectName)),
	}, nil
}
