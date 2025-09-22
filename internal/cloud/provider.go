// Package cloud defines the provider interface for cloud infrastructure management.
// This allows auto-mock to support multiple cloud providers (AWS, GCP, Azure, etc.)
package cloud

// Provider defines the interface that cloud providers must implement
type Provider interface {
	// InitProject sets up the initial cloud infrastructure for a project
	InitProject() error

	// DeleteProject removes all cloud infrastructure for a project
	DeleteProject() error
}
