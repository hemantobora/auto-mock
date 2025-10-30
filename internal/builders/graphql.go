package builders

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

// BuildGraphQLExpectationWithContext builds with context of existing expectations.
func BuildGraphQLExpectationWithContext() (MockExpectation, error) {
	var exp MockExpectation
	exp.HttpRequest = &HttpRequest{}
	exp.HttpResponse = &HttpResponse{}
	exp.HttpResponse.Headers = make(map[string][]string)

	fmt.Println("🧬 Starting GraphQL Expectation Builder (POST/GET JSON only)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Step 1: Endpoint path (usually /graphql)
	if err := collectGraphQLPath(exp.HttpRequest); err != nil {
		return exp, err
	}

	// Step 2: HTTP method
	method, err := selectGraphQLMethod()
	if err != nil {
		return exp, err
	}
	exp.HttpRequest.Method = method

	// Step 3: Collect operation shell (query + operationName)
	gqlQuery, err := collectGraphQLQueryAndOp()
	if err != nil {
		return exp, err
	}

	// Step 4: Collect variables (optional)
	variables, err := collectGraphQLVariables()
	if err != nil {
		return exp, err
	}

	// Step 5: Apply request matching model depending on method
	if strings.EqualFold(method, "GET") {
		applyGETRequest(exp.HttpRequest, gqlQuery, variables)
	} else {
		applyPOSTRequest(exp.HttpRequest, gqlQuery, variables)
	}

	// Step 6: Collect response (JSON only)
	if err := collectGraphQLResponseJSON(exp.HttpResponse); err != nil {
		return exp, err
	}

	// Step 7: Optional status code
	var status int
	if err := survey.AskOne(&survey.Input{
		Message: "HTTP status code? (default 200)",
		Default: "200",
	}, &status, survey.WithValidator(optionalIntValidator)); err == nil && status > 0 {
		exp.HttpResponse.StatusCode = status
	} else {
		exp.HttpResponse.StatusCode = 200
	}
	var mock_configurator MockConfigurator
	mock_configurator.CollectResponseHeader(0, &exp)
	mock_configurator.CollectAdvancedFeatures(0, &exp)

	if err := ReviewGraphQLExpectation(&exp); err != nil {
		return exp, err
	}
	return exp, nil
}

// ─── Collectors ─────────────────────────────────────────────────────────────────

func collectGraphQLPath(req *HttpRequest) error {
	var path string
	defaultPath := "/graphql"
	if req.Path != "" {
		defaultPath = req.Path
	}
	if err := survey.AskOne(&survey.Input{
		Message: "GraphQL endpoint path:",
		Default: defaultPath,
		Help:    "Typically '/graphql'. Regex is allowed if you need flexibility.",
	}, &path, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	path = strings.TrimSpace(path)
	// Optionally allow regex
	var useRegex bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Treat path as regex?",
		Default: false,
	}, &useRegex); err != nil {
		return err
	}
	if useRegex {
		if _, err := regexp.Compile(path); err != nil {
			return fmt.Errorf("invalid path regex: %w", err)
		}
	}
	req.Path = path
	return nil
}

func selectGraphQLMethod() (string, error) {
	var method string
	if err := survey.AskOne(&survey.Select{
		Message: "HTTP method:",
		Options: []string{"POST", "GET"},
		Default: "POST",
	}, &method, survey.WithValidator(survey.Required)); err != nil {
		return "", err
	}
	return method, nil
}

func collectGraphQLQueryAndOp() (query string, err error) {
	if err = survey.AskOne(&survey.Multiline{
		Message: "Paste GraphQL query (operation):",
		Help:    "Example: query GetUser($id:ID!){ user(id:$id){ id name } }",
	}, &query, survey.WithValidator(survey.Required)); err != nil {
		return
	}
	query = strings.TrimSpace(query)
	return
}

func collectGraphQLVariables() (vars map[string]any, err error) {
	var wantVars bool
	if err = survey.AskOne(&survey.Confirm{
		Message: "Add variables?",
		Default: true,
	}, &wantVars); err != nil {
		return nil, err
	}
	if !wantVars {
		return nil, nil
	}
	var raw string
	if err = survey.AskOne(&survey.Multiline{
		Message: "Variables JSON (e.g., {\"id\":\"123\"}):",
	}, &raw); err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("variables must be valid JSON")
	}
	if err = json.Unmarshal([]byte(raw), &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

// ─── Apply request by method ────────────────────────────────────────────────────
func applyPOSTRequest(req *HttpRequest, query string, variables map[string]any) {
	if req.Headers == nil {
		req.Headers = map[string][]any{}
	}
	req.Headers["Content-Type"] = []any{"application/json"}
	_, opName := ExtractGraphQLOperationName(query)

	envelope := map[string]any{"query": query}
	if variables != nil {
		envelope["variables"] = variables
	}
	if opName != "" {
		envelope["operationName"] = opName
	}

	var mt string
	if err := survey.AskOne(&survey.Select{
		Message: "Match type for JSON:",
		Options: []string{string(MatchOnlyMatchingFields), string(MatchStrict)},
		Default: string(MatchOnlyMatchingFields),
		Help:    "ONLY_MATCHING_FIELDS ignores extra fields on the incoming request.",
	}, &mt); err != nil {
		mt = string(MatchOnlyMatchingFields)
	}

	req.Body = map[string]any{
		"type":      "JSON",
		"json":      envelope,
		"matchType": mt, // "STRICT" or "ONLY_MATCHING_FIELDS"
	}
}

func applyGETRequest(req *HttpRequest, query string, variables map[string]any) {
	// GET encodes query & variables in the URL
	if req.QueryStringParameters == nil {
		req.QueryStringParameters = map[string][]string{}
	}
	req.QueryStringParameters["query"] = []string{query}
	if variables != nil {
		// Variables must be a JSON string in the query param.
		b, _ := json.Marshal(variables)
		req.QueryStringParameters["variables"] = []string{string(b)}
	}
}

// ─── Response JSON ─────────────────────────────────────────────────────────────

func collectGraphQLResponseJSON(resp *HttpResponse) error {
	var payload string
	if err := survey.AskOne(&survey.Multiline{
		Message: "Response JSON payload (data / errors):",
		Help:    `Example: {"data":{"user":{"id":"123","name":"Ada"}}}`,
	}, &payload, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	payload = strings.TrimSpace(payload)
	if !json.Valid([]byte(payload)) {
		return fmt.Errorf("response must be valid JSON")
	}
	var v any
	if err := json.Unmarshal([]byte(payload), &v); err != nil {
		return err
	}
	// Ensure Content-Type
	if resp.Headers == nil {
		resp.Headers = map[string][]string{}
	}
	resp.Headers["Content-Type"] = []string{"application/json"}
	// Set body wrapper
	resp.Body = map[string]any{
		"type": "JSON",
		"json": v,
	}
	return nil
}

// summarizeGraphQLBody inspects your request body wrapper and reports match mode + variables presence.
// Supports: {type:JSON,json:{...},matchType:...} and {type:REGEX,regex:...}
func summarizeGraphQLBody(exp *MockExpectation) (mode string, hasVars bool) {
	mode = "N/A"
	if exp == nil || exp.HttpRequest == nil || exp.HttpRequest.Body == nil {
		return mode, false
	}

	// POST JSON wrapper
	if m, ok := exp.HttpRequest.Body.(map[string]any); ok {
		if tRaw, ok := m["type"]; ok {
			t, _ := tRaw.(string)
			if strings.EqualFold(t, "JSON") {
				mode = "STRICT" // default if not set
				if mtRaw, ok := m["matchType"]; ok {
					if mt, _ := mtRaw.(string); mt != "" {
						mode = mt
					}
				}
				// Check variables in JSON envelope
				if j, ok := m["json"].(map[string]any); ok {
					_, hasVars = j["variables"]
					return mode, hasVars
				}
				return mode, false
			}
			if strings.EqualFold(t, "REGEX") {
				return "REGEX", false
			}
		}
	}
	return mode, false
}

// ─── Validators ────────────────────────────────────────────────────────────────

func optionalIntValidator(ans interface{}) error {
	s := fmt.Sprint(ans)
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// cheap check
	for _, r := range s {
		if r < '0' || r > '9' {
			return fmt.Errorf("must be a number")
		}
	}
	return nil
}
