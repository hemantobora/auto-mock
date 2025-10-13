// Package s3 provides AWS S3 storage implementation
package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/hemantobora/auto-mock/internal/models"
)

// SaveConfig saves a mock configuration to S3
func (p *Provider) SaveConfig(ctx context.Context, config *models.MockConfiguration) error {
	// Validate configuration
	if err := models.ValidateConfiguration(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Set metadata
	cleanProjectID := p.naming.ExtractProjectID(p.projectID)
	config.Metadata.ProjectID = cleanProjectID
	config.Metadata.UpdatedAt = time.Now()
	if config.Metadata.CreatedAt.IsZero() {
		config.Metadata.CreatedAt = time.Now()
	}
	if config.Metadata.Version == "" {
		config.Metadata.Version = fmt.Sprintf("v%d", time.Now().Unix())
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Update size in metadata
	config.Metadata.Size = int64(len(jsonData))

	// Save current version
	key := fmt.Sprintf("configs/%s/current.json", cleanProjectID)
	if err := p.putObject(ctx, key, jsonData, "application/json"); err != nil {
		return fmt.Errorf("failed to save current config: %w", err)
	}

	// Save versioned copy
	versionKey := fmt.Sprintf("configs/%s/versions/%s.json", cleanProjectID, config.Metadata.Version)
	if err := p.putObject(ctx, versionKey, jsonData, "application/json"); err != nil {
		fmt.Printf("Warning: failed to save version %s: %v\n", config.Metadata.Version, err)
	}

	// Save metadata index
	if err := p.updateMetadataIndex(ctx, cleanProjectID, config.Metadata); err != nil {
		fmt.Printf("Warning: failed to update metadata index: %v\n", err)
	}

	return nil
}

// GetConfig retrieves the current mock configuration
func (p *Provider) GetConfig(ctx context.Context, projectID string) (*models.MockConfiguration, error) {
	cleanProjectID := p.naming.ExtractProjectID(projectID)
	key := fmt.Sprintf("configs/%s/current.json", cleanProjectID)

	result, err := p.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get config from S3: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read config data: %w", err)
	}

	var config models.MockConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// UpdateConfig updates an existing configuration
func (p *Provider) UpdateConfig(ctx context.Context, config *models.MockConfiguration) error {
	// Get existing config to preserve creation time
	existing, err := p.GetConfig(ctx, config.Metadata.ProjectID)
	if err == nil {
		config.Metadata.CreatedAt = existing.Metadata.CreatedAt
	}

	// Generate new version
	config.Metadata.Version = fmt.Sprintf("v%d", time.Now().Unix())

	// Save the updated config
	return p.SaveConfig(ctx, config)
}

// DeleteConfig removes a configuration and all its versions
func (p *Provider) DeleteConfig(ctx context.Context, projectID string) error {
	cleanProjectID := p.naming.ExtractProjectID(projectID)
	prefix := fmt.Sprintf("configs/%s/", cleanProjectID)

	// List all objects for this project
	result, err := p.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(p.BucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return fmt.Errorf("failed to list objects for deletion: %w", err)
	}

	// Delete all objects
	for _, obj := range result.Contents {
		_, err := p.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(p.BucketName),
			Key:    obj.Key,
		})
		if err != nil {
			fmt.Printf("Warning: failed to delete %s: %v\n", aws.ToString(obj.Key), err)
		}
	}

	// Delete metadata index
	metadataKey := fmt.Sprintf("metadata/%s.json", cleanProjectID)
	_, _ = p.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.BucketName),
		Key:    aws.String(metadataKey),
	})

	return nil
}

// SaveVersion saves a specific version of a configuration
func (p *Provider) SaveVersion(ctx context.Context, config *models.MockConfiguration, version string) error {
	cleanProjectID := p.naming.ExtractProjectID(config.Metadata.ProjectID)
	key := fmt.Sprintf("configs/%s/versions/%s.json", cleanProjectID, version)

	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	return p.putObject(ctx, key, jsonData, "application/json")
}

// GetVersion retrieves a specific version of a configuration
func (p *Provider) GetVersion(ctx context.Context, projectID, version string) (*models.MockConfiguration, error) {
	cleanProjectID := p.naming.ExtractProjectID(projectID)
	key := fmt.Sprintf("configs/%s/versions/%s.json", cleanProjectID, version)

	result, err := p.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get version %s from S3: %w", version, err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read version data: %w", err)
	}

	var config models.MockConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version: %w", err)
	}

	return &config, nil
}

// ListVersions retrieves version history for a project
func (p *Provider) ListVersions(ctx context.Context, projectID string) ([]models.VersionInfo, error) {
	cleanProjectID := p.naming.ExtractProjectID(projectID)
	prefix := fmt.Sprintf("configs/%s/versions/", cleanProjectID)

	result, err := p.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(p.BucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	var versions []models.VersionInfo
	for _, obj := range result.Contents {
		key := aws.ToString(obj.Key)
		parts := strings.Split(key, "/")
		if len(parts) >= 4 {
			versionName := strings.TrimSuffix(parts[3], ".json")
			version := models.VersionInfo{
				Version:   versionName,
				CreatedAt: aws.ToTime(obj.LastModified),
				Size:      aws.ToInt64(obj.Size),
			}
			versions = append(versions, version)
		}
	}

	return versions, nil
}

func (p *Provider) ListProjects(ctx context.Context) ([]models.ProjectInfo, error) {
	fmt.Println("✅ Checking existence of projects")
	var projects []models.ProjectInfo

	// List all S3 buckets
	out, err := p.S3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	// Filter buckets by project ID
	for _, bucket := range out.Buckets {
		if bucket.Name == nil || !strings.HasPrefix(*bucket.Name, p.naming.GetPrefix()) {
			continue
		}
		projectID := p.naming.ExtractProjectID(aws.ToString(bucket.Name))
		projects = append(projects, models.ProjectInfo{
			ProjectID:   projectID,
			DisplayName: projectID,
			StorageName: aws.ToString(bucket.Name),
		})
	}

	return projects, nil
}

// ProjectExists checks if a project exists
func (p *Provider) ProjectExists(ctx context.Context, projectID string) (bool, error) {
	fmt.Printf("✅ Checking existence of project: %s\n", projectID)
	out, err := p.S3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return false, err
	}

	for _, bucket := range out.Buckets {
		if p.naming.ExtractProjectID(aws.ToString(bucket.Name)) == projectID {
			p.BucketName = *bucket.Name
			p.projectID = projectID
			return true, nil
		}
	}
	return false, nil
}

// GetMetadata retrieves metadata for a project
func (p *Provider) GetMetadata(ctx context.Context, projectID string) (*models.ConfigMetadata, error) {
	cleanProjectID := p.naming.ExtractProjectID(projectID)

	// Try to get from metadata index first
	metadata, err := p.getMetadataFromIndex(ctx, cleanProjectID)
	if err == nil {
		return metadata, nil
	}

	// Fall back to reading the actual config
	config, err := p.GetConfig(ctx, cleanProjectID)
	if err != nil {
		return nil, err
	}

	return &config.Metadata, nil
}

// Helper methods

func (p *Provider) putObject(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := p.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(p.BucketName),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(data),
		ContentType:          aws.String(contentType),
		ServerSideEncryption: types.ServerSideEncryption("AES256"),
	})
	return err
}

func (p *Provider) updateMetadataIndex(ctx context.Context, projectID string, metadata models.ConfigMetadata) error {
	key := fmt.Sprintf("metadata/%s.json", projectID)

	jsonData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return p.putObject(ctx, key, jsonData, "application/json")
}

func (p *Provider) getMetadataFromIndex(ctx context.Context, projectID string) (*models.ConfigMetadata, error) {
	key := fmt.Sprintf("metadata/%s.json", projectID)

	result, err := p.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}

	var metadata models.ConfigMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// ListBuckets lists all S3 buckets with a specific prefix
func ListBuckets(ctx context.Context, profile, prefix string) ([]string, error) {
	cfg, err := loadAWSConfig(ctx, profile)
	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(cfg)
	out, err := s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	var filtered []string
	for _, bucket := range out.Buckets {
		if bucket.Name != nil && strings.HasPrefix(*bucket.Name, prefix) {
			filtered = append(filtered, *bucket.Name)
		}
	}

	return filtered, nil
}
