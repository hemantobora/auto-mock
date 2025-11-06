package aws

import (
	"fmt"

	"github.com/hemantobora/auto-mock/internal/models"
)

// DisplayCostEstimateWithSize prints an approximate monthly cost for us-east-1,
// using your size map -> (cpu units, memory MiB). Assumes 1 ALB, 1 NAT, etc.
func (p *Provider) DisplayCostEstimate(options *models.DeploymentOptions) {
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

	totalMonthly := baseMonthly + albMonthly + dataMonthly + storageLogs

	if len(options.NatGatewayIDs) > 0 {
		totalMonthly += natMonthly
	}

	fmt.Printf("  Base (24/7, %d x %s @ %.2fvCPU/%.1fGB):  				$%.2f/month\n",
		options.MinTasks, options.InstanceSize, vCPU, memGB, baseMonthly)
	fmt.Printf("  ALB (1x):                                				$%.2f/month\n", albMonthly)
	if len(options.NatGatewayIDs) > 0 {
		fmt.Printf("  NAT Gateway (1x):                        				$%.2f/month\n", natMonthly)
	}
	fmt.Printf("  Data Transfer (assumed ~20 GB egress @ $0.09/GB):                 	$%.2f/month\n", dataMonthly)
	fmt.Printf("  Storage & Logs (assumed < 1 GB):                			$%.2f/month\n", storageLogs)
	fmt.Printf("  -----------------------------------------------------------------------------\n")
	fmt.Printf("  Total:                                   				$%.2f/month\n", totalMonthly)
	fmt.Println()

	if options.MaxTasks > options.MinTasks {
		peakHourly := float64(options.MaxTasks) * perTaskHour
		fmt.Printf("  Note: Auto-scaling may increase cost up to %d tasks\n", options.MaxTasks)
		fmt.Printf("        Peak compute hourly:               				$%.3f/hour\n", peakHourly)
		fmt.Println()
	}

	fmt.Printf("  (Assumes us-east-1 Fargate: $%.5f/vCPU-hr + $%.5f/GB-hr; ALB/NAT/Data/Logs are rough)\n",
		fargatePerVCPUHour, fargatePerGBHour)
}
