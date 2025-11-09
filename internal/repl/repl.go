package repl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/atotto/clipboard"
	core "github.com/hemantobora/auto-mock/internal"
	"github.com/hemantobora/auto-mock/internal/client"
	"github.com/hemantobora/auto-mock/internal/mcp"
	"github.com/hemantobora/auto-mock/internal/models"

	// aws specific purge (best-effort) only if underlying concrete type is AWS provider
	awsSDK "github.com/aws/aws-sdk-go-v2/aws"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	awsprov "github.com/hemantobora/auto-mock/internal/cloud/aws"
)

const defaultBodyMatchType = "ONLY_MATCHING_FIELDS"

// StartMockGenerationREPL is the main entry point for mock generation
// StartMockGenerationREPL starts the interactive generation REPL.
// If providerOverride is non-empty it will be used as the preselected MCP provider
// (e.g. "anthropic", "openai", "template") and the REPL will skip the
// provider selection prompt.
func StartMockGenerationREPL(projectName string, providerOverride string) (string, error) {
	fmt.Printf("🎯 MockServer Configuration Generator Initialized\n")
	fmt.Printf("📦 Project: %s\n", projectName)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Simple mock config generation for now
	// Step 1: Choose generation method
	var method string
	if err := survey.AskOne(&survey.Select{
		Message: "How do you want to generate your mock configuration?",
		Options: []string{
			"interactive - Build endpoints step-by-step (7-step builder)",
			"collection - Import from Postman/Bruno/Insomnia",
			"describe - Describe your API in natural language (AI-powered)",
			"upload - Upload expectation file directly (JSON)",
		},
		Default: "interactive - Build endpoints step-by-step (7-step builder)",
	}, &method); err != nil {
		return "", err
	}

	method = strings.Split(method, " ")[0]

	// Step 2: Generate mock configuration using MCP engine
	mockServerJSON, err := generateMockConfiguration(method, projectName, providerOverride)
	if err != nil {
		return "", fmt.Errorf("failed to generate configuration: %w", err)
	}

	// Only show menu if we have JSON to work with
	if mockServerJSON == "" {
		return "", fmt.Errorf("no configuration generated")
	}

	// Display the result
	fmt.Println("\n📋 Generated MockServer Configuration:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(mockServerJSON)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Handle the result (save, deploy, etc.)
	// return handleFinalResult(mockServerJSON, projectName)
	return mockServerJSON, nil
}

func ResolveProjectInteractively(existing []models.ProjectInfo) (models.ProjectInfo, error) {
	var options []string
	var nameToProject map[string]models.ProjectInfo = make(map[string]models.ProjectInfo)
	for _, info := range existing {
		options = append(options, info.ProjectID)
		nameToProject[info.ProjectID] = info
	}
	options = append(options, "📝 Create New Project")

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "Select project:",
		Options: options,
	}, &choice); err != nil {
		return models.ProjectInfo{}, err
	}

	if strings.Contains(choice, "Create New") {
		return models.ProjectInfo{}, nil
	}
	return nameToProject[choice], nil
}

func SelectProjectAction(projectName string, existingConfig *models.MockConfiguration) models.ActionType {
	var action string

	// Check if expectations already exist
	expectationsExist := existingConfig != nil && existingConfig.Expectations != nil && len(existingConfig.Expectations) > 0
	var options []string
	if expectationsExist {
		// When expectations exist: management operations + view/download
		options = []string{
			"view - View expectations or entire configuration file",
			"download - Download the entire expectations file",
			"edit - Edit a particular expectation (modify method, path, response, etc.)",
			"remove - Remove specific expectations while keeping others",
			"replace - Replace ALL existing expectations with new ones",
			"delete - Delete the entire project and tear down infrastructure (if running)",
			"add - Add new expectations to existing ones",
			"deploy - Deploy current expectations to cloud infrastructure",
			"exit - Cancel the operation and exit",
		}
	} else {
		// When no expectations exist: only generation (no management operations)
		options = []string{
			"generate - Create a set of expectations from Collection, Interactively or examples",
			"exit - Cancel the operation and exit",
		}
	}

	survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Project: %s - What would you like to do?", projectName),
		Options: options,
	}, &action)

	// Extract the action keyword (first word before " - ")
	return models.ActionType(strings.Split(action, " ")[0])
}

// generateMockConfiguration uses the MCP engine to generate configurations
// Returns: (mockServerJSON, error)
func generateMockConfiguration(method, projectName, providerOverride string) (string, error) {
	ctx := context.Background()
	switch method {
	case "describe":
		return generateFromDescription(ctx, projectName, providerOverride)
	case "interactive":
		return generateInteractiveWithMenu()
	case "collection":
		return generateFromCollectionWithMenu(projectName)
	case "upload":
		return configureUploadedExpectationWithMenu(projectName)
	default:
		return generateFromDescription(ctx, projectName, providerOverride)
	}
}

// generateFromDescription uses AI to generate expectations from natural language.
// - No file I/O, no sanitization (we show a short disclaimer).
// - Provider selection with API key prompt (env first).
// - REST / GraphQL prompt hint.
// - One optional regenerate pass.
// - Returns MockServer JSON string produced from []models.MockExpectation.
func generateFromDescription(ctx context.Context, projectName string, providerOverride string) (string, error) {
	fmt.Println("🤖 AI-Powered Generation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("⚠️  Disclaimer: Review inputs for any secrets/tokens before use.")

	// 1) Providers
	infos := mcp.ListProviders()
	if len(infos) == 0 {
		return "", fmt.Errorf("no AI providers registered")
	}

	// If the CLI passed an explicit provider, use it as the preselected provider.
	var provider string
	if providerOverride != "" {
		provider = providerOverride
		// Normalize casing if possible (use registered name)
		for _, pi := range infos {
			if strings.EqualFold(pi.Name, providerOverride) {
				provider = pi.Name
				break
			}
		}
		fmt.Printf("Using provider (from CLI): %s\n", provider)
	} else {
		opts := make([]string, 0, len(infos))
		for _, pi := range infos {
			label := pi.Name
			if !pi.Available {
				label += " (not configured)"
			}
			fmt.Printf("%s %s\n", ternary(pi.Available, "✅", "❌"), label)
			if pi.Available {
				opts = append(opts, pi.Name)
			}
		}
		if len(opts) == 0 {
			// let user choose anyway, we’ll ask for API key below
			for _, pi := range infos {
				opts = append(opts, pi.Name)
			}
		}
		fmt.Println()

		// 2) Pick provider interactively
		if len(opts) == 0 {
			return "", fmt.Errorf("no providers available to choose from")
		}
		if err := survey.AskOne(&survey.Select{
			Message: "Choose an AI provider:",
			Options: opts,
			Default: opts[0],
		}, &provider); err != nil {
			return "", err
		}
	}

	// 3) Ensure API key (env → prompt once → exit if still missing)
	if !ensureProviderAPIKey(provider) {
		return "", fmt.Errorf("missing API key for provider %q", provider)
	}

	// 4) API style
	var apiStyle string
	if err := survey.AskOne(&survey.Select{
		Message: "API style?",
		Options: []string{"REST", "GraphQL"},
		Default: "REST",
	}, &apiStyle); err != nil {
		return "", err
	}

	// 5) Example preview
	var description string
	var useExample bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "Would you like to view some example API descriptions?",
		Default: false,
	}, &useExample)

	if useExample {
		opts := make([]string, len(mcp.Examples))
		for i, ex := range mcp.Examples {
			opts[i] = ex.Name
		}

		var choice string
		if err := survey.AskOne(&survey.Select{
			Message: "Select an example to preview:",
			Options: opts,
		}, &choice); err != nil {
			return "", err
		}

		for _, ex := range mcp.Examples {
			if ex.Name == choice {
				fmt.Println("\n───────────────────────────────")
				fmt.Println(ex.Description)
				fmt.Println("───────────────────────────────")

				var copyIt bool
				_ = survey.AskOne(&survey.Confirm{
					Message: "Copy this example to clipboard so you can edit externally?",
					Default: true,
				}, &copyIt)

				if copyIt {
					if err := clipboard.WriteAll(ex.Description); err != nil {
						fmt.Printf("⚠️  Failed to copy to clipboard: %v\n", err)
					} else {
						fmt.Println("✅ Copied to clipboard! Paste it into your editor and modify as needed.")
					}
				}

				var useIt bool
				_ = survey.AskOne(&survey.Confirm{
					Message: "Use this example as your description (without editing)?",
					Default: false,
				}, &useIt)
				if useIt {
					description = ex.Description
				}
				break
			}
		}
	}

	if description == "" {
		// Now open the multiline editor, prefilled if we picked an example above
		if err := survey.AskOne(&survey.Multiline{
			Message: "Describe your API (endpoints/types, fields, status codes, etc.):",
			Help:    "Tip: list endpoints/operations, inputs/outputs, auth headers, error envelope, pagination. Include at least one error case.",
			Default: description, // <<— this pre-fills with example if selected
		}, &description); err != nil {
			return "", err
		}
	}

	if strings.TrimSpace(description) == "" {
		return "", fmt.Errorf("description cannot be empty")
	}

	// 6) Optional hints toggle (kept tiny)
	var addHints bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "Add minimal hints (JSON-only request bodies STRlCT/ONLY_MATCHING_FIELDS; Velocity rule for responses)?",
		Default: true,
	}, &addHints)

	// 7) First generation
	prompt := buildPrompt(description, apiStyle, projectName, addHints)
	jsonPreview, exp, err := callAndNormalize(ctx, provider, projectName, prompt)
	if err != nil {
		return "", err
	}
	fmt.Println("\n📦 Preview (first ~40 lines):")
	printFirstLines(jsonPreview, 40)

	// 8) Optional regenerate pass
	var again bool
	_ = survey.AskOne(&survey.Confirm{
		Message: "Regenerate with revised instructions?",
		Default: false,
	}, &again)

	if again {
		var delta string
		if err := survey.AskOne(&survey.Multiline{
			Message: "Add constraints or changes:",
		}, &delta); err != nil {
			return "", err
		}
		if strings.TrimSpace(delta) != "" {
			prompt = prompt + "\n\nRefinements:\n" + strings.TrimSpace(delta)
			jsonPreview, exp, err = callAndNormalize(ctx, provider, projectName, prompt)
			if err != nil {
				return "", err
			}
			fmt.Println("\n📦 Preview (first ~40 lines):")
			printFirstLines(jsonPreview, 40)
		}
	}

	// 9) Return final MockServer JSON (coexists with other generators)
	return models.ExpectationsToMockServerJSON(exp), nil
}

// --- helpers (kept minimal) ---

func buildPrompt(description, apiStyle, projectName string, addHints bool) string {
	var sb strings.Builder

	sb.WriteString("Return ONLY a valid JSON array of MockServer expectations. No prose, no markdown, no code fences.\n\n")

	sb.WriteString("Each element MUST conform to models.MockExpectation with these shapes:\n")
	sb.WriteString(`[
  {
    "description": "short endpoint summary",
    "priority": 10,
    "httpRequest": {
      "method": "GET|POST|PUT|DELETE|PATCH|OPTIONS|HEAD",
      "path": "/example/path",
      "headers": { "Header-Name": ["regex-or-literal"] },
      "queryStringParameters": { "param": ["regex-or-literal"] },
      "body": {
        "type": "JSON",
        "json": { "key": "value" },
        "matchType": "STRICT" // OR "ONLY_MATCHING_FIELDS" (pick one)
      }
    },
    "httpResponse": {
      "statusCode": 200,
      "headers": { "Content-Type": ["application/json"] },
      "body": { "type": "JSON", "json": "{\"key\":\"value\"}" },
      "delay": { "timeUnit": "MILLISECONDS", "value": 100 }
    },
    "times": { "unlimited": true }
  }
]
`)

	sb.WriteString("\nRules:\n")
	sb.WriteString("- The top-level output must be a JSON array ([]).\n")
	sb.WriteString("- Use only fields supported by MockServer and the given schema (no extra keys).\n")
	sb.WriteString("- No comments or natural-language explanations.\n")

	if apiStyle == "GraphQL" {
		sb.WriteString("\nAPI Style: GraphQL\n")
		sb.WriteString("- Use a single POST /graphql endpoint.\n")
		sb.WriteString("- httpRequest.body.json MUST contain { \"query\", \"operationName\", \"variables\" }.\n")
		sb.WriteString("- Include 1–2 queries and 1 mutation example.\n")
		sb.WriteString("- Requests must be JSON-only with {type:JSON, json:<object>, matchType:\"STRICT\" or \"ONLY_MATCHING_FIELDS\"}.\n")
		sb.WriteString("- Prefer including a non-empty operationName when the query declares one.\n")
	} else {
		sb.WriteString("\nAPI Style: REST\n")
		sb.WriteString("- Use realistic verbs and paths (e.g., /auth/login, /users/{id}).\n")
		sb.WriteString("- Include a mix of 2xx success, 4xx error, and an optional 5xx case.\n")
		sb.WriteString("- For authenticated routes, set headers like: { \"Authorization\": [\".*Bearer .*\"] }.\n")
		sb.WriteString("- Include a paginated listing example: /users?page&limit and a representative response body.\n")
		sb.WriteString("- Requests must be JSON-only with {type:JSON, json:<object>, matchType:\"STRICT\" or \"ONLY_MATCHING_FIELDS\"}.\n")
	}

	if addHints {
		sb.WriteString("\nBehavioral & Formatting Hints:\n")
		sb.WriteString("- Response body rule: if it contains $! (Velocity template), put the raw string directly in httpResponse.body.\n")
		sb.WriteString("  Otherwise wrap as {\"type\":\"JSON\",\"json\":\"<stringified JSON>\"}.\n")
		sb.WriteString("- Use ISO 8601 timestamps and deterministic IDs (e.g., u_1001, order_001).\n")
		sb.WriteString("- Prefer compact responses over verbose ones.\n")
		sb.WriteString("- Always include \"times\": { \"unlimited\": true } unless a finite repetition is intended.\n")
		sb.WriteString("- Include at least one error response with envelope: {\"error\":{\"code\":\"<CODE>\",\"message\":\"<DETAIL>\"}}.\n")
	}

	sb.WriteString("\nProject Context:\n")
	sb.WriteString("This is for project: " + projectName + "\n")

	sb.WriteString("\nUser Description:\n")
	sb.WriteString(strings.TrimSpace(description))
	sb.WriteString("\n\nOutput strictly as a raw JSON array. Nothing else.\n")

	return sb.String()
}

func callAndNormalize(ctx context.Context, provider, project, prompt string) (pretty string, exps []models.MockExpectation, err error) {
	// call MCP
	res, err := mcp.GenerateWithProvider(ctx, prompt, provider, project)
	if err != nil {
		return "", nil, err
	}
	raw := strings.TrimSpace(res.MockServerJSON)
	if raw == "" {
		return "", nil, fmt.Errorf("provider returned empty JSON")
	}

	// unmarshal to your model
	var tmp []models.MockExpectation
	if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
		return "", nil, fmt.Errorf("invalid JSON from provider: %w", err)
	}

	// normalize per your strict rules
	normalizeExpectations(&tmp)

	// pretty preview
	out := models.ExpectationsToMockServerJSON(tmp)
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(out), "", "  "); err == nil {
		return buf.String(), tmp, nil
	}
	return out, tmp, nil
}

func normalizeExpectations(exps *[]models.MockExpectation) {
	for i := range *exps {
		e := &(*exps)[i]
		// Request body: JSON only + STRICT + onlyMatchingFields=true
		if e.HttpRequest != nil && e.HttpRequest.Body != nil {
			e.HttpRequest.Body = coerceRequestJSONBody(e.HttpRequest.Body)
		}
		// Response body rule: Velocity vs JSON wrapper
		if e.HttpResponse != nil && e.HttpResponse.Body != nil {
			e.HttpResponse.Body = coerceResponseBody(e.HttpResponse.Body)
		}
	}
}
func coerceRequestJSONBody(body any) any {
	switch v := body.(type) {
	case string:
		trim := strings.TrimSpace(v)
		if (strings.HasPrefix(trim, "{") && strings.HasSuffix(trim, "}")) ||
			(strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]")) {
			var obj any
			if json.Unmarshal([]byte(trim), &obj) == nil {
				return map[string]any{
					"type":      "JSON",
					"json":      obj,
					"matchType": defaultBodyMatchType,
				}
			}
		}
		// fallback: keep string as value
		return map[string]any{
			"type":      "JSON",
			"json":      v,
			"matchType": defaultBodyMatchType,
		}
	case map[string]any, []any:
		return map[string]any{
			"type":      "JSON",
			"json":      v,
			"matchType": defaultBodyMatchType,
		}
	default:
		b, _ := json.Marshal(v)
		return map[string]any{
			"type":      "JSON",
			"json":      string(b),
			"matchType": defaultBodyMatchType,
		}
	}
}

func coerceResponseBody(body any) any {
	switch v := body.(type) {
	case string:
		if strings.Contains(v, "$!") {
			return v // Velocity template goes raw
		}
		return map[string]any{"type": "JSON", "json": v}
	case map[string]any, []any:
		b, _ := json.Marshal(v)
		return map[string]any{"type": "JSON", "json": string(b)}
	default:
		b, _ := json.Marshal(v)
		return map[string]any{"type": "JSON", "json": string(b)}
	}
}

func ensureProviderAPIKey(provider string) bool {
	envByProvider := map[string]string{
		"anthropic": "ANTHROPIC_API_KEY",
		"openai":    "OPENAI_API_KEY",
	}
	envName := envByProvider[strings.ToLower(provider)]
	if envName == "" {
		// unknown provider: ask custom var
		var name, val string
		_ = survey.AskOne(&survey.Input{Message: "Env var name for this provider API key:"}, &name)
		name = strings.TrimSpace(name)
		if name == "" {
			return false
		}
		if os.Getenv(name) == "" {
			_ = survey.AskOne(&survey.Password{Message: "Enter API key:"}, &val)
			val = strings.TrimSpace(val)
			if val == "" {
				return false
			}
			_ = os.Setenv(name, val)
		}
		return true
	}

	if os.Getenv(envName) != "" {
		return true
	}
	var v string
	_ = survey.AskOne(&survey.Password{Message: fmt.Sprintf("Enter %s:", envName)}, &v)
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	_ = os.Setenv(envName, v)
	return true
}

func printFirstLines(s string, n int) {
	sc := bufio.NewScanner(strings.NewReader(s))
	for i := 0; i < n && sc.Scan(); i++ {
		fmt.Println(sc.Text())
	}
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

// StartLoadTestREPL provides an interactive menu for managing Locust load test bundles.
// It supports generating a bundle, uploading, editing the current bundle, deleting pointer, and viewing status.
// If project is empty, it will prompt to select or create a project.
func StartLoadTestREPL(provider core.Provider, project string) error {
	ctx := context.Background()

	// Ensure project context (select or create)
	if strings.TrimSpace(project) == "" {
		projects, _ := provider.ListProjects(ctx)
		selected, err := ResolveProjectInteractively(projects)
		if err != nil {
			return err
		}
		if selected.ProjectID == "" {
			// create new
			var name string
			if err := survey.AskOne(&survey.Input{Message: "Enter new project name:"}, &name, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
			if err := provider.InitProject(ctx, name); err != nil {
				return fmt.Errorf("init project: %w", err)
			}
			project = name
		} else {
			project = selected.ProjectID
		}
	}

	for {
		// probe pointer
		ptr, _ := provider.GetLoadTestPointer(ctx, project)
		hasActive := ptr != nil && ptr.ActiveVersion != ""

		// Build menu options with user-friendly labels & internal keys (first token before space)
		options := []string{
			"generate-local  – Generate a new bundle from a collection file",
			"upload-local-dir – Upload an existing local bundle directory",
			"view-status     – Show current pointer summary (if any)",
		}
		if hasActive {
			options = append(options,
				"edit-current    – Download & optionally re-upload active bundle",
				"delete-pointer  – Remove current pointer (keep versions)",
				"purge-bundle    – Delete active bundle files (destructive)",
			)
		}
		options = append(options, "exit            – Leave LoadTest REPL")

		fmt.Println("\nℹ️  Actions: generate → optional upload, upload-local-dir → direct cloud storage push, edit-current → modify & version bump.")
		fmt.Println("    Dry-run prompts let you simulate uploads without persisting objects.")

		var choice string
		if err := survey.AskOne(&survey.Select{
			Message: fmt.Sprintf("LoadTest REPL — project: %s", project),
			Options: options,
			Default: options[0],
		}, &choice); err != nil {
			return err
		}

		// Normalize to internal key (first token before space)
		choice = strings.Split(strings.TrimSpace(choice), " ")[0]

		switch choice {
		case "generate-local":
			// Ask for collection inputs and out dir
			var answers struct {
				CollectionFile string `survey:"collectionFile"`
				CollectionType string `survey:"collectionType"`
				OutDir         string `survey:"outDir"`
			}
			_ = survey.Ask([]*survey.Question{
				{Name: "collectionFile", Prompt: &survey.Input{Message: "Collection file (Postman/Bruno/Insomnia path):"}},
				{Name: "collectionType", Prompt: &survey.Select{Message: "Collection type:", Options: []string{"postman", "bruno", "insomnia"}, Default: "postman"}},
				{Name: "outDir", Prompt: &survey.Input{Message: "Output directory:", Default: fmt.Sprintf("loadtest_%d", time.Now().Unix())}},
			}, &answers)

			// Build options; set safe defaults for pointer fields
			headless := false
			distributed := false
			opts := client.Options{
				CollectionType:             answers.CollectionType,
				CollectionPath:             answers.CollectionFile,
				OutDir:                     answers.OutDir,
				Headless:                   &headless,
				GenerateDistributedHelpers: &distributed,
			}
			if err := client.GenerateLoadtestBundle(opts); err != nil {
				fmt.Printf("❌ Generation failed: %v\n", err)
				break
			}
			fmt.Printf("✅ Bundle generated at: %s\n", answers.OutDir)

			var doUpload bool
			_ = survey.AskOne(&survey.Confirm{Message: "Upload this bundle now?", Default: false}, &doUpload)
			if doUpload {
				pointer, version, err := provider.UploadLoadTestBundle(ctx, project, answers.OutDir)
				if err != nil {
					fmt.Printf("❌ Upload failed: %v\n", err)
					break
				}
				fmt.Println("✅ Uploaded:")
				b, _ := json.MarshalIndent(struct {
					Pointer *models.LoadTestPointer `json:"pointer"`
					Version *models.LoadTestVersion `json:"version"`
				}{pointer, version}, "", "  ")
				fmt.Println(string(b))
			}

		case "upload-local-dir":
			var dir string
			_ = survey.AskOne(&survey.Input{Message: "Directory to upload:", Default: "./loadtest"}, &dir)
			dir = strings.TrimSpace(dir)
			if dir == "" {
				break
			}
			if !filepath.IsAbs(dir) {
				cwd, _ := os.Getwd()
				dir = filepath.Join(cwd, dir)
			}
			pointer, version, err := provider.UploadLoadTestBundle(ctx, project, dir)
			if err != nil {
				fmt.Printf("❌ Upload failed: %v\n", err)
				break
			}
			fmt.Println("✅ Uploaded:")
			b, _ := json.MarshalIndent(struct {
				Pointer *models.LoadTestPointer `json:"pointer"`
				Version *models.LoadTestVersion `json:"version"`
			}{pointer, version}, "", "  ")
			fmt.Println(string(b))

		case "edit-current":
			if !hasActive {
				fmt.Println("⚠️  No active bundle pointer found.")
				break
			}
			workdir := fmt.Sprintf("loadtest_edit_%d", time.Now().Unix())
			ptr, localDir, err := provider.DownloadLoadTestBundle(ctx, project, workdir)
			if err != nil {
				fmt.Printf("❌ Download failed: %v\n", err)
				break
			}
			fmt.Printf("📦 Downloaded active bundle (version %s) to: %s\n", ptr.ActiveVersion, localDir)
			var re bool
			_ = survey.AskOne(&survey.Confirm{Message: "Re-upload now as a new version?", Default: false}, &re)
			if re {
				pointer, version, err := provider.UploadLoadTestBundle(ctx, project, localDir)
				if err != nil {
					fmt.Printf("❌ Re-upload failed: %v\n", err)
					break
				}
				fmt.Println("✅ Re-uploaded:")
				b, _ := json.MarshalIndent(struct {
					Pointer *models.LoadTestPointer `json:"pointer"`
					Version *models.LoadTestVersion `json:"version"`
				}{pointer, version}, "", "  ")
				fmt.Println(string(b))
			}

		case "delete-pointer":
			if !hasActive {
				fmt.Println("⚠️  No active pointer to delete.")
				break
			}
			// Desired behavior: remove current pointer AND associated bundle; then roll back pointer to previous version if one exists.
			// Implemented for AWS provider; others fall back to simple pointer delete.
			if awsp, ok := provider.(interface{ GetProviderType() string }); ok && awsp.GetProviderType() == "aws" {
				ap, ok2 := provider.(*awsprov.Provider)
				if !ok2 {
					fmt.Println("⚠️  AWS delete-pointer enhanced flow unavailable (type assertion failed)")
					break
				}
				// Fetch current pointer to know active bundle/version
				curPtr, err := provider.GetLoadTestPointer(ctx, project)
				if err != nil || curPtr == nil || curPtr.ActiveVersion == "" {
					fmt.Println("⚠️  No active pointer found or failed to load; deleting pointer only.")
					_ = provider.DeleteLoadTestPointer(ctx, project)
					break
				}

				// Confirm destructive action
				var confirm bool
				_ = survey.AskOne(&survey.Confirm{Message: fmt.Sprintf("Delete bundle %s and roll back pointer to previous version?", curPtr.BundleID), Default: false}, &confirm)
				if !confirm {
					break
				}

				// 1) Delete the active bundle directory
				bundlePrefix := fmt.Sprintf("configs/%s-loadtest/bundles/%s/", curPtr.ProjectID, curPtr.BundleID)
				deleted := 0
				var token *string
				for {
					resp, err := ap.S3Client.ListObjectsV2(ctx, &s3sdk.ListObjectsV2Input{Bucket: awsSDK.String(ap.BucketName), Prefix: awsSDK.String(bundlePrefix), ContinuationToken: token})
					if err != nil {
						fmt.Printf("❌ List failed: %v\n", err)
						break
					}
					if len(resp.Contents) == 0 {
						break
					}
					for _, obj := range resp.Contents {
						_, derr := ap.S3Client.DeleteObject(ctx, &s3sdk.DeleteObjectInput{Bucket: awsSDK.String(ap.BucketName), Key: obj.Key})
						if derr == nil {
							deleted++
						} else {
							fmt.Printf("⚠️  Failed delete %s: %v\n", awsSDK.ToString(obj.Key), derr)
						}
					}
					if (resp.IsTruncated != nil && *resp.IsTruncated) && resp.NextContinuationToken != nil {
						token = resp.NextContinuationToken
						continue
					}
					break
				}

				// 2) Find previous version (precursor)
				versionsPrefix := fmt.Sprintf("configs/%s-loadtest/versions/", curPtr.ProjectID)
				var versions []string
				token = nil
				for {
					resp, err := ap.S3Client.ListObjectsV2(ctx, &s3sdk.ListObjectsV2Input{Bucket: awsSDK.String(ap.BucketName), Prefix: awsSDK.String(versionsPrefix), ContinuationToken: token})
					if err != nil {
						fmt.Printf("❌ List versions failed: %v\n", err)
						break
					}
					for _, obj := range resp.Contents {
						versions = append(versions, awsSDK.ToString(obj.Key))
					}
					if (resp.IsTruncated != nil && *resp.IsTruncated) && resp.NextContinuationToken != nil {
						token = resp.NextContinuationToken
						continue
					}
					break
				}
				// Sort by version timestamp descending (keys are .../v<ts>.json)
				sort.Slice(versions, func(i, j int) bool { return versions[i] > versions[j] })
				currentKey := fmt.Sprintf("%s%s.json", versionsPrefix, curPtr.ActiveVersion)
				var prevKey string
				for _, k := range versions {
					if k < currentKey { // lexicographic works for v<unix>
						prevKey = k
						break
					}
				}

				if prevKey == "" {
					// No previous version: delete pointer entirely
					if err := provider.DeleteLoadTestPointer(ctx, project); err != nil {
						fmt.Printf("⚠️  Deleted bundle (%d objects) but failed to remove pointer: %v\n", deleted, err)
					} else {
						fmt.Printf("✅ Deleted bundle (%d objects) and removed pointer (no previous version).\n", deleted)
					}
					break
				}

				// 3) Load previous version snapshot to get BundleID, then rewrite pointer
				prevObj, err := ap.S3Client.GetObject(ctx, &s3sdk.GetObjectInput{Bucket: awsSDK.String(ap.BucketName), Key: awsSDK.String(prevKey)})
				if err != nil {
					fmt.Printf("❌ Failed to read previous version: %v\n", err)
					break
				}
				prevData, _ := io.ReadAll(prevObj.Body)
				prevObj.Body.Close()
				var prevVer models.LoadTestVersion
				if err := json.Unmarshal(prevData, &prevVer); err != nil {
					fmt.Printf("❌ Failed to parse previous version: %v\n", err)
					break
				}
				// Rebuild pointer for previous bundle
				files := map[string]string{
					"locustfile":   fmt.Sprintf("configs/%s-loadtest/bundles/%s/locustfile.py", prevVer.ProjectID, prevVer.BundleID),
					"requirements": fmt.Sprintf("configs/%s-loadtest/bundles/%s/requirements.txt", prevVer.ProjectID, prevVer.BundleID),
					"endpoints":    fmt.Sprintf("configs/%s-loadtest/bundles/%s/locust_endpoints.json", prevVer.ProjectID, prevVer.BundleID),
					"user_data":    fmt.Sprintf("configs/%s-loadtest/bundles/%s/user_data.yaml", prevVer.ProjectID, prevVer.BundleID),
					"manifest":     fmt.Sprintf("configs/%s-loadtest/bundles/%s/manifest.json", prevVer.ProjectID, prevVer.BundleID),
				}
				newPtr := models.NewDefaultLoadTestPointer(prevVer.ProjectID, prevVer.Version, prevVer.BundleID, files, &models.LoadTestSummary{Tasks: prevVer.Metrics["tasks"], Endpoints: prevVer.Metrics["endpoints"], HasHost: prevVer.Validation != nil && prevVer.Validation.HostDefined})
				b, _ := json.MarshalIndent(newPtr, "", "  ")
				pointerKey := fmt.Sprintf("configs/%s-loadtest/current.json", prevVer.ProjectID)
				_, err = ap.S3Client.PutObject(ctx, &s3sdk.PutObjectInput{Bucket: awsSDK.String(ap.BucketName), Key: awsSDK.String(pointerKey), Body: bytes.NewReader(b), ContentType: awsSDK.String("application/json")})
				if err != nil {
					fmt.Printf("❌ Failed to update pointer: %v\n", err)
					break
				}
				fmt.Printf("✅ Deleted bundle (%d objects) and rolled back pointer to %s (%s)\n", deleted, prevVer.Version, prevVer.BundleID)
			} else {
				// Fallback: original behavior
				var sure bool
				_ = survey.AskOne(&survey.Confirm{Message: "Delete current pointer only? (versions remain)", Default: false}, &sure)
				if !sure {
					break
				}
				if err := provider.DeleteLoadTestPointer(ctx, project); err != nil {
					fmt.Printf("❌ Delete failed: %v\n", err)
					break
				}
				fmt.Println("✅ Deleted current pointer.")
			}

		case "purge-bundle":
			if !hasActive {
				fmt.Println("⚠️  No active bundle to purge.")
				break
			}
			// New behavior: delete ALL load test artifacts for this project (current pointer, versions, bundles, metadata)
			if awsp, ok := provider.(interface{ GetProviderType() string }); ok && awsp.GetProviderType() == "aws" {
				ap, ok2 := provider.(*awsprov.Provider)
				if !ok2 {
					fmt.Println("⚠️  AWS purge not available (type assertion failed)")
					break
				}
				fullID := fmt.Sprintf("%s-loadtest", ptr.ProjectID)
				var confirm bool
				_ = survey.AskOne(&survey.Confirm{Message: "This will delete ALL artifacts for loadtest. Continue?", Default: false}, &confirm)
				if !confirm {
					break
				}
				var typed string
				_ = survey.AskOne(&survey.Input{Message: "Type 'permanently delete' to confirm:"}, &typed)
				if strings.TrimSpace(typed) != "permanently delete" {
					fmt.Println("❌ Confirmation mismatch.")
					break
				}
				prefixes := []string{fmt.Sprintf("configs/%s/", fullID)}
				totalDeleted := 0
				for _, prefix := range prefixes {
					var token *string
					for {
						resp, err := ap.S3Client.ListObjectsV2(ctx, &s3sdk.ListObjectsV2Input{Bucket: awsSDK.String(ap.BucketName), Prefix: awsSDK.String(prefix), ContinuationToken: token})
						if err != nil {
							fmt.Printf("❌ List failed for %s: %v\n", prefix, err)
							break
						}
						if len(resp.Contents) == 0 {
							break
						}
						for _, obj := range resp.Contents {
							_, derr := ap.S3Client.DeleteObject(ctx, &s3sdk.DeleteObjectInput{Bucket: awsSDK.String(ap.BucketName), Key: obj.Key})
							if derr == nil {
								totalDeleted++
							} else {
								fmt.Printf("⚠️  Failed delete %s: %v\n", awsSDK.ToString(obj.Key), derr)
							}
						}
						if (resp.IsTruncated != nil && *resp.IsTruncated) && resp.NextContinuationToken != nil {
							token = resp.NextContinuationToken
							continue
						}
						break
					}
				}
				// Delete metadata file best-effort
				metaKey := fmt.Sprintf("metadata/%s.json", fullID)
				_, _ = ap.S3Client.DeleteObject(ctx, &s3sdk.DeleteObjectInput{Bucket: awsSDK.String(ap.BucketName), Key: awsSDK.String(metaKey)})
				fmt.Printf("✅ Purged load test artifacts. Deleted ~%d objects; metadata removed (best-effort).\n", totalDeleted)
			} else {
				fmt.Println("⚠️  Purge not implemented for this provider.")
			}

		case "view-status":
			ptr, err := provider.GetLoadTestPointer(ctx, project)
			if err != nil || ptr == nil {
				fmt.Println("ℹ️  No active pointer.")
				break
			}
			b, _ := json.MarshalIndent(ptr, "", "  ")
			fmt.Println(string(b))

		case "exit":
			return nil
		}
	}
}
