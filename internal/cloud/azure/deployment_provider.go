package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hemantobora/auto-mock/internal/models"
)

// ── MockServer deployment metadata ───────────────────────────────────────────
// All metadata is stored as JSON blobs in the same container as the project
// config — exactly the same key paths as the AWS implementation.
// The blob key "deployment-metadata.json" (at the root of the container) signals
// that a MockServer stack is currently deployed.

const deploymentMetadataKey = "deployment-metadata.json"

// SaveDeploymentMetadata persists infrastructure output metadata to Blob Storage.
func (p *Provider) SaveDeploymentMetadata(output *models.InfrastructureOutputs) error {
	metadata := &models.DeploymentMetadata{
		ProjectName:      p.projectID,
		DeploymentStatus: "deployed",
		DeployedAt:       time.Now().UTC(),
		CustomDomain:     output.CustomDomain,
		CreateHostedZone: output.CreateHostedZone,
		PrivateALB:       output.PrivateALB,
		Details:          output,
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal deployment metadata: %w", err)
	}
	if err := p.putBlob(context.Background(), deploymentMetadataKey, data, "application/json"); err != nil {
		return fmt.Errorf("failed to upload deployment metadata: %w", err)
	}
	return nil
}

// GetDeploymentMetadata retrieves the deployment metadata blob.
func (p *Provider) GetDeploymentMetadata() (*models.DeploymentMetadata, error) {
	data, err := p.getBlob(context.Background(), deploymentMetadataKey)
	if err != nil {
		return nil, fmt.Errorf("failed to download deployment metadata: %w", err)
	}
	var metadata models.DeploymentMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
	}
	return &metadata, nil
}

// DeleteDeploymentMetadata removes the deployment metadata blob.
func (p *Provider) DeleteDeploymentMetadata() error {
	return p.deleteSingleBlob(context.Background(), deploymentMetadataKey)
}

// IsDeployed returns true when a deployment-metadata.json blob exists and its
// status is "deployed".
func (p *Provider) IsDeployed() (bool, error) {
	metadata, err := p.GetDeploymentMetadata()
	if err != nil {
		return false, err
	}
	return metadata.DeploymentStatus == "deployed", nil
}

// ── Locust (load test) deployment metadata ────────────────────────────────────
// Same pattern — a separate well-known key signals a deployed Locust stack.

const loadTestDeploymentMetadataKey = "deployment-metadata-loadtest.json"

// SaveLoadTestDeploymentMetadata persists Locust deployment metadata.
func (p *Provider) SaveLoadTestDeploymentMetadata(output *models.LoadTestDeploymentOutputs) error {
	md := &models.LoadTestDeploymentMetadata{
		ProjectName:      p.projectID,
		DeploymentStatus: "deployed",
		DeployedAt:       time.Now().UTC(),
		Details:          output,
	}
	data, err := json.MarshalIndent(md, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal loadtest metadata: %w", err)
	}
	if err := p.putBlob(context.Background(), loadTestDeploymentMetadataKey, data, "application/json"); err != nil {
		return fmt.Errorf("upload loadtest metadata: %w", err)
	}
	return nil
}

// GetLoadTestDeploymentMetadata retrieves the Locust deployment metadata.
func (p *Provider) GetLoadTestDeploymentMetadata() (*models.LoadTestDeploymentMetadata, error) {
	data, err := p.getBlob(context.Background(), loadTestDeploymentMetadataKey)
	if err != nil {
		return nil, fmt.Errorf("get loadtest metadata: %w", err)
	}
	var md models.LoadTestDeploymentMetadata
	if err := json.Unmarshal(data, &md); err != nil {
		return nil, fmt.Errorf("unmarshal loadtest metadata: %w", err)
	}
	return &md, nil
}

// DeleteLoadTestDeploymentMetadata removes the Locust deployment metadata blob.
func (p *Provider) DeleteLoadTestDeploymentMetadata() error {
	return p.deleteSingleBlob(context.Background(), loadTestDeploymentMetadataKey)
}

// FillLoadTestOptions injects Azure-specific fields into the options struct
// before terraform.tfvars is rendered.
//
// Azure uses a two-part storage identity (account + container) instead of a
// single bucket name, and requires subscription/resource-group context that
// has no AWS equivalent.  The manager pre-populates BucketName with the
// container name (from GetStorageName); we overwrite it here with the account
// name because the Azure loadtest Terraform module expects storage_account_name.
func (p *Provider) FillLoadTestOptions(opts *models.LoadTestDeploymentOptions) {
	opts.BucketName           = p.accountName    // storage_account_name in tfvars
	opts.StorageContainerName = p.containerName  // container_name in tfvars
	opts.SubscriptionID       = p.subscriptionID
	opts.ResourceGroup        = p.resourceGroup
}
