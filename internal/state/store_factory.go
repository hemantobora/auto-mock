// Package state provides factory functions for creating storage instances
package state

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// StoreForProject creates a new S3Store instance for the given project
// This is the centralized way to create stores throughout the application
func StoreForProject(ctx context.Context, projectName string) (*S3Store, error) {
	return StoreForProjectWithProfile(ctx, projectName, "")
}

// StoreForProjectWithProfile creates a new S3Store instance with a specific AWS profile
// Use this when you need to specify a custom AWS profile
func StoreForProjectWithProfile(ctx context.Context, projectName, awsProfile string) (*S3Store, error) {
	// Load AWS configuration
	var cfg aws.Config
	var err error

	if awsProfile != "" {
		cfg, err = config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(awsProfile))
	} else {
		cfg, err = config.LoadDefaultConfig(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(cfg)

	// Create and return store
	return CreateS3Store(s3Client, projectName), nil
}

// CreateS3StoreWithBucket creates a new S3Store instance with a specific bucket name
// Use this when you already know the exact bucket name (e.g., from Terraform outputs)
func CreateS3StoreWithBucket(ctx context.Context, projectName, bucketName, awsProfile string) (*S3Store, error) {
	// Load AWS configuration
	var cfg aws.Config
	var err error

	if awsProfile != "" {
		cfg, err = config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(awsProfile))
	} else {
		cfg, err = config.LoadDefaultConfig(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(cfg)

	// Create store with specific bucket name
	return &S3Store{
		client:     s3Client,
		bucketName: bucketName,
	}, nil
}
