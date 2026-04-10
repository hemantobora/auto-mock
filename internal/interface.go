package internal

import (
	"context"

	"github.com/hemantobora/auto-mock/internal/models"
)

// Provider defines the interface for storage operations
// Implementations handle cloud-specific storage (S3, GCS, Azure Blob, etc.)
type Provider interface {
	// Configuration management
	SaveConfig(ctx context.Context, config *models.MockConfiguration) error
	GetConfig(ctx context.Context, projectID string) (*models.MockConfiguration, error)
	UpdateConfig(ctx context.Context, config *models.MockConfiguration) error
	DeleteProject(projectID string) error

	// Versioning
	SaveVersion(ctx context.Context, config *models.MockConfiguration, version string) error
	GetVersion(ctx context.Context, projectID, version string) (*models.MockConfiguration, error)
	ListVersions(ctx context.Context, projectID string) ([]models.VersionInfo, error)

	// Project management
	ListProjects(ctx context.Context) ([]models.ProjectInfo, error)
	ProjectExists(ctx context.Context, projectID string) (bool, error)

	// Metadata
	GetMetadata(ctx context.Context, projectID string) (*models.ConfigMetadata, error)

	// Provider info
	GetProviderType() string // Returns "aws", "gcp", "azure", etc.

	// GetBackendConfig returns the complete Terraform backend HCL block for
	// this provider, ready to write as backend.tf. The stateKey argument is
	// the object/blob path for the state file (e.g. "terraform/state/terraform.tfstate").
	GetBackendConfig(stateKey string) string

	// Ensure resources
	InitProject(ctx context.Context, projectID string) error
	ValidateProjectName(projectID string) error

	GetStorageName() string
	GetProjectName() string
	SetStorageName(name string)
	SetProjectName(name string)
	GetRegion() string

	// Deployment metadata management
	SaveDeploymentMetadata(metadata *models.InfrastructureOutputs) error
	GetDeploymentMetadata() (*models.DeploymentMetadata, error)
	DeleteDeploymentMetadata() error
	IsDeployed() (bool, error)

	CreateDeploymentConfiguration() *models.DeploymentOptions
	DisplayCostEstimate(options *models.DeploymentOptions)
	CreateDefaultDeploymentConfiguration() *models.DeploymentOptions

	// Load test bundle management
	UploadLoadTestBundle(ctx context.Context, projectID, bundleDir string) (*models.LoadTestPointer, *models.LoadTestVersion, error)
	GetLoadTestPointer(ctx context.Context, projectID string) (*models.LoadTestPointer, error)
	DownloadLoadTestBundle(ctx context.Context, projectID, destDir string) (*models.LoadTestPointer, string, error)
	DeleteLoadTestPointer(ctx context.Context, projectID string) error

	// Advanced load test artifact lifecycle
	// DeleteActiveLoadTestBundleAndRollback deletes the bundle referenced by the current pointer
	// and attempts to roll back the pointer to the previous version (if any). It returns the
	// new pointer (nil if none) and the count of deleted bundle objects.
	DeleteActiveLoadTestBundleAndRollback(ctx context.Context, projectID string) (*models.LoadTestPointer, int, error)
	// PurgeLoadTestArtifacts removes ALL load test related objects (bundles, versions, pointer, metadata)
	// for the given project. It returns the count of deleted object versions and a flag indicating
	// whether the underlying storage bucket/container was also deleted as a consequence.
	PurgeLoadTestArtifacts(ctx context.Context, projectID string) (int, bool, error)

	// Load test (Locust) deployment metadata management
	SaveLoadTestDeploymentMetadata(metadata *models.LoadTestDeploymentOutputs) error
	GetLoadTestDeploymentMetadata() (*models.LoadTestDeploymentMetadata, error)
	DeleteLoadTestDeploymentMetadata() error

	// FillLoadTestOptions lets each provider inject provider-specific fields
	// (subscription ID, resource group, storage account name, VM size, etc.)
	// into a LoadTestDeploymentOptions before terraform.tfvars is rendered.
	// Common fields (ProjectName, Region, BucketName, Provider) are already
	// populated by the manager before this is called; the provider only needs
	// to set/override what is specific to it (e.g. Azure overwrites BucketName
	// with the storage account name and adds SubscriptionID / ResourceGroup).
	FillLoadTestOptions(opts *models.LoadTestDeploymentOptions)
}

// NamingStrategy defines how project names are converted to storage names
type NamingStrategy interface {
	// GenerateStorageName converts a project ID to a storage-specific name
	// Example: "my-project" -> "auto-mock-my-project-abc123"
	GenerateStorageName(projectID string) string

	// ExtractProjectID extracts the project ID from a storage name
	// Example: "auto-mock-my-project-abc123" -> "my-project"
	ExtractProjectID(storageName string) string

	// ValidateProjectID validates a project ID for naming constraints
	ValidateProjectID(projectID string) error

	// GenerateSuffix generates a random suffix for new projects
	GenerateSuffix() (string, error)

	// GetPrefix returns the naming prefix (e.g., "auto-mock")
	GetPrefix() string

	// Load test (locust) helpers for segregated storage paths
	LoadTestProjectID(projectID string) string
	LoadTestCurrentKey(projectID string) string
	LoadTestVersionKey(projectID, version string) string
	LoadTestBundlesPrefix(projectID string) string
	LoadTestBundleDir(projectID, bundleID string) string
	LoadTestBundleFileKey(projectID, bundleID, fileName string) string
	LoadTestMetadataKey(projectID string) string
}
