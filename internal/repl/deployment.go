// internal/repl/deployment.go
// REPL deployment integration with Terraform
package repl

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/terraform"
	"github.com/hemantobora/auto-mock/internal/utils"
)

// deployInfrastructureWithTerraform deploys actual infrastructure using Terraform
func deployInfrastructureWithTerraform(projectName, awsProfile string) error {
	cleanName := utils.ExtractUserProjectName(projectName)

	fmt.Println("\n🏗️  Complete Infrastructure Deployment")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Check Terraform installation
	if err := terraform.CheckTerraformInstalled(); err != nil {
		return fmt.Errorf("terraform not found: %w\nPlease install from https://terraform.io/downloads", err)
	}

	// Prompt for deployment options
	options, err := promptDeploymentOptionsREPL()
	if err != nil {
		return err
	}

	// Show cost estimate
	terraform.DisplayCostEstimate(10, 200, options.TTLHours)

	// Confirm deployment
	var confirmed bool
	confirmPrompt := &survey.Confirm{
		Message: "Proceed with infrastructure deployment?",
		Default: true,
		Help:    "This will create ECS Fargate cluster, ALB, and supporting resources",
	}

	if err := survey.AskOne(confirmPrompt, &confirmed); err != nil {
		return err
	}

	if !confirmed {
		fmt.Println("\n❌ Deployment cancelled")
		return nil
	}

	// Create Terraform manager
	manager := terraform.NewManager(cleanName, awsProfile)

	// Deploy infrastructure
	fmt.Println("\n🚀 Deploying infrastructure with Terraform...")
	outputs, err := manager.Deploy(options)
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	// Display results
	terraform.DisplayDeploymentResults(outputs, cleanName)

	return nil
}

// promptDeploymentOptionsREPL prompts for deployment configuration in REPL
func promptDeploymentOptionsREPL() (*terraform.DeploymentOptions, error) {
	options := terraform.DefaultDeploymentOptions()

	fmt.Println("\n⚙️  Deployment Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Instance size
	var instanceSize string
	sizePrompt := &survey.Select{
		Message: "Select instance size:",
		Options: []string{"small", "medium", "large", "xlarge"},
		Default: "small",
		Description: func(value string, index int) string {
			switch value {
			case "small":
				return "0.5 vCPU, 1GB RAM (recommended for testing)"
			case "medium":
				return "1 vCPU, 2GB RAM (moderate load)"
			case "large":
				return "2 vCPU, 4GB RAM (high load)"
			case "xlarge":
				return "4 vCPU, 8GB RAM (very high load)"
			default:
				return ""
			}
		},
	}

	if err := survey.AskOne(sizePrompt, &instanceSize); err != nil {
		return nil, err
	}
	options.InstanceSize = instanceSize

	// TTL hours
	var ttlHours string
	ttlPrompt := &survey.Input{
		Message: "Auto-teardown timeout (hours, 0 = disabled):",
		Default: "8",
		Help:    "Infrastructure will be automatically deleted after this time to prevent runaway costs",
	}

	if err := survey.AskOne(ttlPrompt, &ttlHours); err != nil {
		return nil, err
	}

	// Convert to int
	var ttlInt int
	fmt.Sscanf(ttlHours, "%d", &ttlInt)
	options.TTLHours = ttlInt
	options.EnableTTLCleanup = ttlInt > 0

	// Notification email (if TTL enabled)
	if ttlInt > 0 {
		var emailWanted bool
		emailPrompt := &survey.Confirm{
			Message: "Receive notification before auto-teardown?",
			Default: false,
		}

		if err := survey.AskOne(emailPrompt, &emailWanted); err != nil {
			return nil, err
		}

		if emailWanted {
			var email string
			emailInputPrompt := &survey.Input{
				Message: "Notification email:",
			}

			if err := survey.AskOne(emailInputPrompt, &email); err != nil {
				return nil, err
			}
			options.NotificationEmail = email
		}
	}

	// Custom domain (optional)
	var useCustomDomain bool
	domainPrompt := &survey.Confirm{
		Message: "Use custom domain?",
		Default: false,
	}

	if err := survey.AskOne(domainPrompt, &useCustomDomain); err != nil {
		return nil, err
	}

	if useCustomDomain {
		var domain string
		domainInputPrompt := &survey.Input{
			Message: "Custom domain (e.g., api.example.com):",
		}

		if err := survey.AskOne(domainInputPrompt, &domain); err != nil {
			return nil, err
		}
		options.CustomDomain = domain

		var hostedZoneID string
		zonePrompt := &survey.Input{
			Message: "Route53 Hosted Zone ID:",
		}

		if err := survey.AskOne(zonePrompt, &hostedZoneID); err != nil {
			return nil, err
		}
		options.HostedZoneID = hostedZoneID
	}

	return options, nil
}
