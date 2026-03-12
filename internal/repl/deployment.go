package repl

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/terraform"
)

type Deployment struct {
	ProjectName string
	Provider    internal.Provider
	Profile     string
}

// NewDeployment creates a new Deployment instance
func NewDeployment(projectName, profile string, provider internal.Provider) *Deployment {
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

	// ── 1) Check Project Configuration ───────────────────────────────────────
	if _, err := manager.Provider.GetConfig(context.Background(), d.ProjectName); err != nil {
		return fmt.Errorf("project configuration does not exist, nothing to deploy; please run 'auto-mock init' first")
	}

	// Check Terraform installation
	if err := terraform.CheckTerraformInstalled(); err != nil {
		return fmt.Errorf("terraform not found: %w\nPlease install from https://terraform.io/downloads", err)
	}

	options := d.Provider.CreateDeploymentConfiguration()

	// Optional: show cost estimate (AWS)
	d.Provider.DisplayCostEstimate(options)
	fmt.Println()

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
	outputs, err := manager.Deploy(options) // uses the options we just assembled
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	// Carry custom domain + hosted zone flag into outputs so they are persisted
	// in deployment metadata and can be restored verbatim when destroy runs.
	outputs.CustomDomain = options.CustomDomain
	outputs.CreateHostedZone = options.CreateHostedZone

	d.Provider.SaveDeploymentMetadata(outputs)
	return nil
}
