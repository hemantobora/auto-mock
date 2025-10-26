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
	DeleteConfig(ctx context.Context, projectID string) error

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

	// Cleanup
	DeleteProject() error

	// Deployment metadata management
	SaveDeploymentMetadata(ctx context.Context, metadata *models.DeploymentMetadata) error
	GetDeploymentMetadata(ctx context.Context) (*models.DeploymentMetadata, error)
	DeleteDeploymentMetadata(ctx context.Context) error
	UpdateDeploymentStatus(ctx context.Context, status string) error

	CreateDeploymentConfiguration() *models.DeploymentOptions
	DisplayCostEstimate(options *models.DeploymentOptions)
	CreateDefaultDeploymentConfiguration() *models.DeploymentOptions
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
}
