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

	// TTL Information
	if ttl, ok := outputs.InfrastructureSummary["ttl"].(map[string]interface{}); ok {
		if enabled, ok := ttl["enabled"].(bool); ok && enabled {
			fmt.Println("AUTO-TEARDOWN:")
			fmt.Printf("  Enabled:          Yes\n")
			fmt.Printf("  TTL:              %v hours\n", ttl["hours"])
			fmt.Printf("  Expires at:       %v\n", ttl["expiry"])
			fmt.Println()
			fmt.Println("  Note: Infrastructure will be automatically deleted when TTL expires.")
			fmt.Println("        Use 'automock extend-ttl' to extend if needed.")
			fmt.Println()
		}
	}

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
	fmt.Println("  - ECS Cluster and Service")
	fmt.Println("  - Application Load Balancer")
	fmt.Println("  - VPC and Networking Resources")
	fmt.Println("  - Storage Configuration Bucket (and all data)")
	fmt.Println("  - CloudWatch Logs")
	fmt.Println("  - TTL Cleanup Lambda Function")
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
		fmt.Println("Estimated monthly cost savings: ~$93")
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

// DisplayCostEstimateWithSize prints an approximate monthly cost for us-east-1,
// using your size map -> (cpu units, memory MiB). Assumes 1 ALB, 1 NAT, etc.
func DisplayAwsCostEstimate(options DeploymentOptions) {
	fmt.Println()
	fmt.Println("APPROX. COST ESTIMATE (us-east-1):")

	// --- Assumed unit prices (rounded, us-east-1) ---
	const (
		hoursPerMonth = 730.0
		// Fargate Linux/x86 pricing (per hour):
		fargatePerVCPUHour = 0.04048
		fargatePerGBHour   = 0.004445

		// Simple add-ons (you can tune these defaults as needed):
		albMonthly  = 20.00 // 1 ALB: hourly + a modest LCU buffer
		natMonthly  = 32.85 // 1 NAT gateway hourly (no per-GB here)
		dataMonthly = 1.80  // ~20 GB egress @ $0.09/GB
		storageLogs = 2.70  // CloudWatch logs + S3 small foot-print
	)

	// Convert ECS CPU units/MiB -> vCPU/GB
	vCPU := float64(options.CPUUnits) / 1024.0
	memGB := float64(options.MemoryUnits) / 1024.0

	// Per-task hourly (Fargate compute)
	perTaskHour := vCPU*fargatePerVCPUHour + memGB*fargatePerGBHour

	// Base compute (24/7 @ minTasks)
	baseMonthly := float64(options.MinTasks) * perTaskHour * hoursPerMonth

	totalMonthly := baseMonthly + albMonthly + natMonthly + dataMonthly + storageLogs

	fmt.Printf("  Base (24/7, %d x %s @ %.2fvCPU/%.1fGB):  $%.2f/month\n",
		options.MinTasks, options.InstanceSize, vCPU, memGB, baseMonthly)
	fmt.Printf("  ALB (1x):                                $%.2f/month\n", albMonthly)
	fmt.Printf("  NAT Gateway (1x):                        $%.2f/month\n", natMonthly)
	fmt.Printf("  Data Transfer (assumed):                 $%.2f/month\n", dataMonthly)
	fmt.Printf("  Storage & Logs (assumed):                $%.2f/month\n", storageLogs)
	fmt.Printf("  ----------------------------------------------------\n")
	fmt.Printf("  Total:                                   $%.2f/month\n", totalMonthly)
	fmt.Println()

	if options.TTLHours > 0 {
		actual := totalMonthly * (float64(options.TTLHours) / hoursPerMonth)
		fmt.Printf("  Actual cost (TTL=%dh):                   $%.2f\n", options.TTLHours, actual)
		fmt.Println()
	}

	if options.MaxTasks > options.MinTasks {
		peakHourly := float64(options.MaxTasks) * perTaskHour
		fmt.Printf("  Note: Auto-scaling may increase cost up to %d tasks\n", options.MaxTasks)
		fmt.Printf("        Peak compute hourly:               $%.3f/hour\n", peakHourly)
		fmt.Println()
	}

	fmt.Printf("  (Assumes us-east-1 Fargate: $%.5f/vCPU-hr + $%.5f/GB-hr; ALB/NAT/Data/Logs are rough)\n",
		fargatePerVCPUHour, fargatePerGBHour)
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

	// TTL info
	if ttl, ok := outputs.InfrastructureSummary["ttl"].(map[string]interface{}); ok {
		if enabled, ok := ttl["enabled"].(bool); ok && enabled {
			fmt.Printf("TTL:          %v hours remaining (expires: %v)\n",
				ttl["hours"], ttl["expiry"])
		} else {
			fmt.Println("TTL:          Disabled (no auto-teardown)")
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
}
