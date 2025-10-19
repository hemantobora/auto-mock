package builders

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

// Re-export types from models for backward compatibility
type MockExpectation = models.MockExpectation
type HttpRequest = models.HttpRequest
type HttpResponse = models.HttpResponse
type Times = models.Times
type Delay = models.Delay
type ConnectionOptions = models.ConnectionOptions
type PathMatchingStrategy = models.PathMatchingStrategy
type QueryParamMatchingStrategy = models.QueryParamMatchingStrategy
type RequestBodyMatchingStrategy = models.RequestBodyMatchingStrategy
type Progressive = models.Progressive

// Re-export constants
const (
	PathExact = models.PathExact
	PathRegex = models.PathRegex

	QueryExact  = models.QueryExact
	QueryRegex  = models.QueryRegex
	QuerySubset = models.QuerySubset

	BodyExact   = models.BodyExact
	BodyPartial = models.BodyPartial
	BodyRegex   = models.BodyRegex
)

// ExpectationsToMockServerJSON is a wrapper that calls the function from models
func ExpectationsToMockServerJSON(expectations []MockExpectation) string {
	return models.ExpectationsToMockServerJSON(expectations)
}

// ValidateJSON validates if a string is valid JSON
func ValidateJSON(jsonStr string) error {
	return models.ValidateJSON(jsonStr)
}

// FormatJSON formats JSON string with proper indentation
func FormatJSON(jsonStr string) (string, error) {
	return models.FormatJSON(jsonStr)
}

// IsValidRegex tests if a regex pattern is valid
func IsValidRegex(pattern string) error {
	return models.IsValidRegex(pattern)
}

func CommonStatusCodes() map[string][]StatusCode {
	return map[string][]StatusCode{
		"2xx Success": {
			{Code: 200, Description: "OK - Request successful"},
			{Code: 201, Description: "Created - Resource created successfully"},
			{Code: 202, Description: "Accepted - Request accepted for processing"},
			{Code: 204, Description: "No Content - Success with no response body"},
		},
		"3xx Redirection": {
			{Code: 301, Description: "Moved Permanently"},
			{Code: 302, Description: "Found - Temporary redirect"},
			{Code: 304, Description: "Not Modified"},
		},
		"4xx Client Error": {
			{Code: 400, Description: "Bad Request - Invalid request syntax"},
			{Code: 401, Description: "Unauthorized - Authentication required"},
			{Code: 403, Description: "Forbidden - Access denied"},
			{Code: 404, Description: "Not Found - Resource not found"},
			{Code: 409, Description: "Conflict - Resource conflict"},
			{Code: 422, Description: "Unprocessable Entity - Validation error"},
			{Code: 429, Description: "Too Many Requests - Rate limited"},
		},
		"5xx Server Error": {
			{Code: 500, Description: "Internal Server Error"},
			{Code: 502, Description: "Bad Gateway"},
			{Code: 503, Description: "Service Unavailable"},
			{Code: 504, Description: "Gateway Timeout"},
		},
	}
}

// StatusCode represents an HTTP status code with description
type StatusCode struct {
	Code        int    `json:"code"`
	Description string `json:"description"`
}

// RegexPatterns returns comprehensive regex patterns with examples
func RegexPatterns() map[string]RegexPattern {
	return map[string]RegexPattern{
		"Basic Patterns": {
			Pattern:     ".* (any), \\d+ (numbers), \\w+ (words), \\s+ (whitespace)",
			Description: "Fundamental regex building blocks",
			Examples:    []string{".*", "\\d+", "\\w+", "\\s+"},
		},
		"Numbers": {
			Pattern:     `\\d+`,
			Description: "One or more digits",
			Examples:    []string{"123", "7", "999"},
		},
		"Decimal Numbers": {
			Pattern:     `\\d+\\.\\d+`,
			Description: "Decimal numbers (e.g., prices, coordinates)",
			Examples:    []string{"12.99", "3.14159", "0.75"},
		},
		"UUID": {
			Pattern:     `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
			Description: "Standard UUID format (case-insensitive)",
			Examples:    []string{"550e8400-e29b-41d4-a716-446655440000"},
		},
		"Email": {
			Pattern:     `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}`,
			Description: "Email address format",
			Examples:    []string{"user@example.com", "test.email+tag@domain.org"},
		},
		"Words": {
			Pattern:     `\\w+`,
			Description: "One or more word characters (letters, digits, underscore)",
			Examples:    []string{"hello", "test123", "user_name"},
		},
		"Alphanumeric": {
			Pattern:     `[a-zA-Z0-9]+`,
			Description: "Letters and numbers only (no symbols)",
			Examples:    []string{"abc123", "Test789", "ID42"},
		},
		"Custom ID": {
			Pattern:     `[a-zA-Z0-9_-]+`,
			Description: "Letters, numbers, underscore, hyphen",
			Examples:    []string{"user-123", "item_abc", "order-789"},
		},
		"Date Formats": {
			Pattern:     `\\d{4}-\\d{2}-\\d{2}`,
			Description: "ISO date format (YYYY-MM-DD)",
			Examples:    []string{"2025-09-21", "2024-12-31", "2023-01-15"},
		},
		"Time Formats": {
			Pattern:     `\\d{2}:\\d{2}(:\\d{2})?`,
			Description: "Time format (HH:MM or HH:MM:SS)",
			Examples:    []string{"15:30", "09:45:30", "23:59"},
		},
		"Phone Numbers": {
			Pattern:     `\\+?[0-9()-\\s]+`,
			Description: "Flexible phone number format",
			Examples:    []string{"+1-555-123-4567", "(555) 123-4567", "+44 20 7946 0958"},
		},
		"URLs": {
			Pattern:     `https?://[\\w.-]+(/[\\w.-]*)*/?`,
			Description: "HTTP/HTTPS URLs",
			Examples:    []string{"https://api.example.com", "http://localhost:3000/api"},
		},
		"IP Addresses": {
			Pattern:     `\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}`,
			Description: "IPv4 address format",
			Examples:    []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"},
		},
		"Wildcards": {
			Pattern:     "NEW|OLD|ACTIVE (alternatives), .*test.* (contains), ^prefix.* (starts with)",
			Description: "Common wildcard patterns",
			Examples:    []string{"NEW|OLD", ".*test.*", "^api_"},
		},
		"Case Insensitive": {
			Pattern:     `[nN][eE][wW]`,
			Description: "Case-insensitive matching for specific words",
			Examples:    []string{"[nN][eE][wW]", "[aA][pP][iI]", "[tT][eE][sS][tT]"},
		},
		"Anchors & Boundaries": {
			Pattern:     "^start, end$, \\\\b (word boundary)",
			Description: "Anchors for precise matching",
			Examples:    []string{"^api", "v1$", "\\\\buser\\\\b"},
		},
	}
}

// RegexPattern represents a regex pattern with documentation
type RegexPattern struct {
	Pattern     string   `json:"pattern"`
	Description string   `json:"description"`
	Examples    []string `json:"examples"`
	Category    string   `json:"category,omitempty"`
	Difficulty  string   `json:"difficulty,omitempty"`
}

// GetCommonRegexPatterns returns a map of commonly used regex patterns for quick access
// This function provides easy access to the most frequently used regex patterns
func GetCommonRegexPatterns() map[string]string {
	return map[string]string{
		"numbers":      "\\d+",                                                         // 123, 456, 789
		"words":        "\\w+",                                                         // user, test123, user_name
		"alphanumeric": "[a-zA-Z0-9]+",                                                 // abc123, Test789, ID42
		"custom_id":    "[a-zA-Z0-9_-]+",                                               // user-123, item_abc, order-789
		"date":         "\\d{4}-\\d{2}-\\d{2}",                                         // 2025-09-21, 2024-12-31
		"time":         "\\d{2}:\\d{2}(:\\d{2})?",                                      // 15:30, 09:45:30
		"uuid":         "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", // UUID format
		"email":        "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}",              // user@example.com
		"url":          "https?://[\\w.-]+(/[\\w.-]*)*/?",                              // https://api.example.com
		"ip":           "\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}",                    // 192.168.1.1
		"phone":        "\\+?[0-9()-\\s]+",                                             // +1-555-123-4567
		"decimal":      "\\d+\\.\\d+",                                                  // 12.99, 3.14159
		"any":          ".*",                                                           // Any characters
		"whitespace":   "\\s+",                                                         // One or more spaces
	}
}

// GetRegexDescription returns a human-readable description and examples for common patterns
func GetRegexDescription(pattern string) (description string, examples []string) {
	descriptions := map[string]struct {
		desc     string
		examples []string
	}{
		"\\d+":                    {"One or more digits", []string{"123", "456", "789"}},
		"\\w+":                    {"One or more word characters", []string{"user", "test123", "user_name"}},
		"[a-zA-Z0-9]+":            {"Letters and numbers only", []string{"abc123", "Test789", "ID42"}},
		"[a-zA-Z0-9_-]+":          {"Letters, numbers, underscore, hyphen", []string{"user-123", "item_abc", "order-789"}},
		"\\d{4}-\\d{2}-\\d{2}":    {"ISO date format (YYYY-MM-DD)", []string{"2025-09-21", "2024-12-31", "2023-01-15"}},
		"\\d{2}:\\d{2}(:\\d{2})?": {"Time format (HH:MM or HH:MM:SS)", []string{"15:30", "09:45:30", "23:59"}},
		"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}": {"UUID format", []string{"550e8400-e29b-41d4-a716-446655440000"}},
		"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}":              {"Email address format", []string{"user@example.com", "test.email+tag@domain.org"}},
		"https?://[\\w.-]+(/[\\w.-]*)*/?":                              {"HTTP/HTTPS URLs", []string{"https://api.example.com", "http://localhost:3000/api"}},
		"\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}":                    {"IPv4 address format", []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"}},
		"\\+?[0-9()-\\s]+": {"Flexible phone number format", []string{"+1-555-123-4567", "(555) 123-4567", "+44 20 7946 0958"}},
		"\\d+\\.\\d+":      {"Decimal numbers", []string{"12.99", "3.14159", "0.75"}},
		".*":               {"Any characters (wildcard)", []string{"any", "characters", "here"}},
		"\\s+":             {"One or more whitespace characters", []string{" ", "  ", "\t"}},
	}

	if info, exists := descriptions[pattern]; exists {
		return info.desc, info.examples
	}
	return "Custom regex pattern", []string{"pattern specific examples"}
}

// ensure maps exist before writes
func ensureMaps(m *MockExpectation) {
	if m.HttpRequest != nil && m.HttpRequest.Headers == nil {
		m.HttpRequest.Headers = map[string][]any{}
	}
	if m.HttpResponse != nil && m.HttpResponse.Headers == nil {
		m.HttpResponse.Headers = map[string][]string{}
	}
	if m.HttpRequest != nil && m.HttpRequest.QueryStringParameters == nil {
		m.HttpRequest.QueryStringParameters = map[string][]string{}
	}
}

func CloneExpectation(src *MockExpectation) *MockExpectation {
	if src == nil {
		return nil
	}
	dst := *src // copy scalars

	// ---- HttpRequest ----
	if src.HttpRequest != nil {
		dst.HttpRequest = new(models.HttpRequest)
		*dst.HttpRequest = *src.HttpRequest // copy scalars

		// PathParameters: map[string][]string
		if src.HttpRequest.PathParameters != nil {
			dst.HttpRequest.PathParameters = make(map[string][]string, len(src.HttpRequest.PathParameters))
			for k, v := range src.HttpRequest.PathParameters {
				cp := make([]string, len(v))
				copy(cp, v)
				dst.HttpRequest.PathParameters[k] = cp
			}
		}

		// QueryStringParameters: map[string][]string
		if src.HttpRequest.QueryStringParameters != nil {
			dst.HttpRequest.QueryStringParameters = make(map[string][]string, len(src.HttpRequest.QueryStringParameters))
			for k, v := range src.HttpRequest.QueryStringParameters {
				cp := make([]string, len(v))
				copy(cp, v)
				dst.HttpRequest.QueryStringParameters[k] = cp
			}
		}

		// Headers: map[string][]any   (deep copy slice + elements)
		if src.HttpRequest.Headers != nil {
			dst.HttpRequest.Headers = make(map[string][]any, len(src.HttpRequest.Headers))
			for k, sv := range src.HttpRequest.Headers {
				cp := make([]any, len(sv))
				for i := range sv {
					cp[i] = deepCopyInterface(sv[i]) // ensure objects like {"regex": "..."} are cloned
				}
				dst.HttpRequest.Headers[k] = cp
			}
		}

		// Body: any
		if src.HttpRequest.Body != nil {
			dst.HttpRequest.Body = deepCopyInterface(src.HttpRequest.Body)
		}
	}

	// ---- HttpResponse ----
	if src.HttpResponse != nil {
		dst.HttpResponse = new(models.HttpResponse)
		*dst.HttpResponse = *src.HttpResponse // copy scalars

		// Headers: map[string][]string
		if src.HttpResponse.Headers != nil {
			dst.HttpResponse.Headers = make(map[string][]string, len(src.HttpResponse.Headers))
			for k, v := range src.HttpResponse.Headers {
				cp := make([]string, len(v))
				copy(cp, v)
				dst.HttpResponse.Headers[k] = cp
			}
		}

		// Body: any
		if src.HttpResponse.Body != nil {
			dst.HttpResponse.Body = deepCopyInterface(src.HttpResponse.Body)
		}

		// Delay: *Delay
		if src.HttpResponse.Delay != nil {
			tmp := *src.HttpResponse.Delay
			dst.HttpResponse.Delay = &tmp
		}
	}

	// ---- Other pointers ----
	if src.Times != nil {
		tmp := *src.Times
		dst.Times = &tmp
	}
	if src.ConnectionOptions != nil {
		tmp := *src.ConnectionOptions
		dst.ConnectionOptions = &tmp
	}

	return &dst
}

func deepCopyInterface(v interface{}) interface{} {
	b, _ := json.Marshal(v)
	var out interface{}
	_ = json.Unmarshal(b, &out)
	return out
}

// Enhanced regex pattern collection with comprehensive validation and hints
func collectRegexPattern(expectation *MockExpectation) error {
	fmt.Println("\n📝 Enhanced Regex Pattern Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show quick common patterns first
	fmt.Println("⚡ Quick Common Patterns:")
	fmt.Println("   \\d+           - Numbers (123, 456)")
	fmt.Println("   \\w+           - Words (user, test123)")
	fmt.Println("   [a-zA-Z0-9]+  - Alphanumeric (abc123)")
	fmt.Println("   .*            - Any characters")
	fmt.Println("   /api/users/\\d+ - Users with numeric ID")

	// Ask if user wants to see full library
	var showFullLibrary bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Show complete regex pattern library?",
		Default: false,
		Help:    "View comprehensive patterns with examples and descriptions",
	}, &showFullLibrary); err != nil {
		return err
	}

	if showFullLibrary {
		// Show comprehensive patterns with categories
		fmt.Println("\n💡 Comprehensive Regex Pattern Library:")
		patterns := RegexPatterns()
		for name, pattern := range patterns {
			fmt.Printf("\n   📂 %s:\n", name)
			fmt.Printf("      Pattern: %s\n", pattern.Pattern)
			fmt.Printf("      Description: %s\n", pattern.Description)
			fmt.Printf("      Examples: %s\n", strings.Join(pattern.Examples, ", "))
		}
	}

	fmt.Println("\n🔧 Regex Quick Reference:")
	fmt.Println("   . = any character          \\d = digit           \\w = word char")
	fmt.Println("   * = zero or more           + = one or more      ? = zero or one")
	fmt.Println("   ^ = start of string        $ = end of string   \\b = word boundary")
	fmt.Println("   [abc] = any of a,b,c      [^abc] = not a,b,c   | = OR")
	fmt.Println("   () = grouping              {} = exact count     [] = character class")

	var useTemplate bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Would you like to select from common patterns?",
		Default: true,
		Help:    "Choose from pre-built patterns or create custom regex",
	}, &useTemplate); err != nil {
		return err
	}

	var regexPattern string

	if useTemplate {
		// Quick selection menu with most common patterns
		var selectedPattern string
		if err := survey.AskOne(&survey.Select{
			Message: "Select a pattern:",
			Options: []string{
				"\\d+ - Numbers (user IDs, order numbers)",
				"\\w+ - Words (usernames, names)",
				"[a-zA-Z0-9]+ - Alphanumeric (codes, tokens)",
				"[a-zA-Z0-9_-]+ - IDs with dashes/underscores",
				"\\d{4}-\\d{2}-\\d{2} - Dates (YYYY-MM-DD)",
				"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12} - UUIDs",
				".* - Any characters (wildcard)",
				"browse-all - Browse complete pattern library",
				"custom - Create custom pattern",
			},
			Default: "\\d+ - Numbers (user IDs, order numbers)",
		}, &selectedPattern); err != nil {
			return err
		}

		// Handle browse-all option
		if strings.HasPrefix(selectedPattern, "browse-all") {
			// Show full library and let user select
			patterns := RegexPatterns()
			var patternOptions []string
			for name := range patterns {
				patternOptions = append(patternOptions, name)
			}
			patternOptions = append(patternOptions, "custom - Create custom pattern")

			if err := survey.AskOne(&survey.Select{
				Message: "Select from complete library:",
				Options: patternOptions,
			}, &selectedPattern); err != nil {
				return err
			}
		}

		if selectedPattern == "custom - Create custom pattern" {
			useTemplate = false
		} else if strings.HasPrefix(selectedPattern, "\\d+") {
			// Quick pattern: Numbers
			regexPattern = "\\d+"
			fmt.Printf("\n✅ Selected: Numbers pattern (\\d+)\n")
			fmt.Printf("   Matches: 123, 456, 789, 1001\n")
		} else if strings.HasPrefix(selectedPattern, "\\w+") {
			// Quick pattern: Words
			regexPattern = "\\w+"
			fmt.Printf("\n✅ Selected: Words pattern (\\w+)\n")
			fmt.Printf("   Matches: user, test123, user_name\n")
		} else if strings.HasPrefix(selectedPattern, "[a-zA-Z0-9]+") {
			// Quick pattern: Alphanumeric
			regexPattern = "[a-zA-Z0-9]+"
			fmt.Printf("\n✅ Selected: Alphanumeric pattern ([a-zA-Z0-9]+)\n")
			fmt.Printf("   Matches: abc123, Test789, ID42\n")
		} else if strings.HasPrefix(selectedPattern, "[a-zA-Z0-9_-]+") {
			// Quick pattern: IDs with dashes/underscores
			regexPattern = "[a-zA-Z0-9_-]+"
			fmt.Printf("\n✅ Selected: ID pattern ([a-zA-Z0-9_-]+)\n")
			fmt.Printf("   Matches: user-123, item_abc, order-789\n")
		} else if strings.HasPrefix(selectedPattern, "\\d{4}-\\d{2}-\\d{2}") {
			// Quick pattern: Dates
			regexPattern = "\\d{4}-\\d{2}-\\d{2}"
			fmt.Printf("\n✅ Selected: Date pattern (\\d{4}-\\d{2}-\\d{2})\n")
			fmt.Printf("   Matches: 2025-09-21, 2024-12-31, 2023-01-15\n")
		} else if strings.Contains(selectedPattern, "[0-9a-f]{8}-") {
			// Quick pattern: UUIDs
			regexPattern = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
			fmt.Printf("\n✅ Selected: UUID pattern\n")
			fmt.Printf("   Matches: 550e8400-e29b-41d4-a716-446655440000\n")
		} else if strings.HasPrefix(selectedPattern, ".*") {
			// Quick pattern: Wildcard
			regexPattern = ".*"
			fmt.Printf("\n✅ Selected: Wildcard pattern (.*)\n")
			fmt.Printf("   Matches: Any characters\n")
		} else {
			// From complete library
			patterns := RegexPatterns()
			if pattern, exists := patterns[selectedPattern]; exists {
				regexPattern = pattern.Examples[0] // Use first example as default
				fmt.Printf("\n💡 Selected pattern: %s\n", pattern.Pattern)
				fmt.Printf("   Description: %s\n", pattern.Description)
				fmt.Printf("   Default example: %s\n", regexPattern)

				var customize bool
				if err := survey.AskOne(&survey.Confirm{
					Message: "Customize this pattern?",
					Default: false,
				}, &customize); err != nil {
					return err
				}

				if customize {
					useTemplate = false
				}
			} else {
				useTemplate = false
			}
		}
	}

	if !useTemplate {
		if err := survey.AskOne(&survey.Input{
			Message: "Enter custom regex pattern for path:",
			Default: regexPattern,
			Help:    "Use patterns above or create custom regex. Test at regex101.com",
		}, &regexPattern); err != nil {
			return err
		}
	}

	fmt.Println("\n📚 Professional Regex Resources:")
	fmt.Println("   Interactive Testing: https://regex101.com/")
	fmt.Println("   Learning Tutorial: https://regexone.com/")
	fmt.Println("   Reference Guide: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Regular_Expressions")
	fmt.Println("   MockServer Patterns: https://mock-server.com/mock_server/request_matchers.html#regex-matcher")

	return nil
}

func ReviewGraphQLExpectation(exp *MockExpectation) error {
	fmt.Println("\n🔄 Review and Confirm")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Optional custom header you sometimes set upstream
	var opType string
	if exp.HttpRequest != nil && exp.HttpRequest.Headers != nil {
		if vals, ok := exp.HttpRequest.Headers["X-GraphQL-Operation-Type"]; ok && len(vals) > 0 {
			switch v := vals[0].(type) {
			case string:
				opType = v
			case map[string]string:
				opType = v["regex"]
			}
		}
	}

	// Safe getters
	method := ""
	path := ""
	if exp.HttpRequest != nil {
		method = exp.HttpRequest.Method
		path = exp.HttpRequest.Path
	}

	status := 200
	if exp.HttpResponse != nil && exp.HttpResponse.StatusCode > 0 {
		status = exp.HttpResponse.StatusCode
	}

	// Count request headers excluding the optional op-type header
	reqHeaderCount := 0
	if exp.HttpRequest != nil && exp.HttpRequest.Headers != nil {
		for k := range exp.HttpRequest.Headers {
			if strings.EqualFold(k, "X-GraphQL-Operation-Type") {
				continue
			}
			reqHeaderCount++
		}
	}

	// Work out body match mode & whether variables are present
	bodyMode, hasVars := summarizeGraphQLBody(exp)

	// Display summary
	fmt.Printf("\n📋 GraphQL Expectation Summary:\n")
	if exp.Description != "" {
		fmt.Printf("   Description: %s\n", exp.Description)
	}
	if opType != "" {
		fmt.Printf("   Operation Type: %s\n", opType)
	}
	fmt.Printf("   Endpoint: %s %s\n", method, path)
	fmt.Printf("   Status Code: %d\n", status)

	if reqHeaderCount > 0 {
		fmt.Printf("   Request Headers: %d\n", reqHeaderCount)
	}

	// Request matching summary (POST vs GET)
	if exp.HttpRequest != nil && strings.EqualFold(method, "GET") {
		q := exp.HttpRequest.QueryStringParameters
		_, hasQuery := q["query"]
		_, hasOpName := q["operationName"]
		_, hasV := q["variables"]
		fmt.Printf("   Transport: GET (query string)\n")
		fmt.Printf("   Query present: %v, OperationName: %v, Variables: %v\n", hasQuery, hasOpName, hasV)
	} else {
		fmt.Printf("   Transport: POST (application/json)\n")
		fmt.Printf("   Body match mode: %s, Variables: %v\n", bodyMode, hasVars)
	}

	var confirm bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Create this GraphQL expectation?",
		Default: true,
	}, &confirm); err != nil {
		return err
	}
	if !confirm {
		return &models.ExpectationBuildError{
			ExpectationType: "GraphQL",
			Step:            "Review and Confirm",
			Cause:           fmt.Errorf("expectation creation cancelled by user"),
		}
	}
	return nil
}

func ExtendExpectationsForProgressive(expectations []MockExpectation) []MockExpectation {
	fmt.Println("\n🚀 Extending Expectations for Progressive Responses")

	// 1) Find starting max priority
	maxPriority := 0
	for i := range expectations {
		if expectations[i].Priority > maxPriority {
			maxPriority = expectations[i].Priority
		}
	}

	added := 0

	// 2) Walk the original slice by index so edits stick
	for i := range expectations {
		// ensure monotonically increasing priorities on originals
		maxPriority++
		if expectations[i].Priority < maxPriority {
			expectations[i].Priority = maxPriority
		}

		p := expectations[i].Progressive
		if p == nil || p.Step <= 0 || p.Base < 0 || p.Cap < p.Base {
			continue
		}

		// 3) Generate progressive clones
		delay := p.Base + p.Step
		for delay <= p.Cap {
			clone := CloneExpectation(&expectations[i])

			// Description / delay
			if clone.Description != "" {
				clone.Description = fmt.Sprintf("%s [Progressive delay: %d ms]", clone.Description, delay)
			} else {
				clone.Description = fmt.Sprintf("Progressive delay: %d ms", delay)
			}

			// Times: fire once for all but the last; last can be unlimited
			next := delay + p.Step
			if next <= p.Cap {
				clone.Times = &Times{RemainingTimes: 1}
			} else {
				clone.Times = &Times{Unlimited: true} // no remainingTimes when unlimited
			}

			// Response delay
			if clone.HttpResponse == nil {
				clone.HttpResponse = &HttpResponse{}
			}
			clone.HttpResponse.Delay = &Delay{
				TimeUnit: "MILLISECONDS",
				Value:    delay,
			}

			// Give each progressive a unique, increasing priority
			maxPriority++
			clone.Priority = maxPriority

			// Append to the same slice we're returning
			expectations = append(expectations, *clone)
			added++

			delay = next
		}
	}

	fmt.Printf("   Added %d progressive expectations; total: %d\n", added, len(expectations))
	return expectations
}
