// internal/terraform/display.go
package terraform

import (
	"fmt"
	"strings"
)

// DisplayInfrastructureInfo shows detailed infrastructure information
func DisplayInfrastructureInfo(projectName string, outputs *InfrastructureOutputs) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("📊 INFRASTRUCTURE STATUS: %s\n", projectName)
	fmt.Println(strings.Repeat("=", 70))
	
	// Basic information
	if outputs.MockServerURL != "" {
		fmt.Printf("🔗 API Endpoint: %s\n", outputs.MockServerURL)
	}
	
	if outputs.DashboardURL != "" {
		fmt.Printf("📊 Dashboard: %s\n", outputs.DashboardURL)
	}
	
	if outputs.ConfigBucket != "" {
		fmt.Printf("🪣 Config Bucket: %s\n", outputs.ConfigBucket)
	}
	
	// Integration summary
	if outputs.IntegrationSummary != nil {
		fmt.Println("\n📋 Integration Status:")
		
		if bucket, ok := outputs.IntegrationSummary["s3_bucket"].(string); ok {
			fmt.Printf("   S3 Bucket: %s\n", bucket)
		}
		
		if cluster, ok := outputs.IntegrationSummary["ecs_cluster_arn"].(string); ok {
			clusterName := extractClusterName(cluster)
			fmt.Printf("   ECS Cluster: %s\n", clusterName)
		}
		
		if service, ok := outputs.IntegrationSummary["ecs_service_name"].(string); ok {
			fmt.Printf("   ECS Service: %s\n", service)
		}
		
		if method, ok := outputs.IntegrationSummary["config_reload_method"].(string); ok {
			fmt.Printf("   Config Reload: %s\n", method)
		}
	}
	
	// Quick commands
	if outputs.CLICommands != nil && len(outputs.CLICommands) > 0 {
		fmt.Println("\n🔧 Quick Commands:")
		
		if cmd, ok := outputs.CLICommands["upload_config"]; ok {
			fmt.Printf("   Upload Config: %s\n", cmd)
		}
		
		if cmd, ok := outputs.CLICommands["reload_service"]; ok {
			fmt.Printf("   Reload Service: %s\n", cmd)
		}
		
		if cmd, ok := outputs.CLICommands["view_logs"]; ok {
			fmt.Printf("   View Logs: %s\n", cmd)
		}
	}
	
	fmt.Println(strings.Repeat("=", 70) + "\n")
}

// DisplayProjectOptions shows available actions for an existing project
func DisplayProjectOptions(projectName string, outputs *InfrastructureOutputs) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("🎛️  PROJECT OPTIONS: %s\n", projectName)
	fmt.Println(strings.Repeat("=", 70))
	
	fmt.Println("Available actions:")
	fmt.Println("1. 🧠 Generate new mock configuration")
	fmt.Println("2. ✏️  Edit existing configuration")
	fmt.Println("3. 📊 View infrastructure status")
	fmt.Println("4. 🗑️  Destroy infrastructure")
	fmt.Println("5. ❌ Cancel")
	
	fmt.Println(strings.Repeat("=", 70) + "\n")
}

// DisplayConfigurationUpload shows information about uploading configuration
func DisplayConfigurationUpload(bucketName string, expectations interface{}) {
	fmt.Println("\n" + strings.Repeat("-", 50))
	fmt.Println("📤 UPLOADING CONFIGURATION")
	fmt.Println(strings.Repeat("-", 50))
	
	fmt.Printf("Target: s3://%s/expectations.json\n", bucketName)
	fmt.Printf("Status: Uploading...\n")
}

// DisplayConfigurationUploaded shows successful upload confirmation
func DisplayConfigurationUploaded(bucketName string, reloadCommand string) {
	fmt.Println("✅ Configuration uploaded successfully!")
	fmt.Printf("📍 Location: s3://%s/expectations.json\n", bucketName)
	
	if reloadCommand != "" {
		fmt.Println("\n🔄 To activate the new configuration, run:")
		fmt.Printf("   %s\n", reloadCommand)
	}
	
	fmt.Println(strings.Repeat("-", 50) + "\n")
}

// extractClusterName extracts the cluster name from an ARN
func extractClusterName(clusterArn string) string {
	parts := strings.Split(clusterArn, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return clusterArn
}

// DisplayTerraformError shows formatted Terraform errors
func DisplayTerraformError(operation string, err error) {
	fmt.Println("\n" + strings.Repeat("❌", 20))
	fmt.Printf("TERRAFORM %s FAILED\n", strings.ToUpper(operation))
	fmt.Println(strings.Repeat("❌", 20))
	
	fmt.Printf("Error: %v\n", err)
	
	fmt.Println("\n💡 Troubleshooting:")
	fmt.Println("1. Check your AWS credentials are configured")
	fmt.Println("2. Ensure Terraform is installed and in PATH")
	fmt.Println("3. Verify you have necessary AWS permissions")
	fmt.Println("4. Check for resource naming conflicts")
	
	fmt.Println(strings.Repeat("=", 50) + "\n")
}

// DisplayTerraformSuccess shows successful Terraform operations
func DisplayTerraformSuccess(operation string, projectName string) {
	fmt.Println("\n" + strings.Repeat("✅", 20))
	fmt.Printf("TERRAFORM %s COMPLETED\n", strings.ToUpper(operation))
	fmt.Println(strings.Repeat("✅", 20))
	
	fmt.Printf("Project: %s\n", projectName)
	fmt.Printf("Operation: %s\n", operation)
	
	if operation == "destroy" {
		fmt.Println("🧹 All infrastructure has been removed")
		fmt.Println("💰 No further AWS charges will be incurred")
	}
	
	fmt.Println(strings.Repeat("=", 50) + "\n")
}
