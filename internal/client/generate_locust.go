package client

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/collections"
	"github.com/hemantobora/auto-mock/internal/models"
)

/* =========================
   Embedded templates
   ========================= */

//go:embed templates/locustfile.py
var locustfilePy []byte

//go:embed templates/requirements.txt
var requirementsTxt []byte

//go:embed templates/user_data.yaml
var userDataYaml []byte

//go:embed templates/LOADTEST_README.md
var loadtestReadme []byte

// macOS/Linux
//
//go:embed templates/run_locust_headless.sh
var runHeadlessSh []byte

//go:embed templates/run_locust_ui.sh
var runUISh []byte

// Windows
//
//go:embed templates/run_locust_headless.ps1
var runHeadlessPs1 []byte

//go:embed templates/run_locust_ui.ps1
var runUIPs1 []byte

// Optional distributed helpers
//
//go:embed templates/run_locust_master.sh
var runMasterSh []byte

//go:embed templates/run_locust_worker.sh
var runWorkerSh []byte

//go:embed templates/run_locust_master.ps1
var runMasterPs1 []byte

//go:embed templates/run_locust_worker.ps1
var runWorkerPs1 []byte

/* =========================
   Public entry
   ========================= */

type Options struct {
	// If empty, we prompt.
	CollectionPath string
	CollectionType string // Postman | Insomnia | Bruno | Other (informational only)
	OutDir         string // default ./loadtest
	// If nil, we prompt. If non-nil, use the value (true=headless, false=UI).
	Headless *bool
	// If nil, we prompt. If non-nil, use the value.
	GenerateDistributedHelpers *bool
}

func GenerateLoadtestBundle(opts Options) error {
	// Ask for missing basics
	if opts.CollectionPath == "" {
		if err := survey.AskOne(&survey.Input{
			Message: "Path to collection file:",
		}, &opts.CollectionPath, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}
	if opts.CollectionType == "" {
		if err := survey.AskOne(&survey.Select{
			Message: "Collection type:",
			Options: []string{"Postman", "Insomnia", "Bruno"},
			Default: "Postman",
		}, &opts.CollectionType); err != nil {
			return err
		}
	}
	if opts.OutDir == "" {
		opts.OutDir = "../loadtest"
	}

	processor, err := collections.NewCollectionProcessor("locust_loadtest", strings.ToLower(opts.CollectionType))
	if err != nil {
		return fmt.Errorf("create collection processor: %w", err)
	}

	// Parse the collection with your existing code (no logic change required)
	apiReqs, err := processor.ParseCollectionFile(opts.CollectionPath)
	if err != nil {
		return fmt.Errorf("parse collection: %w", err)
	}
	if len(apiReqs) == 0 {
		return errors.New("no API requests found in the collection")
	}

	if err := extractAndResolveVariables(apiReqs, processor); err != nil {
		return fmt.Errorf("extract and resolve variables: %w", err)
	}

	keepFullHost, err := promptRetainHostSelection(apiReqs)
	if err != nil {
		return err
	}
	eps := buildEndpointsFromAPIRequestsWithHost(apiReqs, keepFullHost)

	// Let the user pick the authentication request (or none)
	authIndex, authMode, tokenPath, headerName, headerPrefix, err := promptAuthSelection(apiReqs)
	if err != nil {
		return err
	}

	// Compose auth block
	auth := map[string]any{"mode": "none"}
	if authIndex >= 0 {
		r := apiReqs[authIndex]
		auth = map[string]any{
			"mode":            authMode, // shared | per_user
			"method":          strings.ToUpper(r.Method),
			"path":            toPath(r.URL),
			"headers":         r.Headers,
			"body":            r.Body,
			"token_json_path": tokenPath,
			"header_name":     headerName,
			"header_prefix":   headerPrefix,
		}
		// Remove auth endpoint from load targets
		eps = filterOut(eps, r.Method, toPath(r.URL))
	}

	// Final spec JSON
	spec := map[string]any{
		"auth": auth,
		"config": map[string]any{
			"default_headers":  map[string]string{"Content-Type": "application/json"},
			"min_wait_seconds": 0.2,
			"max_wait_seconds": 1.0,
		},
		"endpoints": eps,
	}

	// Write files
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(opts.OutDir, "locust_endpoints.json"), spec); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, "locustfile.py"), locustfilePy, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, "requirements.txt"), requirementsTxt, 0o644); err != nil {
		return err
	}

	// Add a sample user_data.yaml (only if absent)
	userDataPath := filepath.Join(opts.OutDir, "user_data.yaml")
	if _, statErr := os.Stat(userDataPath); os.IsNotExist(statErr) {
		if err := os.WriteFile(userDataPath, userDataYaml, 0o644); err != nil {
			return err
		}
	}

	// Add a README for the load test bundle (only if absent)
	readmePath := filepath.Join(opts.OutDir, "LOADTEST_README.md")
	if _, statErr := os.Stat(readmePath); os.IsNotExist(statErr) {
		if err := os.WriteFile(readmePath, loadtestReadme, 0o644); err != nil {
			return err
		}
	}

	// OS-aware runner scripts — ONLY write what makes sense for the host OS
	switch runtime.GOOS {
	case "windows":
		if *opts.Headless {
			if err := os.WriteFile(filepath.Join(opts.OutDir, "run_locust_headless.ps1"), runHeadlessPs1, 0o644); err != nil {
				return err
			}
		} else {
			if err := os.WriteFile(filepath.Join(opts.OutDir, "run_locust_ui.ps1"), runUIPs1, 0o644); err != nil {
				return err
			}
		}
		if *opts.GenerateDistributedHelpers {
			if err := os.WriteFile(filepath.Join(opts.OutDir, "run_locust_master.ps1"), runMasterPs1, 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(opts.OutDir, "run_locust_worker.ps1"), runWorkerPs1, 0o644); err != nil {
				return err
			}
		}
	default: // darwin, linux, etc.
		if *opts.Headless {
			if err := os.WriteFile(filepath.Join(opts.OutDir, "run_locust_headless.sh"), runHeadlessSh, 0o755); err != nil {
				return err
			}
		} else {
			if err := os.WriteFile(filepath.Join(opts.OutDir, "run_locust_ui.sh"), runUISh, 0o755); err != nil {
				return err
			}
		}
		if *opts.GenerateDistributedHelpers {
			if err := os.WriteFile(filepath.Join(opts.OutDir, "run_locust_master.sh"), runMasterSh, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(opts.OutDir, "run_locust_worker.sh"), runWorkerSh, 0o755); err != nil {
				return err
			}
		}
	}

	// Friendly next-steps
	fmt.Printf("✅ Locust bundle written to %s\n", opts.OutDir)
	if *opts.Headless {
		switch runtime.GOOS {
		case "windows":
			fmt.Printf("Next:\n  cd %s\n  .\\run_locust_headless.ps1  # set host via --host or AM_HOST\n", opts.OutDir)
		default:
			fmt.Printf("Next:\n  cd %s\n  ./run_locust_headless.sh    # set host via --host or AM_HOST\n", opts.OutDir)
		}
	} else {
		switch runtime.GOOS {
		case "windows":
			fmt.Printf("Next:\n  cd %s\n  .\\run_locust_ui.ps1        # open web UI and set host there\n", opts.OutDir)
		default:
			fmt.Printf("Next:\n  cd %s\n  ./run_locust_ui.sh          # open web UI and set host there\n", opts.OutDir)
		}
	}
	if *opts.GenerateDistributedHelpers {
		fmt.Println("Distributed mode:")
		switch runtime.GOOS {
		case "windows":
			fmt.Println("  Master: .\\run_locust_master.ps1")
			fmt.Println("  Worker: .\\run_locust_worker.ps1 -MASTER_HOST <master-ip>")
		default:
			fmt.Println("  Master: ./run_locust_master.sh")
			fmt.Println("  Worker: ./run_locust_worker.sh MASTER_HOST=<master-ip>")
		}
	}

	fmt.Println()
	fmt.Println("Data parameterization:")
	fmt.Println("  - Edit 'user_data.yaml' in the bundle to add fields like account_number, username, etc.")
	fmt.Println("  - Use placeholders ${data.<field>} in locust_endpoints.json headers/params/body.")
	fmt.Println("  - Control selection with config.data_assignment: round_robin | shared | random.")
	fmt.Println("    • Set 'shared' to use the same row for all users (or keep a single row).")
	printLocustConfigHelp()
	return nil
}

func replaceVariables(text string, variables map[string]string) string {
	for k, v := range variables {
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
		text = strings.ReplaceAll(text, "${"+k+"}", v)
	}
	return text
}

// resolveVariables performs the 5-step variable resolution process
func resolveVariables(neededVars []string, variables map[string]string) error {
	for _, varName := range neededVars {
		// Check if already resolved
		if _, exists := variables[varName]; exists {
			fmt.Printf("✅ %s (from previous setting)\n", varName)
			continue
		}

		// Step 2: Check environment
		if envVal := os.Getenv(varName); envVal != "" {
			variables[varName] = envVal
			fmt.Printf("✅ %s (from environment)\n", varName)
			continue
		}

		// Step 5: Ask user and confirm
		fmt.Printf("\n⚠️  Variable '%s' not found in environment\n", varName)
		var value string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Enter value for '%s':", varName),
			Help:    "This variable is needed to build the API. Enter the value or press Ctrl+C to cancel.",
		}, &value); err != nil {
			return &models.VariableResolutionError{
				VariableName: varName,
				Source:       "user-input",
				Cause:        err,
			}
		}

		if value == "" {
			return &models.VariableResolutionError{
				VariableName: varName,
				Source:       "user-input",
				Cause:        fmt.Errorf("no value provided for required variable '%s'", varName),
			}
		}

		// Confirm the value (without printing it)
		var confirm bool
		if err := survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("Use provided value for '%s'?", varName),
			Default: true,
		}, &confirm); err != nil {
			return &models.VariableResolutionError{
				VariableName: varName,
				Source:       "user-input",
				Cause:        err,
			}
		}

		if !confirm {
			// Ask again
			if err := survey.AskOne(&survey.Input{
				Message: fmt.Sprintf("Re-enter value for '%s':", varName),
			}, &value); err != nil {
				return &models.VariableResolutionError{
					VariableName: varName,
					Source:       "user-input",
					Cause:        err,
				}
			}
		}

		variables[varName] = value
		fmt.Printf("✅ %s (user input)\n", varName)
	}

	return nil
}

/* =========================
   Prompt & mapping helpers
   ========================= */

func promptAuthSelection(reqs []collections.APIRequest) (index int, mode, tokenPath, headerName, headerPrefix string, err error) {
	opts := []string{"None"}
	for _, r := range reqs {
		opts = append(opts, fmt.Sprintf("%s %s", strings.ToUpper(r.Method), toPath(r.URL)))
	}
	choice := ""
	if err = survey.AskOne(&survey.Select{
		Message:  "Select the authentication request (or None):",
		Options:  opts,
		Default:  "None",
		PageSize: 12,
	}, &choice); err != nil {
		return
	}
	if choice == "None" {
		return -1, "none", "", "", "", nil
	}

	// Resolve index in reqs
	idx := -1
	for i := range reqs {
		if choice == fmt.Sprintf("%s %s", strings.ToUpper(reqs[i].Method), toPath(reqs[i].URL)) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return -1, "none", "", "", "", errors.New("failed to resolve selected auth request")
	}

	// Auth scope
	scope := ""
	if err = survey.AskOne(&survey.Select{
		Message: "Auth scope:",
		Options: []string{"shared (once for all users)", "per_user (once per virtual user)"},
		Default: "shared (once for all users)",
	}, &scope); err != nil {
		return
	}
	if strings.HasPrefix(scope, "shared") {
		mode = "shared"
	} else {
		mode = "per_user"
	}

	// Token extraction & header injection
	tokenPath = "access_token"
	_ = survey.AskOne(&survey.Input{Message: "Token JSON path in login response (e.g., access_token or data.token):", Default: "access_token"}, &tokenPath)
	headerName = "Authorization"
	_ = survey.AskOne(&survey.Input{Message: "Header name to carry the token:", Default: "Authorization"}, &headerName)
	headerPrefix = "Bearer "
	_ = survey.AskOne(&survey.Input{Message: "Header prefix (empty for none, e.g. ' '):", Default: "Bearer "}, &headerPrefix)

	return idx, mode, tokenPath, headerName, headerPrefix, nil
}

type Endpoint struct {
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Weight  int               `json:"weight,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
	Body    any               `json:"body,omitempty"`
	Tags    []string          `json:"tags,omitempty"`
}

func extractAndResolveVariables(reqs []collections.APIRequest, cp *collections.CollectionProcessor) error {
	variables := make(map[string]string)

	for i := range reqs {
		r := &reqs[i] // <— mutate the actual element
		needed := cp.ExtractVariablesFromAPI(r, false)
		if len(needed) == 0 {
			continue
		}

		if err := resolveVariables(needed, variables); err != nil {
			return fmt.Errorf("resolve variables: %w", err)
		}

		// Persist replacements back to the element
		r.URL = replaceVariables(r.URL, variables)

		if r.Body != "" {
			r.Body = replaceVariables(r.Body, variables)
		}
		for k, v := range r.Headers {
			if strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "authorization") {
				continue // Skip Authorization header
			}
			r.Headers[k] = replaceVariables(v, variables)
		}
	}
	return nil
}

// promptRetainHostSelection lets the user choose which requests should keep their full URL
// (scheme + host). Returns a set of string keys ("METHOD|PATH") for selected requests.
func promptRetainHostSelection(reqs []collections.APIRequest) (map[string]bool, error) {
	opts := make([]string, 0, len(reqs))
	keyMap := make(map[string]string)

	for _, r := range reqs {
		method := strings.ToUpper(r.Method)
		display := fmt.Sprintf("%s %s", method, r.Name)
		key := fmt.Sprintf("%s|%s", method, r.URL)
		opts = append(opts, display)
		keyMap[display] = key
	}

	selected := []string{}
	prompt := &survey.MultiSelect{
		Message:  "Select API requests to RETAIN full host & scheme:",
		Options:  opts,
		PageSize: 15,
	}

	if err := survey.AskOne(prompt, &selected); err != nil {
		return nil, err
	}

	keep := make(map[string]bool)
	for _, sel := range selected {
		if key, ok := keyMap[sel]; ok {
			keep[key] = true
		}
	}
	return keep, nil
}

func buildEndpointsFromAPIRequestsWithHost(reqs []collections.APIRequest, keep map[string]bool) []Endpoint {
	out := make([]Endpoint, 0, len(reqs))
	for _, r := range reqs {
		method := strings.ToUpper(r.Method)
		key := fmt.Sprintf("%s|%s", method, r.URL)
		rawURL := r.URL

		path := rawURL
		if !keep[key] {
			path = toPath(rawURL) // strip host only if not selected
		}

		name := r.Name
		if name == "" {
			name = fmt.Sprintf("%s %s", method, path)
		}

		// Filter out Authorization header; it will be injected by locust at runtime
		filteredHeaders := map[string]string{}
		for hk, hv := range r.Headers {
			if strings.EqualFold(hk, "Authorization") {
				continue
			}
			filteredHeaders[hk] = hv
		}

		out = append(out, Endpoint{
			Name:    name,
			Method:  method,
			Path:    path,
			Headers: filteredHeaders,
			Body:    r.Body,
			Weight:  1,
		})
	}
	return out
}

func filterOut(eps []Endpoint, method, path string) []Endpoint {
	keep := eps[:0]
	m := strings.ToUpper(method)
	for _, e := range eps {
		if !(strings.EqualFold(e.Path, path) && strings.EqualFold(e.Method, m)) {
			keep = append(keep, e)
		}
	}
	return keep
}

func toPath(raw string) string {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		rest := raw[strings.Index(raw, "://")+3:]
		if i := strings.Index(rest, "/"); i >= 0 {
			return "/" + strings.TrimLeft(rest[i+1:], "/")
		}
		return "/"
	}
	if !strings.HasPrefix(raw, "/") {
		return "/" + raw
	}
	return raw
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func printLocustConfigHelp() {
	fmt.Println()
	fmt.Println("───────────────────────────────────────────────────────────────────────────────")
	fmt.Println("⚙️  Locust Configuration Options (editable in locust_endpoints.json)")
	fmt.Println("───────────────────────────────────────────────────────────────────────────────")
	fmt.Println("These control runtime behavior of your generated Locust test:")
	fmt.Println()
	fmt.Printf("%-28s %-12s %s\n", "Key", "Default", "Description")
	fmt.Printf("%-28s %-12s %s\n", "────────────────────────────", "────────────", "────────────────────────────────────────────")
	fmt.Printf("%-28s %-12s %s\n", "wait_strategy", "\"between\"", "Wait pattern: between | constant | random_exp")
	fmt.Printf("%-28s %-12s %s\n", "min_wait_seconds", "0.2", "Lower bound of user think time (seconds)")
	fmt.Printf("%-28s %-12s %s\n", "max_wait_seconds", "1.0", "Upper bound of user think time (seconds)")
	fmt.Printf("%-28s %-12s %s\n", "constant_wait_seconds", "1.0", "Exact wait if strategy = constant")
	fmt.Printf("%-28s %-12s %s\n", "request_timeout_seconds", "30", "Per-request timeout (seconds)")
	fmt.Printf("%-28s %-12s %s\n", "verify_tls", "true", "Set false to skip SSL verification")
	fmt.Printf("%-28s %-12s %s\n", "default_headers", "{}", "Merged into all request headers")
	fmt.Printf("%-28s %-12s %s\n", "default_params", "{}", "Merged into all request query params")
	fmt.Printf("%-28s %-12s %s\n", "data_assignment", "\"round_robin\"", "User data selection: shared | round_robin | random")
	fmt.Printf("%-28s %-12s %s\n", "data_shared_index", "0", "Row index used when data_assignment=shared")
	fmt.Println()
	fmt.Println("Edit these under the \"config\" block in locust_endpoints.json to customize behavior.")
	fmt.Println("Example:")
	fmt.Println(`  "config": {`)
	fmt.Println(`    "wait_strategy": "random_exp",`)
	fmt.Println(`    "min_wait_seconds": 0.1,`)
	fmt.Println(`    "max_wait_seconds": 2.0,`)
	fmt.Println(`    "request_timeout_seconds": 20,`)
	fmt.Println(`    "verify_tls": false,`)
	fmt.Println(`    "data_assignment": "round_robin",`)
	fmt.Println(`    "data_shared_index": 0,`)
	fmt.Println(`    "default_headers": { "Content-Type": "application/json" }`)
	fmt.Println(`  }`)
	fmt.Println("───────────────────────────────────────────────────────────────────────────────")
	fmt.Println("")
}
