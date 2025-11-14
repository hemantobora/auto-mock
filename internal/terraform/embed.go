package terraform

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// NOTE: embed patterns are relative to this file's directory (internal/terraform).

// Embedded Terraform templates for the mock stack.
//
//go:embed infra/mock/*.tf
var mockTemplates embed.FS

// Embedded Terraform templates for the loadtest stack.
//
//go:embed infra/loadtest/*.tf
var loadtestTemplates embed.FS

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
