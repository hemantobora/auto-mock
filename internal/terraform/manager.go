// internal/terraform/manager.go
package terraform

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/models"
)

// Manager handles Terraform operations for AutoMock infrastructure
type Manager struct {
	ProjectName        string
	Region             string
	WorkingDir         string
	Provider           internal.Provider
	Profile            string
	ExistingBucketName string
}

// DefaultDeploymentOptions returns sensible defaults for development
func DefaultDeploymentOptions() *models.DeploymentOptions {
	return &models.DeploymentOptions{
		InstanceSize: "small",
	}
}

// NewManager creates a new Terraform manager
func NewManager(cleanProject, profile string, provider internal.Provider) (*Manager, error) {
	// Create a unique working directory for this deployment
	workingDir := filepath.Join(os.TempDir(), fmt.Sprintf("automock-%s-%s", cleanProject, time.Now().Format("20060102-150405")))

	return &Manager{
		ProjectName:        cleanProject,
		WorkingDir:         workingDir,
		Provider:           provider,
		ExistingBucketName: provider.GetStorageName(), // Use existing bucket if available
		Profile:            profile,
		Region:             provider.GetRegion(),
	}, nil
}

func (m *Manager) createTerraformVars(options *models.DeploymentOptions) error {
	varsFile := filepath.Join(m.WorkingDir, "terraform.tfvars")
	return os.WriteFile(varsFile, []byte(options.CreateTerraformVars()), 0644)
}

func (m *Manager) createBackendConfig() error {
	if m.ExistingBucketName == "" {
		return fmt.Errorf("no S3 bucket configured")
	}

	// NO leading spaces in the template string!
	backendConfig := fmt.Sprintf(`terraform {
  backend "s3" {
    bucket  = "%s"
    key     = "terraform/state/terraform.tfstate"
    region  = "%s"
    encrypt = true
  }
}
`, m.ExistingBucketName, m.Region)

	backendFile := filepath.Join(m.WorkingDir, "backend.tf")
	if err := os.WriteFile(backendFile, []byte(backendConfig), 0644); err != nil {
		return fmt.Errorf("failed to write backend config: %w", err)
	}

	fmt.Printf("✓ Configured Terraform backend: %s/terraform/state/\n",
		m.ExistingBucketName)
	return nil
}

// Deploy creates the complete infrastructure using Terraform
func (m *Manager) Deploy(options *models.DeploymentOptions) (*InfrastructureOutputs, error) {
	fmt.Printf("🚀 Deploying infrastructure for project: %s\n", m.ProjectName)

	// Validate bucket name was found
	if m.ExistingBucketName == "" {
		return nil, fmt.Errorf("no Storage bucket found for project '%s'. Please run 'automock init' first", m.ProjectName)
	}

	// Step 1: Prepare Terraform workspace
	if err := m.prepareWorkspace(); err != nil {
		return nil, fmt.Errorf("failed to prepare workspace: %w", err)
	}
	defer m.cleanup()

	// Create backend config
	if err := m.createBackendConfig(); err != nil {
		return nil, fmt.Errorf("failed to create backend config: %w", err)
	}

	// Step 2: Initialize Terraform
	if err := m.initTerraform(); err != nil {
		return nil, fmt.Errorf("failed to initialize terraform: %w", err)
	}

	// Step 3: Create terraform.tfvars file
	if err := m.createTerraformVars(options); err != nil {
		return nil, fmt.Errorf("failed to create terraform vars: %w", err)
	}

	// Step 4: Plan infrastructure
	if err := m.planTerraform(); err != nil {
		return nil, fmt.Errorf("terraform plan failed: %w", err)
	}

	// Step 5: Apply infrastructure
	fmt.Println("\n🏗️  Applying infrastructure changes...")
	if err := m.applyTerraform(); err != nil {
		return nil, fmt.Errorf("terraform apply failed: %w", err)
	}

	// Step 6: Get outputs
	outputs, err := m.getOutputs()
	if err != nil {
		return nil, fmt.Errorf("failed to get terraform outputs: %w", err)
	}

	fmt.Printf("✅ Infrastructure deployed successfully for project: %s\n", m.ProjectName)
	return outputs, nil
}

// Destroy removes the infrastructure
func (m *Manager) Destroy() error {
	fmt.Printf("🗑️  Destroying infrastructure for project: %s\n", m.ProjectName)

	if err := m.prepareWorkspace(); err != nil {
		return fmt.Errorf("failed to prepare workspace: %w", err)
	}
	defer m.cleanup()

	// Create backend config
	if err := m.createBackendConfig(); err != nil {
		return fmt.Errorf("failed to create backend config: %w", err)
	}

	if err := m.initTerraform(); err != nil {
		return fmt.Errorf("failed to initialize terraform: %w", err)
	}

	if err := m.createTerraformVars(m.Provider.CreateDefaultDeploymentConfiguration()); err != nil {
		return fmt.Errorf("failed to create terraform vars: %w", err)
	}

	fmt.Println("💥 Destroying infrastructure...")
	if err := m.destroyTerraform(); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	fmt.Printf("✅ Infrastructure destroyed successfully for project: %s\n", m.ProjectName)

	m.Provider.DeleteDeploymentMetadata()
	return nil
}

// prepareWorkspace sets up the Terraform working directory
func (m *Manager) prepareWorkspace() error {
	if err := os.MkdirAll(m.WorkingDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	// Materialize embedded Terraform templates into the working directory.
	if err := writeEmbeddedTemplates(mockTemplates, m.WorkingDir); err != nil {
		return fmt.Errorf("failed to materialize terraform templates: %w", err)
	}

	return nil
}

// initTerraform initializes the Terraform working directory
func (m *Manager) initTerraform() error {
	fmt.Println("🔧 Initializing Terraform...")

	done := make(chan bool)
	go m.ShowProgress("Initializing", done)

	cmd := exec.Command("terraform", "init")
	cmd.Dir = m.WorkingDir
	cmd.Env = append(os.Environ(), m.getTerraformEnv()...)

	output, err := cmd.CombinedOutput()
	done <- true

	if err != nil {
		return fmt.Errorf("terraform init failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// planTerraform runs terraform plan
func (m *Manager) planTerraform() error {
	fmt.Println("📋 Planning infrastructure changes...")

	done := make(chan bool)
	go m.ShowProgress("Planning", done)

	cmd := exec.Command("terraform", "plan", "-out=tfplan")
	cmd.Dir = m.WorkingDir
	cmd.Env = append(os.Environ(), m.getTerraformEnv()...)

	output, err := cmd.CombinedOutput()
	done <- true

	if err != nil {
		return fmt.Errorf("%w\nOutput: %s", err, string(output))
	}

	return nil
}

// applyTerraform runs terraform apply
func (m *Manager) applyTerraform() error {
	cmd := exec.Command("terraform", "apply", "-auto-approve", "tfplan")
	cmd.Dir = m.WorkingDir
	cmd.Env = append(os.Environ(), m.getTerraformEnv()...)

	return m.runCommandWithOutput(cmd)
}

// destroyTerraform runs terraform destroy
func (m *Manager) destroyTerraform() error {
	cmd := exec.Command("terraform", "destroy", "-auto-approve")
	cmd.Dir = m.WorkingDir
	cmd.Env = append(os.Environ(), m.getTerraformEnv()...)

	return m.runCommandWithOutput(cmd)
}

// getOutputs retrieves Terraform outputs
func (m *Manager) getOutputs() (*InfrastructureOutputs, error) {
	cmd := exec.Command("terraform", "output", "-json")
	cmd.Dir = m.WorkingDir
	cmd.Env = append(os.Environ(), m.getTerraformEnv()...)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get terraform outputs: %w", err)
	}

	var rawOutputs map[string]struct {
		Value interface{} `json:"value"`
	}

	if err := json.Unmarshal(output, &rawOutputs); err != nil {
		return nil, fmt.Errorf("failed to parse terraform outputs: %w", err)
	}

	outputs := &InfrastructureOutputs{}

	if val, ok := rawOutputs["mockserver_url"]; ok {
		if url, ok := val.Value.(string); ok {
			outputs.MockServerURL = url
		}
	}

	if val, ok := rawOutputs["dashboard_url"]; ok {
		if url, ok := val.Value.(string); ok {
			outputs.DashboardURL = url
		}
	}

	if val, ok := rawOutputs["config_bucket"]; ok {
		if bucket, ok := val.Value.(string); ok {
			outputs.ConfigBucket = bucket
		}
	}

	if val, ok := rawOutputs["integration_summary"]; ok {
		if summary, ok := val.Value.(map[string]interface{}); ok {
			outputs.IntegrationSummary = summary
		}
	}

	if val, ok := rawOutputs["cli_integration_commands"]; ok {
		if commands, ok := val.Value.(map[string]interface{}); ok {
			outputs.CLICommands = make(map[string]string)
			for k, v := range commands {
				if cmd, ok := v.(string); ok {
					outputs.CLICommands[k] = cmd
				}
			}
		}
	}

	if val, ok := rawOutputs["infrastructure_summary"]; ok {
		if summary, ok := val.Value.(map[string]interface{}); ok {
			outputs.InfrastructureSummary = summary
		}
	}

	return outputs, nil
}

// getTerraformEnv returns environment variables for Terraform
func (m *Manager) getTerraformEnv() []string {
	env := []string{}

	if m.Profile != "" {
		switch m.Provider.GetProviderType() {
		case "aws":
			env = append(env, fmt.Sprintf("AWS_PROFILE=%s", m.Profile))
		case "gcp":
			env = append(env, fmt.Sprintf("GOOGLE_CLOUD_PROJECT=%s", m.Profile))
		case "azure":
			env = append(env, fmt.Sprintf("AZURE_SUBSCRIPTION_ID=%s", m.Profile))
		}
	}
	env = append(env, "TF_CLI_CONFIG_FILE=/dev/null")

	return env
}

// runCommandWithOutput runs a command and streams output to the user
func (m *Manager) runCommandWithOutput(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Fprintf(os.Stderr, "%s\n", scanner.Text())
		}
	}()

	return cmd.Wait()
}

// cleanup removes the temporary working directory
func (m *Manager) cleanup() {
	if m.WorkingDir != "" && strings.Contains(m.WorkingDir, "automock-") {
		os.RemoveAll(m.WorkingDir)
	}
}

// CheckTerraformInstalled verifies that Terraform is installed and accessible
func CheckTerraformInstalled() error {
	cmd := exec.Command("terraform", "version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("terraform not found in PATH. Please install Terraform: https://terraform.io/downloads")
	}

	version := strings.Split(string(output), "\n")[0]
	fmt.Printf("🔧 Found %s\n", version)

	return nil
}

// ShowProgress displays a spinner/progress indicator during long operations
func (m *Manager) ShowProgress(action string, done chan bool) {
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠇", "⠏", "⠉"}
	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			fmt.Printf("\r\033[K✓ %s complete\n", action)
			return
		case <-ticker.C:
			fmt.Printf("\r%s %s...", spinners[i%len(spinners)], action)
			i++
		}
	}
}
