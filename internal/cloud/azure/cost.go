package azure

import (
	"fmt"

	"github.com/hemantobora/auto-mock/internal/models"
)

// DisplayCostEstimate prints an approximate monthly cost for the Azure AKS deployment.
// Figures are rough East US list prices as of early 2025 — actual costs vary by region,
// reserved pricing, and Azure credits.
func (p *Provider) DisplayCostEstimate(opts *models.DeploymentOptions) {
	vmSize := "Standard_B2s"
	nodeCount := 1
	if opts != nil {
		if opts.NodeVMSize != "" {
			vmSize = opts.NodeVMSize
		}
		if opts.NodeCount > 0 {
			nodeCount = opts.NodeCount
		}
	}

	// Approximate hourly node costs (East US, pay-as-you-go)
	nodeHourlyCost := map[string]float64{
		"Standard_B2s":    0.047,
		"Standard_D2s_v3": 0.096,
		"Standard_D4s_v3": 0.192,
	}

	hourly, ok := nodeHourlyCost[vmSize]
	if !ok {
		hourly = 0.10 // reasonable unknown default
	}

	monthlyNode := hourly * float64(nodeCount) * 730 // ~730 hrs/month
	monthlyStorage := 2.0                            // Storage Account + Blob ops (typical AutoMock usage)
	monthlyAKSMgmt := 0.10                           // AKS control plane: free tier in most regions ($0.10/hr in some)
	total := monthlyNode + monthlyStorage + monthlyAKSMgmt

	fmt.Println()
	fmt.Println("💰 Estimated Monthly Cost (Azure East US — pay-as-you-go)")
	fmt.Println("   ─────────────────────────────────────────────────────")
	fmt.Printf("   AKS nodes  : %d × %-20s ~$%.2f/mo\n", nodeCount, vmSize, monthlyNode)
	fmt.Printf("   Blob Storage                        ~$%.2f/mo\n", monthlyStorage)
	fmt.Printf("   AKS control plane                   ~$%.2f/mo\n", monthlyAKSMgmt)
	fmt.Println("   ─────────────────────────────────────────────────────")
	fmt.Printf("   Total                               ~$%.2f/mo\n", total)
	fmt.Println()
	fmt.Println("   💡 Tip: Stop/deallocate the node pool when not in use to avoid idle costs.")
	fmt.Println("      Azure Reserved Instances (1yr) typically save ~40%.")
	fmt.Println()
}
