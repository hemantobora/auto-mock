// Package models provides shared data structures
package models

// ProviderInfo contains information about an available provider
type ProviderInfo struct {
	Name      string `json:"name"`       // "AWS", "GCP", "Azure"
	Type      string `json:"type"`       // "aws", "gcp", "azure"
	Available bool   `json:"available"`  // Has valid credentials
	Region    string `json:"region"`     // Current region
	Account   string `json:"account"`    // Account ID/Project ID
}

// AccountInfo contains cloud account information
type AccountInfo struct {
	AccountID   string            `json:"account_id"`   // AWS Account, GCP Project, Azure Subscription
	UserID      string            `json:"user_id"`      // IAM User, Service Account, etc.
	Region      string            `json:"region"`       // Default region
	Permissions []string          `json:"permissions"`  // Available permissions
	Metadata    map[string]string `json:"metadata"`     // Provider-specific metadata
}

// Permission represents a cloud permission
type Permission struct {
	Resource string   `json:"resource"` // Resource type (s3:bucket, gcs:bucket, etc.)
	Actions  []string `json:"actions"`  // Allowed actions
}

// CloudCapability represents what a provider can do
type CloudCapability struct {
	Storage   bool `json:"storage"`    // Can store configurations
	Compute   bool `json:"compute"`    // Can deploy infrastructure
	DNS       bool `json:"dns"`        // Can manage DNS
	TLS       bool `json:"tls"`        // Can manage TLS certificates
	Monitoring bool `json:"monitoring"` // Has monitoring capabilities
}
