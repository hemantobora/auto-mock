package loadtest

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Result captures validation findings for a generated bundle.
type Result struct {
	HostDefined       bool
	PlaceholderErrors []string
	Tasks             int
	Endpoints         int
}

var (
	hostPattern        = regexp.MustCompile(`(?i)host\s*=\s*['"].+?['"]`)
	taskClassPattern   = regexp.MustCompile(`class\s+\w+\s*\(.*HttpUser.*\):`)
	endpointPattern    = regexp.MustCompile(`self\.client\.(get|post|put|delete|patch|options|head)\(\s*['"][^'"]+['"]`)
	placeholderPattern = regexp.MustCompile(`{{[^{}]+}}`)
)

// ValidateBundle inspects key files (locustfile.py, user_data.yaml) returning signals.
// Non-fatal errors are accumulated in PlaceholderErrors.
func ValidateBundle(dir string) (*Result, error) {
	res := &Result{}
	// locustfile
	lf := filepath.Join(dir, "locustfile.py")
	if data, err := os.ReadFile(lf); err == nil {
		content := string(data)
		res.HostDefined = hostPattern.FindStringIndex(content) != nil
		res.Tasks = len(taskClassPattern.FindAllString(content, -1))
		res.Endpoints = len(endpointPattern.FindAllString(content, -1))
	}
	// user_data.yaml placeholder scan
	ud := filepath.Join(dir, "user_data.yaml")
	if f, err := os.Open(ud); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			matches := placeholderPattern.FindAllString(line, -1)
			for _, m := range matches {
				// treat unresolved if still wrapped in {{ }} and not an allowed template directive
				if strings.Contains(m, "TODO") || strings.Contains(m, "REPLACE") {
					res.PlaceholderErrors = append(res.PlaceholderErrors, m)
				}
			}
		}
	}
	return res, nil
}
