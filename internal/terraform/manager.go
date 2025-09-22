// internal/terraform/manager.go
package terraform

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hemantobora/auto-mock/internal/utils"
)

// Manager handles Terraform operations for AutoMock infrastructure
type Manager struct {
	ProjectName   string
	Environment   string
	Region        string
	TerraformDir  string
	WorkingDir    string
	AWSProfile    string
}

// DeploymentOptions configures the infrastructure deployment
type DeploymentOptions struct {
	InstanceSize      string
	TTLHours          int
	CustomDomain      string
	HostedZoneID      string
	NotificationEmail string
	EnableTTLCleanup  bool
}

// DefaultDeploymentOptions returns sensible defaults for development
func DefaultDeploymentOptions() *DeploymentOptions {
	return &DeploymentOptions{
		InstanceSize:     "small",
		TTLHours:         4,
		EnableTTLCleanup: true,
	}
}

// InfrastructureOutputs contains Terraform outputs after deployment
type InfrastructureOutputs struct {
	MockServerURL          string                 `json:"mockserver_url"`
	DashboardURL           string                 `json:"dashboard_url"`
	ConfigBucket           string                 `json:"config_bucket"`
	IntegrationSummary     map[string]interface{} `json:"integration_summary"`
	CLICommands            map[string]string      `json:"cli_integration_commands"`
	InfrastructureSummary  map[string]interface{} `json:"infrastructure_summary"`
}

// NewManager creates a new Terraform manager
func NewManager(projectName, awsProfile string) *Manager {
	// Extract clean project name and create environment
	cleanProject := utils.ExtractUserProjectName(projectName)
	environment := "dev" // Default to dev for now
	
	// Get the project root directory
	execPath, _ := os.Executable()
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(execPath)))
	
	// Use embedded terraform directory or fallback to project terraform dir
	terraformDir := filepath.Join(projectRoot, "terraform")
	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		// Fallback to current directory terraform
		terraformDir = "./terraform"
	}
	
	// Create a unique working directory for this deployment
	workingDir := filepath.Join(os.TempDir(), fmt.Sprintf("automock-%s-%s", cleanProject, time.Now().Format("20060102-150405")))
	
	return &Manager{
		ProjectName:  cleanProject,
		Environment:  environment,
		Region:       "us-east-1", // Default region
		TerraformDir: terraformDir,
		WorkingDir:   workingDir,
		AWSProfile:   awsProfile,
	}
}

// Deploy creates the complete infrastructure using Terraform
func (m *Manager) Deploy(options *DeploymentOptions) (*InfrastructureOutputs, error) {
	fmt.Printf("🚀 Deploying infrastructure for project: %s\n", m.ProjectName)
	
	// Step 1: Prepare Terraform workspace
	if err := m.prepareWorkspace(); err != nil {
		return nil, fmt.Errorf("failed to prepare workspace: %w", err)
	}
	defer m.cleanup() // Clean up temporary directory
	
	// Step 2: Initialize Terraform
	if err := m.initTerraform(); err != nil {
		return nil, fmt.Errorf("failed to initialize terraform: %w", err)
	}
	
	// Step 3: Create terraform.tfvars file
	if err := m.createTerraformVars(options); err != nil {
		return nil, fmt.Errorf("failed to create terraform vars: %w", err)
	}
	
	// Step 4: Plan infrastructure
	fmt.Println("📋 Planning infrastructure changes...")
	if err := m.planTerraform(); err != nil {
		return nil, fmt.Errorf("terraform plan failed: %w", err)
	}
	
	// Step 5: Apply infrastructure
	fmt.Println("🏗️  Applying infrastructure changes...")
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
	
	// Prepare workspace
	if err := m.prepareWorkspace(); err != nil {
		return fmt.Errorf("failed to prepare workspace: %w", err)
	}
	defer m.cleanup()
	
	// Initialize Terraform
	if err := m.initTerraform(); err != nil {
		return fmt.Errorf("failed to initialize terraform: %w", err)
	}
	
	// Create minimal terraform.tfvars for destroy
	options := DefaultDeploymentOptions()
	if err := m.createTerraformVars(options); err != nil {
		return fmt.Errorf("failed to create terraform vars: %w", err)
	}
	
	// Destroy infrastructure
	fmt.Println("💥 Destroying infrastructure...")
	if err := m.destroyTerraform(); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}
	
	fmt.Printf("✅ Infrastructure destroyed successfully for project: %s\n", m.ProjectName)
	return nil
}

// prepareWorkspace sets up the Terraform working directory
func (m *Manager) prepareWorkspace() error {
	// Create working directory
	if err := os.MkdirAll(m.WorkingDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}
	
	// Copy Terraform files to working directory
	if err := m.copyTerraformFiles(); err != nil {
		return fmt.Errorf("failed to copy terraform files: %w", err)
	}
	
	return nil
}

// copyTerraformFiles copies the Terraform configuration to the working directory
func (m *Manager) copyTerraformFiles() error {
	return filepath.Walk(m.TerraformDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Calculate relative path
		relPath, err := filepath.Rel(m.TerraformDir, path)
		if err != nil {
			return err
		}
		
		// Target path in working directory
		targetPath := filepath.Join(m.WorkingDir, relPath)
		
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		
		// Copy file
		return m.copyFile(path, targetPath)
	})
}

// copyFile copies a single file
func (m *Manager) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()
	
	_, err = io.Copy(destFile, sourceFile)
	return err
}

// initTerraform initializes the Terraform working directory
func (m *Manager) initTerraform() error {
	fmt.Println("🔧 Initializing Terraform...")
	cmd := exec.Command("terraform", "init")
	cmd.Dir = m.WorkingDir
	cmd.Env = append(os.Environ(), m.getTerraformEnv()...)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("terraform init failed: %w\nOutput: %s", err, string(output))
	}
	
	return nil
}

// createTerraformVars creates the terraform.tfvars file
func (m *Manager) createTerraformVars(options *DeploymentOptions) error {
	vars := fmt.Sprintf(`# AutoMock Terraform Variables
# Generated automatically - do not edit manually

project_name = "%s"
environment = "%s"
aws_region = "%s"
instance_size = "%s"
ttl_hours = %d
enable_ttl_cleanup = %t
`, m.ProjectName, m.Environment, m.Region, options.InstanceSize, options.TTLHours, options.EnableTTLCleanup)
	
	// Add optional variables
	if options.CustomDomain != "" {
		vars += fmt.Sprintf(`custom_domain = "%s"` + "\n", options.CustomDomain)
	}
	if options.HostedZoneID != "" {
		vars += fmt.Sprintf(`hosted_zone_id = "%s"` + "\n", options.HostedZoneID)
	}
	if options.NotificationEmail != "" {
		vars += fmt.Sprintf(`notification_email = "%s"` + "\n", options.NotificationEmail)
	}
	
	varsFile := filepath.Join(m.WorkingDir, "terraform.tfvars")
	return os.WriteFile(varsFile, []byte(vars), 0644)
}

// planTerraform runs terraform plan
func (m *Manager) planTerraform() error {
	cmd := exec.Command("terraform", "plan", "-out=tfplan")
	cmd.Dir = m.WorkingDir
	cmd.Env = append(os.Environ(), m.getTerraformEnv()...)
	
	// Stream output to user
	return m.runCommandWithOutput(cmd)
}

// applyTerraform runs terraform apply
func (m *Manager) applyTerraform() error {
	cmd := exec.Command("terraform", "apply", "-auto-approve", "tfplan")
	cmd.Dir = m.WorkingDir
	cmd.Env = append(os.Environ(), m.getTerraformEnv()...)
	
	// Stream output to user
	return m.runCommandWithOutput(cmd)
}

// destroyTerraform runs terraform destroy
func (m *Manager) destroyTerraform() error {
	cmd := exec.Command("terraform", "destroy", "-auto-approve")
	cmd.Dir = m.WorkingDir
	cmd.Env = append(os.Environ(), m.getTerraformEnv()...)
	
	// Stream output to user
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
	
	// Parse Terraform outputs
	var rawOutputs map[string]struct {
		Value interface{} `json:"value"`
	}
	
	if err := json.Unmarshal(output, &rawOutputs); err != nil {
		return nil, fmt.Errorf("failed to parse terraform outputs: %w", err)
	}
	
	// Convert to our output structure
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
	
	// Set AWS profile if specified
	if m.AWSProfile != "" {
		env = append(env, fmt.Sprintf("AWS_PROFILE=%s", m.AWSProfile))
	}
	
	// Disable Terraform CLI auto-upgrades for stability
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
	
	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()
	
	// Stream stderr
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
	
	// Extract version for user feedback
	version := strings.Split(string(output), "\n")[0]
	fmt.Printf("🔧 Found %s\n", version)
	
	return nil
}
