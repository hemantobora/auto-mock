package builders

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

type MockConfigurator struct {
}

// ---- Body matchers (map-based) ----

type MatchType string

const (
	MatchStrict             MatchType = "STRICT"
	MatchOnlyMatchingFields MatchType = "ONLY_MATCHING_FIELDS"
)

func NewJSONBody(value any, mt MatchType) map[string]any {
	m := map[string]any{
		"type": "JSON",
		"json": value,
	}
	if mt != "" {
		m["matchType"] = string(mt)
	}
	return m
}

func NewRegexBody(pattern string) map[string]any {
	return map[string]any{
		"type":  "REGEX",
		"regex": pattern,
	}
}

type NameValues struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

func NewParametersBody(params []NameValues) map[string]any {
	out := make([]map[string]any, 0, len(params))
	for _, p := range params {
		out = append(out, map[string]any{
			"name":   p.Name,
			"values": p.Values,
		})
	}
	return map[string]any{
		"type":       "PARAMETERS",
		"parameters": out,
	}
}

func (mc *MockConfigurator) EditRequestBody(exp *MockExpectation) error {
	// Choose matcher type
	var kind string
	if err := survey.AskOne(&survey.Select{
		Message: "Choose body matcher type:",
		Options: []string{"JSON", "JSON_SCHEMA", "REGEX", "PARAMETERS", "STRING (exact text)"},
		Default: "JSON",
	}, &kind, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	switch {
	case kind == "JSON_SCHEMA":
		var rootType string
		if err := survey.AskOne(&survey.Select{
			Message: "Root body type:",
			Options: []string{"object", "array"},
			Default: "object",
		}, &rootType); err != nil {
			return err
		}

		var schema map[string]any

		if rootType == "array" {
			// Interactive construction for arrays is not supported.
			// Accept the full schema directly.
			fmt.Println("ℹ️  Interactive field construction is only supported for object bodies.")
			fmt.Println("   Please paste the full JSON Schema below.")
			var schemaJSON string
			if err := survey.AskOne(&survey.Multiline{
				Message: "Paste JSON Schema:",
				Help:    `e.g. {"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}}`,
			}, &schemaJSON); err != nil {
				return err
			}
			schemaJSON = strings.TrimSpace(schemaJSON)
			if !json.Valid([]byte(schemaJSON)) {
				return fmt.Errorf("invalid JSON Schema: not valid JSON")
			}
			if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
				return err
			}
		} else {
			// Interactive field-by-field construction for object bodies.
			fmt.Println("Define the fields to match. Empty field name to finish.")
			type fieldDef struct {
				name     string
				typ      string
				pattern  string
				required bool
			}
			var fields []fieldDef
			for {
				var name string
				if err := survey.AskOne(&survey.Input{
					Message: "Field name (empty to finish):",
				}, &name); err != nil {
					return err
				}
				name = strings.TrimSpace(name)
				if name == "" {
					break
				}

				var typ string
				if err := survey.AskOne(&survey.Select{
					Message: fmt.Sprintf("Type for '%s':", name),
					Options: []string{"string", "number", "integer", "boolean"},
					Default: "string",
				}, &typ); err != nil {
					return err
				}

				var pattern string
				if typ == "string" {
					if err := survey.AskOne(&survey.Input{
						Message: fmt.Sprintf("Pattern for '%s' (Java regex, optional):", name),
						Help:    "e.g. ^[a-f0-9-]{36}$ for UUID. Leave blank to accept any string.",
					}, &pattern); err != nil {
						return err
					}
					pattern = strings.TrimSpace(pattern)
				}

				var required bool
				if err := survey.AskOne(&survey.Confirm{
					Message: fmt.Sprintf("Is '%s' required?", name),
					Default: true,
				}, &required); err != nil {
					return err
				}

				fields = append(fields, fieldDef{name: name, typ: typ, pattern: pattern, required: required})
			}

			if len(fields) == 0 {
				return fmt.Errorf("no fields defined for JSON_SCHEMA matcher")
			}

			properties := map[string]any{}
			var requiredFields []string
			for _, f := range fields {
				prop := map[string]any{"type": f.typ}
				if f.pattern != "" {
					prop["pattern"] = f.pattern
				}
				properties[f.name] = prop
				if f.required {
					requiredFields = append(requiredFields, f.name)
				}
			}
			schema = map[string]any{
				"type":       "object",
				"properties": properties,
			}
			if len(requiredFields) > 0 {
				schema["required"] = requiredFields
			}
		}

		exp.HttpRequest.Body = map[string]any{
			"type":       "JSON_SCHEMA",
			"jsonSchema": schema,
		}
		return nil

	case kind == "JSON":
		// Ask for JSON and optional matchType
		var bodyJSON string
		if err := survey.AskOne(&survey.Multiline{
			Message: "Paste JSON to match (object/array):",
			Help:    "We’ll wrap as {\"type\":\"JSON\",\"json\":...}.",
		}, &bodyJSON); err != nil {
			return err
		}
		bodyJSON = strings.TrimSpace(bodyJSON)
		// Validate JSON
		if !json.Valid([]byte(bodyJSON)) {
			fmt.Println("⚠️  That is not valid JSON. You can still continue.")
			var cont bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Continue anyway (stored as STRING exact match)?",
				Default: false,
			}, &cont); err != nil {
				return err
			}
			if !cont {
				return fmt.Errorf("invalid JSON provided for request body")
			}
			// Fallback: exact STRING match to keep user-provided text intact
			exp.HttpRequest.Body = map[string]any{"type": "STRING", "string": bodyJSON}
			return nil
		}
		// Parse to generic value (preserves numbers/bools/etc.)
		var v any
		if err := json.Unmarshal([]byte(bodyJSON), &v); err != nil {
			return err
		}

		var mt string
		if err := survey.AskOne(&survey.Select{
			Message: "Match type for JSON:",
			Options: []string{string(MatchOnlyMatchingFields), string(MatchStrict)},
			Default: string(MatchOnlyMatchingFields),
			Help:    "ONLY_MATCHING_FIELDS ignores extra fields on the incoming request.",
		}, &mt); err != nil {
			return err
		}
		exp.HttpRequest.Body = NewJSONBody(v, MatchType(mt))
		return nil

	case kind == "REGEX":
		var pattern string
		if err := survey.AskOne(&survey.Input{
			Message: "Enter regex pattern (Java regex, runs on MockServer JVM):",
			Help:    "Use (?s) prefix to match newlines. e.g. (?s).*\"type\"\\s*:\\s*\"MOCK\".*",
		}, &pattern, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		// Note: we intentionally skip Go/RE2 compile-check here because MockServer uses
		// Java regex (java.util.regex.Pattern), which supports features RE2 rejects
		// (e.g. (?s) DOTALL flag, lookaheads, backreferences). Validating with Go's
		// regexp.Compile would give false errors on perfectly valid Java patterns.
		exp.HttpRequest.Body = NewRegexBody(pattern)
		return nil

	case kind == "PARAMETERS":
		// Collect name=values lines like: username=alice ; role=admin,user
		fmt.Println("Enter name=values (comma-separated). Empty line to finish.")
		var items []NameValues
		for {
			var line string
			if err := survey.AskOne(&survey.Input{
				Message: "param (e.g. role=admin,user):",
			}, &line); err != nil {
				return err
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				fmt.Println("↩︎  Please use name=val[,val2]")
				continue
			}
			name := strings.TrimSpace(parts[0])
			vals := strings.Split(parts[1], ",")
			for i := range vals {
				vals[i] = strings.TrimSpace(vals[i])
			}
			if name == "" || len(vals) == 0 || (len(vals) == 1 && vals[0] == "") {
				fmt.Println("↩︎  Need a name and at least one value")
				continue
			}
			items = append(items, NameValues{Name: name, Values: vals})
		}
		if len(items) == 0 {
			return fmt.Errorf("no parameters provided")
		}
		exp.HttpRequest.Body = NewParametersBody(items)
		return nil

	default: // STRING (exact text)
		var s string
		if err := survey.AskOne(&survey.Multiline{
			Message: "Paste exact body text to match:",
		}, &s); err != nil {
			return err
		}
		exp.HttpRequest.Body = map[string]any{
			"type":   "STRING",
			"string": s,
		}
		return nil
	}
}

// CollectRequestBody asks user what kind of matcher to use and stores a schema-valid body wrapper.
func (mc *MockConfigurator) CollectRequestBody(exp *MockExpectation, existing string) error {
	// If a body text was derived earlier, optionally reuse it exactly.
	existing = strings.TrimSpace(existing)

	// Detect Bruno/Postman {{variable}} placeholders in the body template.
	// These are never the real values sent at runtime (they get substituted before the request
	// is made), so offering them as an exact match would produce a matcher that never fires.
	doublebraceRe := regexp.MustCompile(`\{\{[^}]+\}\}`)
	hasTemplatePlaceholders := existing != "" && doublebraceRe.MatchString(existing)

	if hasTemplatePlaceholders {
		// Show the body for reference so the user can copy/adapt it, but skip the exact-match offer.
		fmt.Printf("⚠️  Request body contains template variables — cannot use as an exact match.\n")
		fmt.Printf("   Body (for reference):\n%s\n", existing)

		var needsBody bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Do you want to configure a custom body matcher?",
			Default: false,
			Help:    "Choose ‘Yes’ to set up a JSON, JSON_SCHEMA, regex, or string matcher.",
		}, &needsBody); err != nil {
			return err
		}
		if !needsBody {
			return nil
		}
		return mc.EditRequestBody(exp)
	}

	if existing != "" {
		var useBody bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Use existing request body text as EXACT match?\n" + existing,
			Default: false,
			Help:    "JSON bodies use JSON STRICT matching; non-JSON bodies use exact STRING matching.",
		}, &useBody); err != nil {
			return err
		}
		if useBody {
			// Prefer JSON STRICT if ‘existing’ is valid JSON; otherwise fall back to exact STRING
			trimmed := existing
			if json.Valid([]byte(trimmed)) {
				var v any
				if err := json.Unmarshal([]byte(trimmed), &v); err == nil {
					exp.HttpRequest.Body = NewJSONBody(v, MatchStrict)
					return nil
				}
			}
			// Fallback: exact STRING match (raw text)
			exp.HttpRequest.Body = map[string]any{
				"type":   "STRING",
				"string": trimmed,
			}
			return nil
		}
	}

	var needsBody bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Do you want to match a request body?",
		Default: false,
		Help:    "Choose ‘No’ to skip body matching.",
	}, &needsBody); err != nil {
		return err
	}
	if !needsBody {
		return nil
	}
	return mc.EditRequestBody(exp)
}

func (mc *MockConfigurator) CollectQueryParameterMatching(exp *MockExpectation) error {
	fmt.Printf("\n🔍 Query Parameter Matching\n")

	// Already configured?
	if n := len(exp.HttpRequest.QueryStringParameters); n > 0 {
		fmt.Printf("ℹ️  Already configured %d query parameters from path\n", n)

		var addMore bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Add additional query parameters?",
			Default: false,
		}, &addMore); err != nil {
			return err
		}
		if !addMore {
			fmt.Printf("✅ Query Parameters: %d configured\n", n)
			return nil
		}
	} else {
		var needs bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Does this endpoint require specific query parameters?",
			Default: false,
			Help:    "Only specify if you need to match exact query parameter values",
		}, &needs); err != nil {
			return err
		}
		if !needs {
			fmt.Println("ℹ️  No query parameter matching configured")
			return nil
		}
		exp.HttpRequest.QueryStringParameters = []models.NameValues{}
	}

	for {
		var name string
		if err := survey.AskOne(&survey.Input{
			Message: "Parameter name (empty to finish):",
			Help:    "e.g., 'page', 'limit', 'category'",
		}, &name); err != nil {
			return err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			break
		}

		var value string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Value(s) for '%s' (comma-separated, regex allowed):", name),
			Help:    "Example: admin,user  or  ^cat.*$",
		}, &value); err != nil {
			return err
		}

		// Split on commas, trim each
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) == 0 {
			fmt.Println("↩︎  Skipped (no values provided)")
			continue
		}

		SetNameValues(&exp.HttpRequest.QueryStringParameters, name, out)
		fmt.Printf("✅ Added: %s=%v\n", name, out)
	}

	fmt.Printf("✅ Query Parameters: %d configured\n", len(exp.HttpRequest.QueryStringParameters))
	return nil
}

// parsePathAndQueryParams intelligently separates path from query parameters
func (mc *MockConfigurator) ParsePathAndQueryParams(fullPath string) (cleanPath string, queryParams map[string][]string) {
	queryParams = make(map[string][]string)

	// Ensure path starts with /
	if !strings.HasPrefix(fullPath, "/") {
		fullPath = "/" + fullPath
	}

	// Parse URL to separate path and query
	parsedURL, err := url.Parse(fullPath)
	if err != nil {
		// If parsing fails, return as-is
		return fullPath, queryParams
	}

	cleanPath = parsedURL.Path

	// Extract query parameters, skipping any whose values are Bruno/Postman template
	// placeholders (e.g. {{page_number}}). Those are never the real runtime values
	// and would produce matchers that never fire.
	doublebraceRe := regexp.MustCompile(`\{\{[^}]+\}\}`)
	for name, values := range parsedURL.Query() {
		var filtered []string
		for _, v := range values {
			if doublebraceRe.MatchString(v) {
				fmt.Printf("ℹ️  Skipping query param '%s={{...}}' — template placeholder, not a real value\n", name)
				continue
			}
			filtered = append(filtered, v)
		}
		if len(filtered) > 0 {
			queryParams[name] = filtered
		}
	}

	return cleanPath, queryParams
}

func (mc *MockConfigurator) CollectPathMatchingStrategy(exp *MockExpectation) error {
	fmt.Println("\n🛤️ Path Matching Strategy")

	rawPath := strings.TrimSpace(exp.HttpRequest.Path)
	if rawPath == "" {
		return fmt.Errorf("path is empty")
	}

	// Normalize Bruno/Postman {{var}} double-brace syntax to MockServer {var} single-brace.
	// e.g. /offer/aeid/{{aeid}}/imei/{{imei}} → /offer/aeid/{aeid}/imei/{imei}
	doublebraceRe := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	if doublebraceRe.MatchString(rawPath) {
		rawPath = doublebraceRe.ReplaceAllString(rawPath, "{$1}")
		exp.HttpRequest.Path = rawPath
		fmt.Printf("ℹ️  Normalized path variables: %s\n", rawPath)
	}

	hasBraces := strings.Contains(rawPath, "{") && strings.Contains(rawPath, "}")

	// ─────────────────────────────────────────────────────────────────────────────
	// Case 1: no {params} → exact vs regex (path stays a STRING)
	// ─────────────────────────────────────────────────────────────────────────────
	if !hasBraces {
		var useRegex bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Use regex pattern matching for this path?",
			Default: false,
			Help:    "Regex allows flexible matching (e.g. ^/users/[a-z0-9-]+/posts$).",
		}, &useRegex); err != nil {
			return err
		}

		if useRegex {
			var pattern string
			if err := survey.AskOne(&survey.Input{
				Message: "Enter regex for path (as a string):",
				Default: regexp.QuoteMeta(rawPath),
			}, &pattern, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid path regex: %w", err)
			}
			exp.HttpRequest.Path = pattern // regex in string form (valid for MockServer)
			fmt.Printf("🔍 Using regex path (string): %s\n", pattern)
		} else {
			exp.HttpRequest.Path = rawPath // exact literal
			fmt.Println("ℹ️  Using exact string match for path")
			fmt.Printf("🔍 Path: %s (exact)\n", rawPath)
		}

		fmt.Println("✅ Path matching configured")
		return nil
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// Case 2: templated path with {params}
	// Keep the templated path STRING (MockServer matches it as a path-template)
	// Collect pathParameters as map[string][]string (regex strings or exact values)
	// ─────────────────────────────────────────────────────────────────────────────
	fmt.Printf("ℹ️  Path parameters detected in: %s\n", rawPath)
	exp.HttpRequest.Path = rawPath

	if exp.HttpRequest.PathParameters == nil {
		exp.HttpRequest.PathParameters = make(map[string][]string)
	}

	// Extract param names like {id}
	nameRe := regexp.MustCompile(`\{([^}/]+)\}`)
	matches := nameRe.FindAllStringSubmatch(rawPath, -1)
	seen := map[string]bool{}

	for _, m := range matches {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true

		var valuesLine string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Regex or comma-separated values for {%s}:", name),
			Default: "[^/]+", // common “any segment” regex
			Help:    "Examples → values: 123,456  • regex: ^[0-9]{1,6}$  • simple: [A-Z0-9\\-]+",
		}, &valuesLine, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		valuesLine = strings.TrimSpace(valuesLine)

		// Allow either a single regex/value or a comma list
		var vals []string
		if strings.Contains(valuesLine, ",") {
			parts := strings.Split(valuesLine, ",")
			for _, p := range parts {
				if v := strings.TrimSpace(p); v != "" {
					vals = append(vals, v)
				}
			}
		} else {
			vals = []string{valuesLine}
		}

		// Optional: validate entries that look like regex
		for _, v := range vals {
			looksRegex := strings.HasPrefix(v, "^") || strings.ContainsAny(v, `[]{}+*?|().\^$`)
			if looksRegex {
				if _, err := regexp.Compile(v); err != nil {
					return fmt.Errorf("invalid regex for {%s}: %w", name, err)
				}
			}
		}

		exp.HttpRequest.PathParameters[name] = vals
	}

	fmt.Println("💡 Path parameters will be matched via pathParameters (each entry can be a regex string).")
	fmt.Println("✅ Path matching configured")
	return nil
}

// Step 4: Request Header Matching
func (mc *MockConfigurator) CollectResponseHeader(exp *MockExpectation) error {
	fmt.Printf("\n📝 Response Headers\n")

	var needsHeaders bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Does this response require specific headers?",
		Default: false,
		Help:    "e.g., Content-Type, CORS headers",
	}, &needsHeaders); err != nil {
		return err
	}
	if !needsHeaders {
		fmt.Println("ℹ️  No response header configured")
		return nil
	}

	for {
		var headerName string
		if err := survey.AskOne(&survey.Input{
			Message: "Header name (empty to finish):",
			Help:    "e.g., 'Content-Type'",
			Default: "Content-Type",
		}, &headerName); err != nil {
			return err
		}
		headerName = strings.TrimSpace(headerName)
		if headerName == "" {
			break
		}

		var headerValue string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Value for '%s':", headerName),
			Help:    "e.g., application/json",
			Default: "application/json",
		}, &headerValue); err != nil {
			return err
		}
		headerValue = strings.TrimSpace(headerValue)
		if headerValue == "" {
			continue
		}

		// Append value to response header (case-insensitive name)
		appendNameValues(&exp.HttpResponse.Headers, headerName, headerValue)
		fmt.Printf("✅ Added header: %s: %q\n", headerName, headerValue)
	}

	fmt.Printf("✅ Response Headers: %d configured\n", len(exp.HttpResponse.Headers))
	return nil
}

func parseCSVValues(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// CollectRequestHeaderMatching builds HttpRequest.Headers as []NameValues with exact matching.
func (mc *MockConfigurator) CollectRequestHeaderMatching(exp *models.MockExpectation) error {
	fmt.Printf("\n📝 Request Header Matching\n")

	var needsHeaders bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Does this request require specific headers to match?",
		Default: false,
		Help:    "e.g., Authorization, Content-Type, API keys",
	}, &needsHeaders); err != nil {
		return err
	}
	if !needsHeaders {
		fmt.Println("ℹ️  No request header matching configured")
		return nil
	}

	// Ensure slice init
	if exp.HttpRequest.Headers == nil {
		exp.HttpRequest.Headers = []models.NameValues{}
	}

	for {
		var headerName string
		if err := survey.AskOne(&survey.Input{
			Message: "Header name (empty to finish):",
			Help:    "e.g., 'Authorization', 'Content-Type'",
		}, &headerName); err != nil {
			return err
		}
		headerName = strings.TrimSpace(headerName)
		if headerName == "" {
			break
		}

		var valuesCSV string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Exact value(s) for '%s' (comma-separated for multiple):", headerName),
			Help:    "Examples: 'Bearer abc123' or 'application/json, application/xml'",
		}, &valuesCSV); err != nil {
			return err
		}
		values := parseCSVValues(valuesCSV)

		// upsert into []NameValues
		if idx := headerIndex(exp.HttpRequest.Headers, headerName); idx >= 0 {
			exp.HttpRequest.Headers[idx].Values = values
			fmt.Printf("✅ Updated header: %s: %s\n", headerName, strings.Join(values, ", "))
		} else {
			exp.HttpRequest.Headers = append(exp.HttpRequest.Headers, models.NameValues{
				Name:   headerName,
				Values: values,
			})
			fmt.Printf("✅ Added header: %s: %s\n", headerName, strings.Join(values, ", "))
		}
	}

	fmt.Printf("✅ Request Headers: %d configured\n", len(exp.HttpRequest.Headers))
	return nil
}

func (mc *MockConfigurator) CollectAdvancedFeatures(expectation *MockExpectation) error {
	fmt.Printf("\n⚙️ Advanced MockServer Features\n")

	// 3.2 Feature picker
	reg := Registry()
	choices, err := PickFeaturesInteractively(reg)
	if err != nil {
		return err
	}

	// 3.3 Apply picked features (will trigger sub-dialogs as needed)
	for _, feats := range choices {
		if err := feats.Apply(expectation); err != nil {
			return fmt.Errorf("feature %q failed: %w", feats.Key, err)
		}
	}
	fmt.Println("✅ Advanced features configured")
	return nil
}
