package cloud

import (
	"context"
	"fmt"

	"github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/cloud/aws"
	"github.com/hemantobora/auto-mock/internal/cloud/azure"
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
		return f.createAzureProvider(ctx, opts)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

func (f *Factory) createAWSProvider(ctx context.Context, opts *factoryOptions) (internal.Provider, error) {
	return aws.NewProvider(ctx, aws.WithProfile(opts.profile))
}

func (f *Factory) createAzureProvider(ctx context.Context, _ *factoryOptions) (internal.Provider, error) {
	return azure.NewProvider(ctx)
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

// DetectAvailableProviders returns the provider type strings for every cloud
// provider whose credentials are locally present (e.g. ~/.aws/credentials or
// ~/.azure/azureProfile.json). This is intentionally lightweight — no API
// calls are made. The actual provider (resource group, storage account, etc.)
// is initialised later via CreateProvider, once the user has chosen.
func (f *Factory) DetectAvailableProviders(ctx context.Context, profile string) []string {
	var available []string

	if _, err := aws.ValidateCredentials(ctx, profile); err == nil {
		available = append(available, "aws")
	}

	if _, err := azure.ValidateCredentials(ctx); err == nil {
		available = append(available, "azure")
	}

	// TODO: GCP

	return available
}
