// internal/terraform/status.go
// Status checking for deployed infrastructure
package terraform

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// GetCurrentStatus retrieves the current status of deployed infrastructure
func (m *Manager) GetCurrentStatus() (*InfrastructureOutputs, error) {
	// Prepare workspace
	if err := m.prepareWorkspace(); err != nil {
		return nil, fmt.Errorf("failed to prepare workspace: %w", err)
	}
	defer m.cleanup()

	// Initialize Terraform (required to read outputs)
	if err := m.initTerraform(); err != nil {
		return nil, fmt.Errorf("failed to initialize terraform: %w", err)
	}

	// Create minimal terraform.tfvars for state reading
	options := DefaultDeploymentOptions()
	if err := m.createTerraformVars(options); err != nil {
		return nil, fmt.Errorf("failed to create terraform vars: %w", err)
	}

	// Try to get outputs
	outputs, err := m.getOutputs()
	if err != nil {
		return nil, fmt.Errorf("no infrastructure found or state is invalid: %w", err)
	}

	return outputs, nil
}

// CheckInfrastructureExists checks if infrastructure is deployed for a project
func (m *Manager) CheckInfrastructureExists() (bool, error) {
	// Check if state file exists in S3
	cmd := exec.Command("aws", "s3", "ls",
		fmt.Sprintf("s3://auto-mock-terraform-state-%s/projects/%s/terraform.tfstate",
			m.Region, m.ProjectName))

	if m.AWSProfile != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_PROFILE=%s", m.AWSProfile))
	}

	err := cmd.Run()
	return err == nil, nil
}

// GetInfrastructureSummary provides a quick summary without full Terraform init
func (m *Manager) GetInfrastructureSummary() (map[string]interface{}, error) {
	// This is a lightweight check that doesn't require Terraform
	// Check if key AWS resources exist

	summary := make(map[string]interface{})

	// Check ECS cluster
	clusterName := fmt.Sprintf("automock-%s-%s", m.ProjectName, m.Environment)
	cmd := exec.Command("aws", "ecs", "describe-clusters",
		"--clusters", clusterName,
		"--region", m.Region,
		"--output", "json")

	if m.AWSProfile != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_PROFILE=%s", m.AWSProfile))
	}

	output, err := cmd.Output()
	if err != nil {
		summary["ecs_cluster"] = "not found"
	} else {
		var result map[string]interface{}
		json.Unmarshal(output, &result)

		if clusters, ok := result["clusters"].([]interface{}); ok && len(clusters) > 0 {
			summary["ecs_cluster"] = "exists"
			cluster := clusters[0].(map[string]interface{})
			summary["cluster_status"] = cluster["status"]
			summary["running_tasks"] = cluster["runningTasksCount"]
		} else {
			summary["ecs_cluster"] = "not found"
		}
	}

	// Check S3 bucket
	cmd = exec.Command("aws", "s3api", "list-buckets",
		"--query", fmt.Sprintf("Buckets[?starts_with(Name, 'auto-mock-%s')].Name", m.ProjectName),
		"--output", "json")

	if m.AWSProfile != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_PROFILE=%s", m.AWSProfile))
	}

	output, err = cmd.Output()
	if err == nil {
		var buckets []string
		json.Unmarshal(output, &buckets)
		if len(buckets) > 0 {
			summary["s3_bucket"] = buckets[0]
		} else {
			summary["s3_bucket"] = "not found"
		}
	}

	return summary, nil
}
