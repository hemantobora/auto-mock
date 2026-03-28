package terraform

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// NOTE: embed patterns are relative to this file's directory (internal/terraform).

// ── Mock (MockServer) templates ──────────────────────────────────────────────

//go:embed infra/mock/aws/*.tf
var awsMockTemplates embed.FS

//go:embed infra/mock/azure/*.tf
var azureMockTemplates embed.FS

// ── Loadtest (Locust) templates ──────────────────────────────────────────────

//go:embed infra/loadtest/aws/*.tf
var awsLoadtestTemplates embed.FS

//go:embed infra/loadtest/azure/*.tf
var azureLoadtestTemplates embed.FS

// getMockTemplates returns the embedded Terraform templates for the given
// cloud provider's mock (MockServer) stack.
func getMockTemplates(providerType string) (embed.FS, error) {
	switch providerType {
	case "aws":
		return awsMockTemplates, nil
	case "azure":
		return azureMockTemplates, nil
	default:
		return embed.FS{}, fmt.Errorf("no mock terraform templates for provider %q", providerType)
	}
}

// getLoadtestTemplates returns the embedded Terraform templates for the given
// cloud provider's loadtest (Locust) stack.
func getLoadtestTemplates(providerType string) (embed.FS, error) {
	switch providerType {
	case "aws":
		return awsLoadtestTemplates, nil
	case "azure":
		return azureLoadtestTemplates, nil
	default:
		return embed.FS{}, fmt.Errorf("no loadtest terraform templates for provider %q", providerType)
	}
}

// writeEmbeddedTemplates copies all embedded *.tf files from the given FS root
// into the target directory, preserving base filenames.
func writeEmbeddedTemplates(fsys embed.FS, targetDir string) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".tf" {
			return nil
		}
		content, err := fsys.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		base := filepath.Base(path)
		dest := filepath.Join(targetDir, base)
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		return nil
	})
}
