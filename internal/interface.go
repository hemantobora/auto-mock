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

	// Load test (Locust) deployment metadata management
	SaveLoadTestDeploymentMetadata(metadata *models.LoadTestDeploymentOutputs) error
	GetLoadTestDeploymentMetadata() (*models.LoadTestDeploymentMetadata, error)
	DeleteLoadTestDeploymentMetadata() error
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
