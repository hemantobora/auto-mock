// Package state provides S3-based storage implementation
package state

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
	"github.com/hemantobora/auto-mock/internal/utils"
)

// S3Store implements the Store interface using AWS S3
type S3Store struct {
	client     *s3.Client
	bucketName string
}

// NewS3Store creates a new S3-based store
func NewS3Store(client *s3.Client, projectName string) *S3Store {
	return &S3Store{
		client:     client,
		bucketName: utils.GetBucketName(projectName),
	}
}

// SaveConfig saves a mock configuration to S3
func (s *S3Store) SaveConfig(ctx context.Context, projectID string, config *MockConfiguration) error {
	// Validate configuration
	if err := ValidateConfiguration(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Set metadata
	config.Metadata.ProjectID = projectID
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
	key := fmt.Sprintf("configs/%s/current.json", projectID)
	if err := s.putObject(ctx, key, jsonData, "application/json"); err != nil {
		return fmt.Errorf("failed to save current config: %w", err)
	}

	// Save versioned copy
	versionKey := fmt.Sprintf("configs/%s/versions/%s.json", projectID, config.Metadata.Version)
	if err := s.putObject(ctx, versionKey, jsonData, "application/json"); err != nil {
		// Log warning but don't fail - current version is saved
		fmt.Printf("Warning: failed to save version %s: %v\n", config.Metadata.Version, err)
	}

	// Save metadata index
	if err := s.updateMetadataIndex(ctx, projectID, config.Metadata); err != nil {
		fmt.Printf("Warning: failed to update metadata index: %v\n", err)
	}

	return nil
}

// GetConfig retrieves the current mock configuration
func (s *S3Store) GetConfig(ctx context.Context, projectID string) (*MockConfiguration, error) {
	key := fmt.Sprintf("configs/%s/current.json", projectID)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
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

	var config MockConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// GetConfigVersion retrieves a specific version of a configuration
func (s *S3Store) GetConfigVersion(ctx context.Context, projectID, version string) (*MockConfiguration, error) {
	key := fmt.Sprintf("configs/%s/versions/%s.json", projectID, version)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
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

	var config MockConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version: %w", err)
	}

	return &config, nil
}

// ListConfigs lists all configurations with metadata
func (s *S3Store) ListConfigs(ctx context.Context) ([]ConfigMetadata, error) {
	prefix := "configs/"

	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list configs: %w", err)
	}

	// Extract unique project IDs
	projectMap := make(map[string]bool)
	for _, obj := range result.Contents {
		key := aws.ToString(obj.Key)
		if strings.HasSuffix(key, "/current.json") {
			parts := strings.Split(key, "/")
			if len(parts) >= 3 {
				projectMap[parts[1]] = true
			}
		}
	}

	// Get metadata for each project
	var configs []ConfigMetadata
	for projectID := range projectMap {
		// Try to get metadata from index
		metadata, err := s.getMetadataFromIndex(ctx, projectID)
		if err != nil {
			// Fall back to reading the actual config
			config, err := s.GetConfig(ctx, projectID)
			if err != nil {
				continue // Skip this project
			}
			metadata = &config.Metadata
		}
		configs = append(configs, *metadata)
	}

	return configs, nil
}

// DeleteConfig removes a configuration and all its versions
func (s *S3Store) DeleteConfig(ctx context.Context, projectID string) error {
	// List all objects for this project
	prefix := fmt.Sprintf("configs/%s/", projectID)

	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return fmt.Errorf("failed to list objects for deletion: %w", err)
	}

	// Delete all objects
	for _, obj := range result.Contents {
		_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucketName),
			Key:    obj.Key,
		})
		if err != nil {
			fmt.Printf("Warning: failed to delete %s: %v\n", aws.ToString(obj.Key), err)
		}
	}

	// Delete metadata index
	metadataKey := fmt.Sprintf("metadata/%s.json", projectID)
	_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(metadataKey),
	})

	return nil
}

// UpdateConfig updates an existing configuration
func (s *S3Store) UpdateConfig(ctx context.Context, projectID string, config *MockConfiguration) error {
	// Get existing config to preserve creation time
	existing, err := s.GetConfig(ctx, projectID)
	if err == nil {
		config.Metadata.CreatedAt = existing.Metadata.CreatedAt
	}

	// Generate new version
	config.Metadata.Version = fmt.Sprintf("v%d", time.Now().Unix())

	// Save the updated config
	return s.SaveConfig(ctx, projectID, config)
}

// GetConfigHistory retrieves version history for a project
func (s *S3Store) GetConfigHistory(ctx context.Context, projectID string) ([]ConfigMetadata, error) {
	prefix := fmt.Sprintf("configs/%s/versions/", projectID)

	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	var history []ConfigMetadata
	for _, obj := range result.Contents {
		key := aws.ToString(obj.Key)
		parts := strings.Split(key, "/")
		if len(parts) >= 4 {
			versionName := strings.TrimSuffix(parts[3], ".json")
			metadata := ConfigMetadata{
				ProjectID: projectID,
				Version:   versionName,
				UpdatedAt: aws.ToTime(obj.LastModified),
				Size:      aws.ToInt64(obj.Size),
			}
			history = append(history, metadata)
		}
	}

	return history, nil
}

// Helper methods

func (s *S3Store) putObject(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(s.bucketName),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(data),
		ContentType:          aws.String(contentType),
		ServerSideEncryption: types.ServerSideEncryption("AES256"),
	})
	return err
}

func (s *S3Store) updateMetadataIndex(ctx context.Context, projectID string, metadata ConfigMetadata) error {
	key := fmt.Sprintf("metadata/%s.json", projectID)

	jsonData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return s.putObject(ctx, key, jsonData, "application/json")
}

func (s *S3Store) getMetadataFromIndex(ctx context.Context, projectID string) (*ConfigMetadata, error) {
	key := fmt.Sprintf("metadata/%s.json", projectID)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
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

	var metadata ConfigMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// QuickSave saves raw MockServer JSON quickly without full validation
func (s *S3Store) QuickSave(ctx context.Context, projectID string, mockServerJSON string, provider string) error {
	// Parse to validate it's proper JSON
	config, err := ParseMockServerJSON(mockServerJSON)
	if err != nil {
		return fmt.Errorf("invalid MockServer JSON: %w", err)
	}

	// Set metadata
	config.Metadata.ProjectID = projectID
	config.Metadata.Provider = provider
	config.Metadata.Description = fmt.Sprintf("Generated by %s at %s", provider, time.Now().Format(time.RFC3339))

	// Save the configuration
	return s.SaveConfig(ctx, projectID, config)
}
