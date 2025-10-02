// internal/state/deployment_metadata.go
// Deployment metadata storage and retrieval
package state

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// DeploymentMetadata tracks infrastructure deployment information
type DeploymentMetadata struct {
	ProjectName      string                 `json:"project_name"`
	DeploymentStatus string                 `json:"deployment_status"` // none, deploying, deployed, failed, destroyed
	DeployedAt       time.Time              `json:"deployed_at,omitempty"`
	DestroyedAt      time.Time              `json:"destroyed_at,omitempty"`
	TTLHours         int                    `json:"ttl_hours"`
	TTLExpiry        time.Time              `json:"ttl_expiry,omitempty"`
	Infrastructure   InfrastructureInfo     `json:"infrastructure"`
	Options          DeploymentOptions      `json:"options"`
	Outputs          map[string]interface{} `json:"outputs,omitempty"`
}

// InfrastructureInfo contains details about deployed resources
type InfrastructureInfo struct {
	ClusterName   string `json:"cluster_name"`
	ServiceName   string `json:"service_name"`
	ALBDNS        string `json:"alb_dns"`
	MockServerURL string `json:"mockserver_url"`
	DashboardURL  string `json:"dashboard_url"`
	VPCId         string `json:"vpc_id"`
	Region        string `json:"region"`
}

// DeploymentOptions contains the options used for deployment
type DeploymentOptions struct {
	InstanceSize      string `json:"instance_size"`
	MinTasks          int    `json:"min_tasks"`
	MaxTasks          int    `json:"max_tasks"`
	CustomDomain      string `json:"custom_domain,omitempty"`
	NotificationEmail string `json:"notification_email,omitempty"`
}

// SaveDeploymentMetadata saves deployment metadata to S3
func (s *S3Store) SaveDeploymentMetadata(ctx context.Context, metadata *DeploymentMetadata) error {
	key := "deployment-metadata.json"

	// Marshal to JSON
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Upload to S3 using putObject helper
	if err := s.putObject(ctx, key, data, "application/json"); err != nil {
		return fmt.Errorf("failed to upload metadata: %w", err)
	}

	return nil
}

// GetDeploymentMetadata retrieves deployment metadata from S3
func (s *S3Store) GetDeploymentMetadata(ctx context.Context) (*DeploymentMetadata, error) {
	key := "deployment-metadata.json"

	// Download from S3
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
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
	var metadata DeploymentMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// UpdateDeploymentStatus updates only the deployment status
func (s *S3Store) UpdateDeploymentStatus(ctx context.Context, status string) error {
	// Get existing metadata
	metadata, err := s.GetDeploymentMetadata(ctx)
	if err != nil {
		// If doesn't exist, create new
		metadata = &DeploymentMetadata{
			ProjectName:      s.bucketName,
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

	return s.SaveDeploymentMetadata(ctx, metadata)
}

// DeleteDeploymentMetadata removes deployment metadata from S3
func (s *S3Store) DeleteDeploymentMetadata(ctx context.Context) error {
	key := "deployment-metadata.json"

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	return err
}

// IsDeployed checks if infrastructure is currently deployed
func (s *S3Store) IsDeployed(ctx context.Context) (bool, error) {
	metadata, err := s.GetDeploymentMetadata(ctx)
	if err != nil {
		// If metadata doesn't exist, infrastructure is not deployed
		return false, nil
	}

	return metadata.DeploymentStatus == "deployed", nil
}

// GetTTLRemaining returns remaining TTL time, or 0 if no TTL or expired
func (s *S3Store) GetTTLRemaining(ctx context.Context) (time.Duration, error) {
	metadata, err := s.GetDeploymentMetadata(ctx)
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
func (s *S3Store) ExtendTTL(ctx context.Context, additionalHours int) error {
	metadata, err := s.GetDeploymentMetadata(ctx)
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
	return s.SaveDeploymentMetadata(ctx, metadata)
}
