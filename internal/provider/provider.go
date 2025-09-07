package provider

// Provider is a cloud-agnostic interface for project lifecycle management
// All cloud providers (AWS, Azure, GCP) should implement this interface

type Provider interface {
	InitProject() error
	DeleteProject() error
}
