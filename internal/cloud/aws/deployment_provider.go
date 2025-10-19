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
func (p *Provider) SaveDeploymentMetadata(ctx context.Context, metadata *models.DeploymentMetadata) error {
	key := "deployment-metadata.json"

	// Marshal to JSON
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Upload to S3 using putObject helper
	if err := p.putObject(ctx, key, data, "application/json"); err != nil {
		return fmt.Errorf("failed to upload metadata: %w", err)
	}

	return nil
}

// GetDeploymentMetadata retrieves deployment metadata from S3
func (p *Provider) GetDeploymentMetadata(ctx context.Context) (*models.DeploymentMetadata, error) {
	key := "deployment-metadata.json"

	// Download from S3
	result, err := p.S3Client.GetObject(ctx, &s3.GetObjectInput{
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

// UpdateDeploymentStatus updates only the deployment status
func (p *Provider) UpdateDeploymentStatus(ctx context.Context, status string) error {
	// Get existing metadata
	metadata, err := p.GetDeploymentMetadata(ctx)
	if err != nil {
		// If doesn't exist, create new
		metadata = &models.DeploymentMetadata{
			ProjectName:      p.BucketName,
			DeploymentStatus: status,
		}
	} else {
		metadata.DeploymentStatus = status
	}

	// Update timestamps
	switch status {
	case "deployed":
		metadata.DeployedAt = time.Now()
	case "destroyed":
		metadata.DestroyedAt = time.Now()
	}

	return p.SaveDeploymentMetadata(ctx, metadata)
}

// DeleteDeploymentMetadata removes deployment metadata from S3
func (p *Provider) DeleteDeploymentMetadata(ctx context.Context) error {
	key := "deployment-metadata.json"

	_, err := p.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.BucketName),
		Key:    aws.String(key),
	})

	return err
}

// IsDeployed checks if infrastructure is currently deployed
func (p *Provider) IsDeployed(ctx context.Context) (bool, error) {
	metadata, err := p.GetDeploymentMetadata(ctx)
	if err != nil {
		// If metadata doesn't exist, infrastructure is not deployed
		return false, nil
	}

	return metadata.DeploymentStatus == "deployed", nil
}

// GetTTLRemaining returns remaining TTL time, or 0 if no TTL or expired
func (p *Provider) GetTTLRemaining(ctx context.Context) (time.Duration, error) {
	metadata, err := p.GetDeploymentMetadata(ctx)
	if err != nil {
		return 0, err
	}

	if metadata.TTLExpiry.IsZero() {
		return 0, nil // No TTL set
	}

	remaining := time.Until(metadata.TTLExpiry)
	if remaining < 0 {
		return 0, nil // Already expired
	}

	return remaining, nil
}

// ExtendTTL extends the TTL by adding hours
func (p *Provider) ExtendTTL(ctx context.Context, additionalHours int) error {
	metadata, err := p.GetDeploymentMetadata(ctx)
	if err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	if metadata.TTLExpiry.IsZero() {
		return fmt.Errorf("no TTL set for this deployment")
	}

	// Add hours to current expiry
	metadata.TTLExpiry = metadata.TTLExpiry.Add(time.Duration(additionalHours) * time.Hour)
	metadata.TTLHours += additionalHours

	// Save updated metadata
	return p.SaveDeploymentMetadata(ctx, metadata)
}
