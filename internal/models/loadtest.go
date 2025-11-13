package models

import "time"

// LoadTestPointer represents the active pointer for a project's load test bundle
// Stored at: configs/<project>-loadtest/current.json
// References an immutable version file under versions/
type LoadTestPointer struct {
	ProjectID     string            `json:"project_id"`
	ArtifactType  string            `json:"artifact_type"` // always: "loadtest_bundle"
	ActiveVersion string            `json:"active_version"`
	BundleID      string            `json:"bundle_id"`
	UpdatedAt     time.Time         `json:"updated_at"`
	Files         map[string]string `json:"files"` // logical name -> S3 key
	Summary       *LoadTestSummary  `json:"summary,omitempty"`
}

// LoadTestSummary provides lightweight info for quick UIs and status
type LoadTestSummary struct {
	Tasks     int  `json:"tasks,omitempty"`
	Endpoints int  `json:"endpoints,omitempty"`
	HasHost   bool `json:"has_host,omitempty"`
}

// LoadTestVersion is an immutable snapshot of a bundle version
// Stored at: configs/<project>-loadtest/versions/v<timestamp>.json
// Includes content hashes and validation result
type LoadTestVersion struct {
	ProjectID  string                    `json:"project_id"`
	Version    string                    `json:"version"`
	BundleID   string                    `json:"bundle_id"`
	CreatedAt  time.Time                 `json:"created_at"`
	Hashes     map[string]string         `json:"hashes"`     // filename -> sha256:hex
	Validation *LoadTestValidationResult `json:"validation"` // validation outcome
	Metrics    map[string]int            `json:"metrics,omitempty"`
}

// LoadTestValidationResult captures validation signals for a bundle upload
type LoadTestValidationResult struct {
	LocustfilePresent   bool     `json:"locustfile_present"`
	RequirementsPresent bool     `json:"requirements_present"`
	UserDataPresent     bool     `json:"user_data_present"`
	ManifestPresent     bool     `json:"manifest_present"`
	HostDefined         bool     `json:"host_defined"`
	PlaceholderErrors   []string `json:"placeholder_errors"`
}

// LoadTestManifest describes the files contained in a bundle directory
// Stored alongside files at: configs/<project>-loadtest/bundles/<bundle_id>/manifest.json
type LoadTestManifest struct {
	BundleID    string            `json:"bundle_id"`
	ProjectID   string            `json:"project_id"`
	GeneratedAt time.Time         `json:"generated_at"`
	Files       []LoadTestFileRef `json:"files"`
	Entrypoints []string          `json:"entrypoints"`
	Warnings    []string          `json:"warnings,omitempty"`
	Notes       string            `json:"notes,omitempty"`
}

// LoadTestFileRef is a description of an individual file in the bundle
type LoadTestFileRef struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// LoadTestMetadataIndex is a light index for quick listing/status
// Stored at: metadata/<project>-loadtest.json
// Optional; can be recomputed by listing versions
type LoadTestMetadataIndex struct {
	ProjectID     string    `json:"project_id"`
	LatestVersion string    `json:"latest_version"`
	BundleCount   int       `json:"bundle_count"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NewDefaultLoadTestPointer creates a minimal pointer with defaults
func NewDefaultLoadTestPointer(projectID, version, bundleID string, files map[string]string, summary *LoadTestSummary) *LoadTestPointer {
	return &LoadTestPointer{
		ProjectID:     projectID,
		ArtifactType:  "loadtest_bundle",
		ActiveVersion: version,
		BundleID:      bundleID,
		UpdatedAt:     time.Now().UTC(),
		Files:         files,
		Summary:       summary,
	}
}
