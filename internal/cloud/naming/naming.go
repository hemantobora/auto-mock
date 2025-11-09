package naming

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// DefaultNaming implements the NamingStrategy interface
type DefaultNaming struct {
	prefix string // "auto-mock"
}

// NewDefaultNaming creates a new default naming strategy
func NewDefaultNaming() *DefaultNaming {
	return &DefaultNaming{
		prefix: "auto-mock-",
	}
}

func (n *DefaultNaming) GetPrefix() string {
	return n.prefix
}

// GenerateStorageName converts a project ID to a storage-specific name
// Format: auto-mock-{projectID}-{suffix}
func (n *DefaultNaming) GenerateStorageName(projectID string) string {
	suffix, _ := n.GenerateSuffix()
	return fmt.Sprintf("%s%s-%s", n.prefix, projectID, suffix)
}

// ExtractProjectID extracts the project ID from a storage name
func (n *DefaultNaming) ExtractProjectID(storageName string) string {
	// Remove prefix (handle both old and new formats)
	name := strings.TrimPrefix(storageName, n.prefix)

	// If no prefix was removed, return as-is
	if name == storageName {
		return storageName
	}

	// Remove suffix (last segment after last hyphen, if it looks like a suffix)
	parts := strings.Split(name, "-")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		// Check if last part looks like a generated suffix (6+ alphanumeric chars)
		if len(lastPart) >= 8 && isAlphanumeric(lastPart) {
			return strings.Join(parts[:len(parts)-1], "-")
		}
	}

	return name
}

// ValidateProjectID validates a project ID according to cloud naming constraints
func (n *DefaultNaming) ValidateProjectID(projectID string) error {
	if projectID == "" {
		return fmt.Errorf("project ID cannot be empty")
	}

	// Length check (leaving room for prefix and suffix)
	if len(projectID) > 40 {
		return fmt.Errorf("project ID too long (max 40 characters)")
	}

	// Must start with letter or number
	if !regexp.MustCompile(`^[a-z0-9]`).MatchString(projectID) {
		return fmt.Errorf("project ID must start with a letter or number")
	}

	// Can only contain lowercase letters, numbers, and hyphens
	if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(strings.ToLower(projectID)) {
		return fmt.Errorf("project ID can only contain lowercase letters, numbers, and hyphens")
	}

	// Cannot end with hyphen
	if strings.HasSuffix(projectID, "-") {
		return fmt.Errorf("project ID cannot end with a hyphen")
	}

	// Cannot have consecutive hyphens
	if strings.Contains(projectID, "--") {
		return fmt.Errorf("project ID cannot contain consecutive hyphens")
	}

	return nil
}

// GenerateSuffix generates a random alphanumeric suffix
func (n *DefaultNaming) GenerateSuffix() (string, error) {
	return generateBase36Suffix(8)
}

// Helper functions

// isAlphanumeric checks if a string contains only letters and numbers
func isAlphanumeric(s string) bool {
	return regexp.MustCompile(`^[a-z0-9]+$`).MatchString(s)
}

// generateBase36Suffix generates a random base36 string of specified length
func generateBase36Suffix(length int) (string, error) {
	const charset = "0123456789abcdefghijklmnopqrstuvwxyz"

	// Seed random with current time
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = charset[rng.Intn(len(charset))]
	}

	return string(result), nil
}

// NormalizeProjectID normalizes a project ID to be storage-friendly
func NormalizeProjectID(projectID string) string {
	// Convert to lowercase
	normalized := strings.ToLower(projectID)

	// Replace invalid characters with hyphens
	normalized = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(normalized, "-")

	// Remove consecutive hyphens
	for strings.Contains(normalized, "--") {
		normalized = strings.ReplaceAll(normalized, "--", "-")
	}

	// Trim hyphens from start and end
	normalized = strings.Trim(normalized, "-")

	return normalized
}

// ===== Load Test (Locust) Naming Helpers =====
// These helpers generate stable S3 object keys for load test bundles, versions and metadata.
// Layout (agreed):
// configs/<project>-loadtest/current.json                -> active pointer to latest uploaded bundle version
// configs/<project>-loadtest/versions/v<ts>.json         -> immutable pointer snapshot for a version
// configs/<project>-loadtest/bundles/<bundleID>/...      -> bundle directory (locustfile.py, requirements.txt, user_data.yaml, manifest.json, etc.)
// metadata/<project>-loadtest.json                      -> lightweight summary / index for load test (parallel to mock config metadata)
//
// projectID passed in may be either the raw project id or a storage name; we always normalize/extract first.

// LoadTestProjectID returns the canonical load test project id with -loadtest suffix
func (n *DefaultNaming) LoadTestProjectID(projectID string) string {
	base := n.ExtractProjectID(projectID)
	if strings.HasSuffix(base, "-loadtest") {
		return base
	}
	return fmt.Sprintf("%s-loadtest", base)
}

// LoadTestCurrentKey returns the key for the active load test pointer file (current.json)
func (n *DefaultNaming) LoadTestCurrentKey(projectID string) string {
	lt := n.LoadTestProjectID(projectID)
	return fmt.Sprintf("configs/%s/current.json", lt)
}

// LoadTestVersionKey returns the key for a specific version snapshot file
func (n *DefaultNaming) LoadTestVersionKey(projectID, version string) string {
	lt := n.LoadTestProjectID(projectID)
	return fmt.Sprintf("configs/%s/versions/%s.json", lt, version)
}

// LoadTestBundlesPrefix returns the prefix under which bundle directories are stored
func (n *DefaultNaming) LoadTestBundlesPrefix(projectID string) string {
	lt := n.LoadTestProjectID(projectID)
	return fmt.Sprintf("configs/%s/bundles/", lt)
}

// LoadTestBundleDir returns the directory prefix for a specific bundle ID
func (n *DefaultNaming) LoadTestBundleDir(projectID, bundleID string) string {
	lt := n.LoadTestProjectID(projectID)
	return fmt.Sprintf("configs/%s/bundles/%s/", lt, bundleID)
}

// LoadTestBundleFileKey returns the full object key for a file inside a bundle directory
func (n *DefaultNaming) LoadTestBundleFileKey(projectID, bundleID, fileName string) string {
	lt := n.LoadTestProjectID(projectID)
	return fmt.Sprintf("configs/%s/bundles/%s/%s", lt, bundleID, fileName)
}

// LoadTestMetadataKey returns the key for the load test metadata index file
func (n *DefaultNaming) LoadTestMetadataKey(projectID string) string {
	lt := n.LoadTestProjectID(projectID)
	return fmt.Sprintf("metadata/%s.json", lt)
}
