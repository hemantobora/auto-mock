package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hemantobora/auto-mock/internal/models"
)

// SaveDeploymentMetadata saves deployment metadata to S3
func (p *Provider) SaveDeploymentMetadata(output *models.InfrastructureOutputs) error {
	key := "deployment-metadata.json"

	// Build metadata
	metadata := &models.DeploymentMetadata{
		ProjectName:      p.projectID,
		DeploymentStatus: "deployed",
		DeployedAt:       time.Now().UTC(),
		Details:          output,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Upload to S3 using putObject helper
	if err := p.putObject(context.Background(), key, data, "application/json"); err != nil {
		return fmt.Errorf("failed to upload metadata: %w", err)
	}

	return nil
}

// GetDeploymentMetadata retrieves deployment metadata from S3
func (p *Provider) GetDeploymentMetadata() (*models.DeploymentMetadata, error) {
	key := "deployment-metadata.json"

	// Download from S3
	result, err := p.S3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(p.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download metadata: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Unmarshal JSON
	var metadata models.DeploymentMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// DeleteDeploymentMetadata removes deployment metadata from S3
func (p *Provider) DeleteDeploymentMetadata() error {
	key := "deployment-metadata.json"

	_, err := p.S3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(p.BucketName),
		Key:    aws.String(key),
	})

	return err
}

// IsDeployed checks if infrastructure is currently deployed
func (p *Provider) IsDeployed() (bool, error) {
	metadata, err := p.GetDeploymentMetadata()
	if err != nil {
		// If metadata doesn't exist, infrastructure is not deployed
		return false, err
	}

	return metadata.DeploymentStatus == "deployed", nil
}
