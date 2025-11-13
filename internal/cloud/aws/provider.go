// internal/cloud/aws/provider.go
package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"

	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/cloud/naming"
	"github.com/hemantobora/auto-mock/internal/models"
)

// Provider holds AWS-specific clients and config
type Provider struct {
	projectID  string
	naming     internal.NamingStrategy
	region     string
	BucketName string
	S3Client   *s3.Client
	AWSConfig  aws.Config
}

// ProviderOption is a functional option for provider configuration
type ProviderOption func(*providerOptions)

type providerOptions struct {
	profile string
	region  string
}

// WithRegion specifies the AWS region
func WithRegion(region string) ProviderOption {
	return func(o *providerOptions) {
		o.region = region
	}
}

// WithProfile specifies the AWS profile to use
func WithProfile(profile string) ProviderOption {
	return func(o *providerOptions) {
		o.profile = profile
	}
}

// loadAWSConfig loads AWS configuration with optional profile
func loadAWSConfig(ctx context.Context, profile string) (aws.Config, error) {
	optFns := []func(*config.LoadOptions) error{}
	if profile != "" {
		optFns = append(optFns, config.WithSharedConfigProfile(profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return aws.Config{}, &models.ProviderError{
			Provider:  "aws",
			Operation: "load-config",
			Resource:  fmt.Sprintf("profile:%s", profile),
			Cause:     fmt.Errorf("failed to load AWS config: %w", err),
		}
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return cfg, nil
}

// NewProvider creates a new S3 storage provider
func NewProvider(ctx context.Context, options ...ProviderOption) (*Provider, error) {
	// Apply options
	opts := &providerOptions{}
	for _, opt := range options {
		opt(opts)
	}

	// Load AWS configuration
	cfg, err := loadAWSConfig(ctx, opts.profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Override region if specified
	if opts.region != "" {
		cfg.Region = opts.region
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(cfg)

	// Create provider
	naming := naming.NewDefaultNaming()
	provider := &Provider{
		S3Client:  s3Client,
		naming:    naming,
		region:    cfg.Region,
		AWSConfig: cfg,
	}
	return provider, nil
}

// ValidateCredentials checks if AWS credentials are valid
func ValidateCredentials(ctx context.Context, profile string) (bool, error) {
	cfg, err := loadAWSConfig(ctx, profile)
	if err != nil {
		return false, err
	}

	client := sts.NewFromConfig(cfg)
	_, err = client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return false, err
	}

	return true, nil
}

// GetProviderType returns the provider type
func (p *Provider) GetProviderType() string {
	return "aws"
}

func (p *Provider) ValidateProjectName(projectID string) error {
	return p.naming.ValidateProjectID(projectID)
}

func (p *Provider) GetStorageName() string {
	return p.BucketName
}

func (p *Provider) GetProjectName() string {
	return p.projectID
}

func (p *Provider) SetStorageName(name string) {
	p.BucketName = name
}

func (p *Provider) SetProjectName(name string) {
	p.projectID = name
}

func (p *Provider) GetRegion() string {
	return p.region
}

func (p *Provider) InitProject(ctx context.Context, projectID string) error {
	// Check if bucket exists
	bucketName := p.naming.GenerateStorageName(projectID)
	_, err := p.S3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err == nil {
		// Bucket exists
		p.projectID = projectID
		p.BucketName = bucketName
		// Detect and align region to avoid 301 PermanentRedirect on PutObject
		if loc, lerr := p.S3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(bucketName)}); lerr == nil {
			resolved := string(loc.LocationConstraint)
			if resolved == "" { // us-east-1 returns empty per API
				resolved = "us-east-1"
			}
			if resolved != "" && resolved != p.region {
				p.region = resolved
				// Rebuild S3 client bound to correct region to prevent redirects
				cfg := p.AWSConfig
				cfg.Region = resolved
				p.S3Client = s3.NewFromConfig(cfg)
			}
		}
		fmt.Printf("✅ Project already initialized: %s\n", projectID)
		return nil
	}

	// Create bucket
	var input *s3.CreateBucketInput
	if p.region == "us-east-1" {
		input = &s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
		}
	} else {
		input = &s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
			CreateBucketConfiguration: &types.CreateBucketConfiguration{
				LocationConstraint: types.BucketLocationConstraint(p.region),
			},
		}
	}

	_, err = p.S3Client.CreateBucket(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "BucketAlreadyOwnedByYou":
				fmt.Printf("✅ Project already initialized: %s\n", projectID)
				return nil
			case "BucketAlreadyExists":
				return &models.ProviderError{
					Provider:  "aws",
					Operation: "init",
					Resource:  bucketName,
					Cause:     fmt.Errorf("bucket name '%s' already taken globally — choose a more unique project name", bucketName),
				}
			}
		}
		return &models.ProviderError{
			Provider:  "aws",
			Operation: "init",
			Resource:  bucketName,
			Cause:     fmt.Errorf("failed to create bucket: %w", err),
		}
	}
	fmt.Println("✅ Project initialized:", projectID)
	p.projectID = projectID
	p.BucketName = bucketName
	return nil
}
