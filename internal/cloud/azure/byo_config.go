package azure

import (
	"fmt"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

// vmSizeOptions lists the AKS node VM sizes offered during interactive setup.
// Format: label shown in picker → actual Azure VM size string.
var vmSizeOptions = []struct {
	Label       string
	Size        string
	Description string
}{
	{
		Label:       "Standard_B2s    (2 vCPU,  4 GB RAM) — Dev / Test",
		Size:        "Standard_B2s",
		Description: "Burstable VM. Cheapest option; fine for small-scale testing.",
	},
	{
		Label:       "Standard_D2s_v3 (2 vCPU,  8 GB RAM) — General Purpose",
		Size:        "Standard_D2s_v3",
		Description: "Consistent performance. Good default for shared / staging environments.",
	},
	{
		Label:       "Standard_D4s_v3 (4 vCPU, 16 GB RAM) — High Throughput",
		Size:        "Standard_D4s_v3",
		Description: "Recommended when running many concurrent mock requests or load tests on the same cluster.",
	},
}

// CreateDeploymentConfiguration interactively collects AKS deployment options.
// It asks for node VM size and node count, then returns a DeploymentOptions
// ready to be passed to Terraform.
func (p *Provider) CreateDeploymentConfiguration() *models.DeploymentOptions {
	fmt.Println()
	fmt.Println("🔷 Azure AKS Deployment Configuration")
	fmt.Println("   AutoMock will deploy MockServer onto a managed Kubernetes cluster (AKS).")
	fmt.Println("   You can scale the cluster up or down after deployment from the Azure portal.")
	fmt.Println()

	// ── Node VM size ──────────────────────────────────────────────────────────
	labels := make([]string, len(vmSizeOptions))
	for i, opt := range vmSizeOptions {
		labels[i] = opt.Label
	}

	var selectedLabel string
	sizeQ := &survey.Select{
		Message: "Node VM size:",
		Options: labels,
		Default: labels[0], // Standard_B2s
		Help:    "This is the VM size for each AKS worker node. You can change it later by re-deploying.",
	}
	if err := survey.AskOne(sizeQ, &selectedLabel); err != nil {
		return nil
	}

	chosenSize := vmSizeOptions[0].Size // default
	for _, opt := range vmSizeOptions {
		if opt.Label == selectedLabel {
			chosenSize = opt.Size
			break
		}
	}

	// ── Node count ────────────────────────────────────────────────────────────
	var nodeCountStr string
	countQ := &survey.Input{
		Message: "Number of nodes:",
		Default: "1",
		Help:    "How many AKS worker nodes to provision. 1 is sufficient for dev/test; increase for higher availability.",
	}
	if err := survey.AskOne(countQ, &nodeCountStr, survey.WithValidator(func(ans interface{}) error {
		s, _ := ans.(string)
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 20 {
			return fmt.Errorf("please enter a number between 1 and 20")
		}
		return nil
	})); err != nil {
		return nil
	}

	nodeCount, _ := strconv.Atoi(nodeCountStr)
	if nodeCount < 1 {
		nodeCount = 1
	}

	opts := &models.DeploymentOptions{
		Provider:             p.GetProviderType(),
		ProjectName:          p.GetProjectName(),
		Region:               p.GetRegion(),        // Azure location (e.g. "eastus")
		BucketName:           p.accountName,        // storage_account_name in tfvars
		StorageContainerName: p.containerName,      // container_name in tfvars
		SubscriptionID:       p.subscriptionID,
		ResourceGroup:        p.resourceGroup,
		NodeVMSize:           chosenSize,
		NodeCount:            nodeCount,
	}

	fmt.Printf("\n   Cluster  : automock-%s (AKS, %s × %d)\n", opts.ProjectName, chosenSize, nodeCount)
	fmt.Printf("   Location : %s\n", opts.Region)
	fmt.Printf("   Storage  : %s / %s\n\n", opts.BucketName, opts.StorageContainerName)

	return opts
}

// CreateDefaultDeploymentConfiguration returns a minimal DeploymentOptions used
// by the destroy command. It reconstructs Azure-specific fields from the provider
// state so that Terraform receives all required variables without interaction.
func (p *Provider) CreateDefaultDeploymentConfiguration() *models.DeploymentOptions {
	opts := &models.DeploymentOptions{
		Provider:             p.GetProviderType(),
		ProjectName:          p.GetProjectName(),
		Region:               p.GetRegion(),
		BucketName:           p.accountName,
		StorageContainerName: p.containerName,
		SubscriptionID:       p.subscriptionID,
		ResourceGroup:        p.resourceGroup,
		NodeVMSize:           "Standard_B2s", // safe default; not used for destroy
		NodeCount:            1,
	}
	return opts
}
