package repl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/atotto/clipboard"
	"github.com/hemantobora/auto-mock/internal/mcp"
	"github.com/hemantobora/auto-mock/internal/models"
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
