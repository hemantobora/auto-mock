package cloud

import (
	"context"
	"fmt"

	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/cloud/aws"
	"github.com/hemantobora/auto-mock/internal/cloud/naming"
)

// Factory creates storage providers based on configuration
type Factory struct {
	naming internal.NamingStrategy
}

// NewFactory creates a new storage factory
func NewFactory() *Factory {
	return &Factory{
		naming: naming.NewDefaultNaming(),
	}
}

// CreateProvider creates a storage provider for the specified type
// Supported types: "aws", "gcp", "azure"
func (f *Factory) CreateProvider(ctx context.Context, providerType string, options ...Option) (internal.Provider, error) {
	// Apply options
	opts := &factoryOptions{
		profile: "",
	}
	for _, opt := range options {
		opt(opts)
	}

	switch providerType {
	case "aws":
		return f.createAWSProvider(ctx, opts)
	case "gcp":
		return nil, fmt.Errorf("GCP storage provider not yet implemented")
	case "azure":
		return nil, fmt.Errorf("Azure storage provider not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

func (f *Factory) createAWSProvider(ctx context.Context, opts *factoryOptions) (internal.Provider, error) {
	return aws.NewProvider(ctx, aws.WithProfile(opts.profile))
}

// Option is a functional option for factory configuration
type Option func(*factoryOptions)

type factoryOptions struct {
	profile string
}

// WithProfile specifies the cloud provider profile to use
func WithProfile(profile string) Option {
	return func(o *factoryOptions) {
		o.profile = profile
	}
}

// AutoDetectProvider attempts to detect available storage providers
// Returns the first available provider type
func (f *Factory) AutoDetectProvider(ctx context.Context, profile string) (*internal.Provider, error) {
	// Try AWS
	var available []internal.Provider
	if _, err := aws.ValidateCredentials(ctx, profile); err == nil {
		provider, _ := aws.NewProvider(ctx, aws.WithProfile(profile))
		available = append(available, provider)
	}

	// TODO: Try GCP
	// TODO: Try Azure

	if len(available) == 0 {
		return nil, fmt.Errorf("❌ No valid cloud provider credentials found. Please configure AWS, GCP, or Azure credentials. (Currently, only AWS is supported, other providers are coming soon!)")
	}
	return &available[0], nil
}
