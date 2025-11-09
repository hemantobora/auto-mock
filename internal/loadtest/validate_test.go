package loadtest

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestValidateBundleHostAndTasks(t *testing.T) {
	dir := t.TempDir()
	locustfile := `host = "https://api.example.com"
class UserA(HttpUser):
    def on_start(self):
        self.client.get("/ping")
`
	writeFile(t, dir, "locustfile.py", locustfile)
	writeFile(t, dir, "user_data.yaml", "name: test\nvalue: {{REPLACE_ME}}\n")

	res, err := ValidateBundle(dir)
	if err != nil {
		t.Fatalf("validate error: %v", err)
	}
	if !res.HostDefined {
		t.Errorf("expected host defined")
	}
	if res.Tasks == 0 {
		t.Errorf("expected tasks > 0; got %d", res.Tasks)
	}
	if res.Endpoints == 0 {
		t.Errorf("expected endpoints > 0; got %d", res.Endpoints)
	}
	if len(res.PlaceholderErrors) == 0 {
		t.Errorf("expected placeholder errors detected")
	}
}

func TestValidateBundleNoHost(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "locustfile.py", "class UserB(HttpUser):\n    def task(self):\n        self.client.get('/x')\n")
	res, err := ValidateBundle(dir)
	if err != nil {
		t.Fatalf("validate error: %v", err)
	}
	if res.HostDefined {
		t.Errorf("did not expect host defined")
	}
}
