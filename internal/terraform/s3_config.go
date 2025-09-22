// internal/terraform/s3_config.go
package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3ConfigManager handles MockServer configuration in S3
type S3ConfigManager struct {
	s3Client   *s3.Client
	bucketName string
	region     string
}

// NewS3ConfigManager creates a new S3 configuration manager
func NewS3ConfigManager(bucketName, region, awsProfile string) (*S3ConfigManager, error) {
	// Load AWS config
	optFns := []func(*config.LoadOptions) error{}
	if awsProfile != "" {
		optFns = append(optFns, config.WithSharedConfigProfile(awsProfile))
	}
	
	cfg, err := config.LoadDefaultConfig(context.TODO(), optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	
	if region != "" {
		cfg.Region = region
	}
	
	s3Client := s3.NewFromConfig(cfg)
	
	return &S3ConfigManager{
		s3Client:   s3Client,
		bucketName: bucketName,
		region:     region,
	}, nil
}

// UploadExpectations uploads MockServer expectations to S3
func (m *S3ConfigManager) UploadExpectations(expectations interface{}) error {
	// Convert expectations to JSON
	expectationsJSON, err := json.MarshalIndent(expectations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal expectations: %w", err)
	}
	
	// Upload to S3
	_, err = m.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(m.bucketName),
		Key:         aws.String("expectations.json"),
		Body:        strings.NewReader(string(expectationsJSON)),
		ContentType: aws.String("application/json"),
	})
	
	if err != nil {
		return fmt.Errorf("failed to upload expectations to S3: %w", err)
	}
	
	fmt.Printf("✅ Uploaded expectations to s3://%s/expectations.json\n", m.bucketName)
	return nil
}

// DownloadExpectations downloads current MockServer expectations from S3
func (m *S3ConfigManager) DownloadExpectations() ([]map[string]interface{}, error) {
	// Download from S3
	result, err := m.s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(m.bucketName),
		Key:    aws.String("expectations.json"),
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to download expectations from S3: %w", err)
	}
	defer result.Body.Close()
	
	// Parse JSON
	var expectations []map[string]interface{}
	decoder := json.NewDecoder(result.Body)
	if err := decoder.Decode(&expectations); err != nil {
		return nil, fmt.Errorf("failed to parse expectations JSON: %w", err)
	}
	
	return expectations, nil
}

// UpdateProjectMetadata updates project metadata in S3
func (m *S3ConfigManager) UpdateProjectMetadata(metadata map[string]interface{}) error {
	// Convert metadata to JSON
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	// Upload to S3
	_, err = m.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(m.bucketName),
		Key:         aws.String("project-metadata.json"),
		Body:        strings.NewReader(string(metadataJSON)),
		ContentType: aws.String("application/json"),
	})
	
	if err != nil {
		return fmt.Errorf("failed to upload metadata to S3: %w", err)
	}
	
	return nil
}

// CreateVersionBackup creates a versioned backup of current expectations
func (m *S3ConfigManager) CreateVersionBackup() error {
	// Get current expectations
	expectations, err := m.DownloadExpectations()
	if err != nil {
		return fmt.Errorf("failed to download current expectations: %w", err)
	}
	
	// List existing versions to determine next version number
	versions, err := m.listVersions()
	if err != nil {
		return fmt.Errorf("failed to list versions: %w", err)
	}
	
	nextVersion := len(versions) + 1
	versionKey := fmt.Sprintf("versions/expectations-v%d.json", nextVersion)
	
	// Convert to JSON
	expectationsJSON, err := json.MarshalIndent(expectations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal expectations: %w", err)
	}
	
	// Upload version
	_, err = m.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(m.bucketName),
		Key:         aws.String(versionKey),
		Body:        strings.NewReader(string(expectationsJSON)),
		ContentType: aws.String("application/json"),
	})
	
	if err != nil {
		return fmt.Errorf("failed to upload version backup: %w", err)
	}
	
	fmt.Printf("✅ Created version backup: %s\n", versionKey)
	return nil
}

// listVersions lists all version backups
func (m *S3ConfigManager) listVersions() ([]string, error) {
	result, err := m.s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(m.bucketName),
		Prefix: aws.String("versions/"),
	})
	
	if err != nil {
		return nil, err
	}
	
	var versions []string
	for _, obj := range result.Contents {
		if obj.Key != nil && strings.HasSuffix(*obj.Key, ".json") {
			versions = append(versions, *obj.Key)
		}
	}
	
	return versions, nil
}

// GetS3ConfigManager creates a config manager from infrastructure outputs
func GetS3ConfigManager(outputs *InfrastructureOutputs, awsProfile string) (*S3ConfigManager, error) {
	if outputs.ConfigBucket == "" {
		return nil, fmt.Errorf("no configuration bucket found in infrastructure outputs")
	}
	
	// Default region
	region := "us-east-1"
	
	return NewS3ConfigManager(outputs.ConfigBucket, region, awsProfile)
}
