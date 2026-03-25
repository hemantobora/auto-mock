package aws

import (
	"fmt"

	"github.com/hemantobora/auto-mock/internal/models"
)

// DisplayCostEstimate prints an approximate monthly cost for us-east-1,
// using your size map -> (cpu units, memory MiB). Assumes 1 ALB, 1 NAT, etc.
func (p *Provider) DisplayCostEstimate(options *models.DeploymentOptions) {
	fmt.Println()
	fmt.Println("Approx. monthly cost estimate (us-east-1):")

	const (
		hoursPerMonth      = 730.0
		fargatePerVCPUHour = 0.04048
		fargatePerGBHour   = 0.004445
		albMonthly         = 20.00
		natMonthly         = 32.85
		dataMonthly        = 1.80
		storageLogs        = 2.70
	)

	vCPU := float64(options.CPUUnits) / 1024.0
	memGB := float64(options.MemoryUnits) / 1024.0
	perTaskHour := vCPU*fargatePerVCPUHour + memGB*fargatePerGBHour
	baseMonthly := float64(options.MinTasks) * perTaskHour * hoursPerMonth

	totalMonthly := baseMonthly + albMonthly + dataMonthly + storageLogs
	// NAT cost applies when Terraform creates a new gateway (greenfield VPC).
	// BYO NAT (UseExistingNAT=true) means the user already owns it — no new cost.
	if !options.UseExistingNAT {
		totalMonthly += natMonthly
	}
	if options.PrivateALB {
		totalMonthly += albMonthly
	}

	row := func(label string, cost float64) {
		fmt.Printf("  %-52s $%.2f/month\n", label, cost)
	}

	baseLabel := fmt.Sprintf("Base (%d × %s @ %.2f vCPU/%.1f GB, 24/7):", options.MinTasks, options.InstanceSize, vCPU, memGB)
	row(baseLabel, baseMonthly)
	row("ALB (1×):", albMonthly)
	if options.PrivateALB {
		row("Internal ALB:", albMonthly)
	}
	if !options.UseExistingNAT {
		row("NAT Gateway (1×, Terraform-managed):", natMonthly)
	}
	row("Data transfer (~20 GB egress @ $0.09/GB):", dataMonthly)
	row("Storage & logs (<1 GB):", storageLogs)
	fmt.Println("  ──────────────────────────────────────────────────────────────")
	row("Total:", totalMonthly)
	fmt.Println()

	if options.MaxTasks > options.MinTasks {
		peakHourly := float64(options.MaxTasks) * perTaskHour
		fmt.Printf("  Note: auto-scaling up to %d tasks; peak compute $%.3f/hour\n", options.MaxTasks, peakHourly)
		fmt.Println()
	}

	fmt.Printf("  (Fargate: $%.5f/vCPU-hr + $%.5f/GB-hr; ALB/NAT/data/logs are rough estimates)\n",
		fargatePerVCPUHour, fargatePerGBHour)
}
