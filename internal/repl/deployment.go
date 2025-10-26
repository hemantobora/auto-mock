package repl

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/models"
	"github.com/hemantobora/auto-mock/internal/terraform"
)

type Deployment struct {
	ProjectName string
	Provider    internal.Provider
	Profile     string
}

// NewDeployment creates a new Deployment instance
func NewDeployment(projectName, profile string, provider internal.Provider, options *models.DeploymentOptions) *Deployment {
	return &Deployment{
		ProjectName: projectName,
		Provider:    provider,
		Profile:     profile,
	}
}

// DeployInfrastructureWithTerraform deploys actual infrastructure using Terraform
func (d *Deployment) DeployInfrastructureWithTerraform(skip_confirmation bool) error {
	fmt.Println("\n🏗️  Complete Infrastructure Deployment")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Create Terraform manager
	manager, err := terraform.NewManager(d.ProjectName, d.Profile, d.Provider)
	if err != nil {
		return fmt.Errorf("failed to create terraform manager: %w", err)
	}

	// Check Terraform installation
	if err := terraform.CheckTerraformInstalled(); err != nil {
		return fmt.Errorf("terraform not found: %w\nPlease install from https://terraform.io/downloads", err)
	}

	options := d.Provider.CreateDeploymentConfiguration()
	// <-- IMPORTANT: make these options the ones we deploy with

	// ── 3) Ask for size / min / max (fills remaining fields on d.Options) ─────

	// Optional: show cost estimate (AWS)
	d.Provider.DisplayCostEstimate(options)

	// ── 4) Confirm ────────────────────────────────────────────────────────────
	if !skip_confirmation {
		var confirmed bool
		confirmPrompt := &survey.Confirm{
			Message: "Proceed with infrastructure deployment?",
			Default: true,
			Help:    "This will create necessary resources and deploy your mocks to the cloud provider.",
		}
		if err := survey.AskOne(confirmPrompt, &confirmed); err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("\n❌ Deployment cancelled")
			return nil
		}
	}

	// ── 6) Deploy ─────────────────────────────────────────────────────────────
	fmt.Println("\n🚀 Deploying infrastructure with Terraform...")
	outputs, err := manager.Deploy(options) // uses the options we just assembled
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	// ── 7) Show results ───────────────────────────────────────────────────────
	terraform.DisplayDeploymentResults(outputs, d.ProjectName)
	return nil
}
