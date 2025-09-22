// internal/cloud/aws/provider.go
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/hemantobora/auto-mock/internal/utils"
	"github.com/hemantobora/auto-mock/internal/provider"
)


// Provider holds AWS-specific clients and config
type Provider struct {
   ProjectName string
   BucketName  string
   S3Client    *s3.Client
   AWSConfig   aws.Config
}

// loadAWSConfig loads AWS config with optional profile
func loadAWSConfig(profile string) (aws.Config, error) {
   optFns := []func(*config.LoadOptions) error{}
   if profile != "" {
	   optFns = append(optFns, config.WithSharedConfigProfile(profile))
   }
   cfg, err := config.LoadDefaultConfig(context.TODO(), optFns...)
   if err != nil {
	   return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
   }
   if cfg.Region == "" {
	   cfg.Region = "us-east-1"
   }
   return cfg, nil
}

// Exported for use in manager.go
var LoadAWSConfig = loadAWSConfig

func ListBucketsWithPrefix(profile, prefix string) ([]string, error) {
   cfg, err := loadAWSConfig(profile)
   if err != nil {
	   return nil, err
   }
   s3Client := s3.NewFromConfig(cfg)
   out, err := s3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
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

// Ensure aws.Provider implements provider.Provider
var _ provider.Provider = (*Provider)(nil)

// NewProvider initializes the AWS SDK and returns an AWS Provider instance
func NewProvider(profile, projectName string) (*Provider, error) {
   cfg, err := loadAWSConfig(profile)
   if err != nil {
	   return nil, err
   }
   s3Client := s3.NewFromConfig(cfg)
   bucketName := utils.GetBucketName(strings.ToLower(projectName))
   return &Provider{
	   ProjectName: projectName,
	   BucketName:  bucketName,
	   S3Client:    s3Client,
	   AWSConfig:   cfg,
   }, nil
}

// InitProject creates an S3 bucket for the project if it does not exist
// Optionally can deploy complete infrastructure if requested
func (p *Provider) InitProject() error {
	// For now, keep the existing S3-only behavior
	// Users can optionally upgrade to complete infrastructure
	return p.initS3Project()
}

// initS3Project creates just the S3 bucket (existing behavior)
func (p *Provider) initS3Project() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := p.S3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: &p.BucketName,
	})
	if err == nil {
		cleanName := utils.ExtractUserProjectName(p.ProjectName)
		fmt.Printf("✅ Project already initialized: %s\n", cleanName)
		return nil
	}

	var input *s3.CreateBucketInput
	if p.AWSConfig.Region == "us-east-1" {
		input = &s3.CreateBucketInput{
			Bucket: &p.BucketName,
		}
	} else {
		input = &s3.CreateBucketInput{
			Bucket: &p.BucketName,
			CreateBucketConfiguration: &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(p.AWSConfig.Region),
			},
		}
	}

	_, err = p.S3Client.CreateBucket(ctx, input)


	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "BucketAlreadyOwnedByYou":
				cleanName := utils.ExtractUserProjectName(p.ProjectName)
				fmt.Printf("✅ Project already initialized: %s\n", cleanName)
				return nil
			case "BucketAlreadyExists":
				return fmt.Errorf("❌ bucket name '%s' already taken globally — choose a more unique project name", p.BucketName)
			}
		}
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	fmt.Println("✅ Project initialized:", utils.ExtractUserProjectName(p.ProjectName))
	return nil
}
