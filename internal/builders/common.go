package builders

import (
	"encoding/json"
	"regexp"
)

// MockExpectation represents a complete mock server expectation
type MockExpectation struct {
	// Identification
	Name        string `json:"name,omitempty"`        // User-friendly name for identification
	Description string `json:"description,omitempty"` // Optional detailed description

	// Request matching
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	QueryParams map[string]string `json:"queryParams,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	HeaderTypes map[string]string `json:"headerTypes,omitempty"` // "exact" or "regex"
	Body        interface{}       `json:"body,omitempty"`

	// Response
	StatusCode      int               `json:"statusCode"`
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`
	ResponseBody    interface{}       `json:"responseBody"`

	// Advanced features
	ResponseDelay     string             `json:"responseDelay,omitempty"`
	Times             *Times             `json:"times,omitempty"`
	Callbacks         *CallbackConfig    `json:"callbacks,omitempty"`
	ConnectionOptions *ConnectionOptions `json:"connectionOptions,omitempty"`
	Priority          int                `json:"priority,omitempty"`
}

// Times represents MockServer times configuration
type Times struct {
	RemainingTimes int  `json:"remainingTimes,omitempty"`
	Unlimited      bool `json:"unlimited"`
}

// CallbackConfig represents MockServer callback configuration
type CallbackConfig struct {
	CallbackClass string        `json:"callbackClass,omitempty"`
	HttpCallback  *HttpCallback `json:"httpCallback,omitempty"`
}

// HttpCallback represents HTTP callback configuration
type HttpCallback struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    interface{}       `json:"body,omitempty"`
}

// ConnectionOptions represents MockServer connection options
type ConnectionOptions struct {
	SuppressConnectionErrors bool `json:"suppressConnectionErrors,omitempty"`
	SuppressContentLength    bool `json:"suppressContentLength,omitempty"`
	ChunkedEncoding          bool `json:"chunkedEncoding,omitempty"`
	KeepAlive                bool `json:"keepAlive,omitempty"`
	CloseSocket              bool `json:"closeSocket,omitempty"`
	DropConnection           bool `json:"dropConnection,omitempty"`
}

// PathMatchingStrategy represents how paths should be matched
type PathMatchingStrategy string

const (
	PathExact PathMatchingStrategy = "exact"
	PathRegex PathMatchingStrategy = "regex"
)

// QueryParamMatchingStrategy represents how query parameters should be matched
type QueryParamMatchingStrategy string

const (
	QueryExact  QueryParamMatchingStrategy = "exact"
	QueryRegex  QueryParamMatchingStrategy = "regex"
	QuerySubset QueryParamMatchingStrategy = "subset"
)

// RequestBodyMatchingStrategy represents how request body should be matched
type RequestBodyMatchingStrategy string

const (
	BodyExact   RequestBodyMatchingStrategy = "exact"
	BodyPartial RequestBodyMatchingStrategy = "partial"
	BodyRegex   RequestBodyMatchingStrategy = "regex"
)

// ExpectationsToMockServerJSON converts expectations to MockServer JSON format
func ExpectationsToMockServerJSON(expectations []MockExpectation) string {
	var mockServerExpectations []map[string]interface{}

	for _, expectation := range expectations {
		mockServerExp := map[string]interface{}{
			"httpRequest":  buildHttpRequest(expectation),
			"httpResponse": buildHttpResponse(expectation),
		}

		// Add times if specified
		if expectation.Times != nil {
			mockServerExp["times"] = expectation.Times
		}

		// Add priority if specified
		if expectation.Priority != 0 {
			mockServerExp["priority"] = expectation.Priority
		}

		// Add callbacks if specified
		if expectation.Callbacks != nil {
			if expectation.Callbacks.HttpCallback != nil {
				mockServerExp["httpCallback"] = expectation.Callbacks.HttpCallback
			}
			if expectation.Callbacks.CallbackClass != "" {
				mockServerExp["callback"] = map[string]interface{}{
					"callbackClass": expectation.Callbacks.CallbackClass,
				}
			}
		}

		// Add connection options if specified
		if expectation.ConnectionOptions != nil {
			mockServerExp["connectionOptions"] = expectation.ConnectionOptions
		}

		mockServerExpectations = append(mockServerExpectations, mockServerExp)
	}

	jsonBytes, err := json.MarshalIndent(mockServerExpectations, "", "  ")
	if err != nil {
		return "[]" // Fallback to empty array
	}

	return string(jsonBytes)
}

// buildHttpRequest builds the httpRequest part of MockServer expectation
func buildHttpRequest(expectation MockExpectation) map[string]interface{} {
	request := map[string]interface{}{
		"method": expectation.Method,
		"path":   expectation.Path,
	}

	// Add name for identification (not part of MockServer spec but used for management)
	if expectation.Name != "" {
		request["name"] = expectation.Name
	}

	// Add query parameters if present
	if len(expectation.QueryParams) > 0 {
		queryParams := make(map[string][]string)
		for key, value := range expectation.QueryParams {
			queryParams[key] = []string{value}
		}
		request["queryStringParameters"] = queryParams
	}

	// Add headers if present
	if len(expectation.Headers) > 0 {
		headers := make(map[string]interface{})
		for key, value := range expectation.Headers {
			// Check if this header should use regex matching
			if expectation.HeaderTypes != nil && expectation.HeaderTypes[key] == "regex" {
				headers[key] = map[string]interface{}{
					"matcher": "regex",
					"value":   value,
				}
			} else {
				// Default to exact matching
				headers[key] = value
			}
		}
		request["headers"] = headers
	}

	// Add body if present
	if expectation.Body != nil {
		request["body"] = expectation.Body
	}

	return request
}

// buildHttpResponse builds the httpResponse part of MockServer expectation
func buildHttpResponse(expectation MockExpectation) map[string]interface{} {
	response := map[string]interface{}{
		"statusCode": expectation.StatusCode,
		"body":       expectation.ResponseBody,
	}

	// Add default headers
	headers := map[string]string{
		"Content-Type":                 "application/json",
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type, Authorization",
	}

	// Merge with custom headers
	for key, value := range expectation.ResponseHeaders {
		headers[key] = value
	}

	response["headers"] = headers

	// Add delay if specified
	if expectation.ResponseDelay != "" {
		response["delay"] = map[string]interface{}{
			"timeUnit": "MILLISECONDS",
			"value":    expectation.ResponseDelay,
		}
	}

	return response
}

// ValidateJSON validates if a string is valid JSON
func ValidateJSON(jsonStr string) error {
	var temp interface{}
	return json.Unmarshal([]byte(jsonStr), &temp)
}

// FormatJSON formats JSON string with proper indentation
func FormatJSON(jsonStr string) (string, error) {
	var temp interface{}
	if err := json.Unmarshal([]byte(jsonStr), &temp); err != nil {
		return jsonStr, err
	}

	formatted, err := json.MarshalIndent(temp, "", "  ")
	if err != nil {
		return jsonStr, err
	}

	return string(formatted), nil
}

// CommonHeaders returns common HTTP headers with descriptions
func CommonHeaders() map[string]string {
	return map[string]string{
		"Authorization":   "Bearer token, API key, etc.",
		"Content-Type":    "application/json, application/xml, etc.",
		"Accept":          "application/json, text/html, etc.",
		"User-Agent":      "Client application identifier",
		"X-API-Key":       "API key authentication",
		"X-Request-ID":    "Request tracking identifier",
		"X-Forwarded-For": "Client IP address",
		"Cache-Control":   "Caching directives",
	}
}

// CommonStatusCodes returns status codes grouped by category
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

// IsValidRegex tests if a regex pattern is valid
func IsValidRegex(pattern string) error {
	_, err := regexp.Compile(pattern)
	return err
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

// AdvancedFeatureCategories returns organized advanced feature categories
func AdvancedFeatureCategories() map[string][]string {
	return map[string][]string{
		"Response Behavior": {
			"delays - Add response delays (fixed/random/progressive)",
			"limits - Limit response count/times with reset patterns",
			"priority - Set expectation priority with conflict resolution",
			"headers - Custom response headers with dynamic values",
			"caching - Cache control and ETags",
			"compression - Response compression settings",
		},
		"Dynamic Content": {
			"templating - Echo request data with advanced processing",
			"sequences - Multi-stage response sequences",
			"conditions - Complex conditional logic trees",
			"state-machine - Stateful response patterns",
			"data-generation - Realistic fake data generation",
			"interpolation - Advanced string interpolation",
		},
		"Integration & Callbacks": {
			"webhooks - HTTP callbacks with retry logic",
			"custom-code - Java callback classes with context",
			"forward - Smart request forwarding with fallbacks",
			"proxy - Advanced proxying capabilities",
			"transformation - Request/response transformation",
			"event-streaming - Real-time event streaming",
		},
		"Connection Control": {
			"drop-connection - Network failure simulation",
			"chunked-encoding - Transfer encoding control",
			"keep-alive - Connection persistence patterns",
			"error-simulation - TCP/HTTP error simulation",
			"bandwidth - Bandwidth throttling",
			"ssl-behavior - SSL/TLS behavior simulation",
		},
		"Testing Scenarios": {
			"circuit-breaker - Service failure patterns",
			"rate-limiting - Rate limit testing with backoff",
			"chaos-engineering - Advanced chaos patterns",
			"load-testing - Performance testing patterns",
			"resilience - Resilience testing scenarios",
			"security - Security testing patterns",
		},
		"Advanced Patterns": {
			"stateful-mocking - Stateful interaction patterns",
			"workflow-simulation - Multi-step workflow mocking",
			"event-driven - Event-driven architecture simulation",
			"microservice-patterns - Microservice interaction patterns",
			"api-versioning - API version behavior simulation",
			"tenant-isolation - Multi-tenant behavior patterns",
		},
	}
}

// TemplateVariables returns available MockServer template variables
func TemplateVariables() map[string][]string {
	return map[string][]string{
		"Request Data": {
			"${request.pathParameters.id} - Path parameter values",
			"${request.queryParameters.limit} - Query parameter values",
			"${request.headers.authorization} - Request header values",
			"${request.body.user.email} - Request body field values",
			"${request.method} - HTTP method (GET, POST, etc.)",
			"${request.path} - Request path",
			"${request.url} - Full request URL",
			"${request.protocol} - Protocol (HTTP/HTTPS)",
			"${request.port} - Server port",
			"${request.cookies.sessionId} - Cookie values",
		},
		"Generated Values": {
			"${uuid} - Random UUID v4",
			"${timestamp} - Unix timestamp",
			"${now} - ISO datetime string",
			"${random.integer} - Random integer (0-1000)",
			"${random.string} - Random alphanumeric string",
			"${random.boolean} - Random true/false",
			"${random.float} - Random decimal (0.0-1.0)",
			"${random.email} - Random email address",
			"${random.phone} - Random phone number",
			"${random.company} - Random company name",
			"${random.person.firstName} - Random first name",
			"${random.person.lastName} - Random last name",
			"${random.address.street} - Random street address",
			"${random.address.city} - Random city name",
			"${random.ip} - Random IP address",
		},
		"Mathematical": {
			"${math.add(5,3)} - Addition operation",
			"${math.subtract(10,4)} - Subtraction operation",
			"${math.multiply(6,7)} - Multiplication operation",
			"${math.divide(10,2)} - Division operation",
			"${math.modulo(10,3)} - Modulo operation",
			"${math.random} - Random decimal (0-1)",
			"${math.randomInt(1,100)} - Random integer in range",
			"${math.round(3.14159,2)} - Round to decimals",
			"${math.abs(-5)} - Absolute value",
			"${math.min(5,10)} - Minimum value",
			"${math.max(5,10)} - Maximum value",
		},
		"String Operations": {
			"${string.toLowerCase(VALUE)} - Convert to lowercase",
			"${string.toUpperCase(VALUE)} - Convert to uppercase",
			"${string.substring(VALUE,0,5)} - Extract substring",
			"${string.replace(VALUE,'old','new')} - Replace text",
			"${string.trim(VALUE)} - Remove whitespace",
			"${string.length(VALUE)} - String length",
			"${string.concat(A,B,C)} - Concatenate strings",
			"${string.split(VALUE,',')} - Split string",
			"${string.contains(VALUE,'search')} - Check if contains",
			"${string.startsWith(VALUE,'prefix')} - Check prefix",
			"${string.endsWith(VALUE,'suffix')} - Check suffix",
			"${string.reverse(VALUE)} - Reverse string",
			"${string.base64Encode(VALUE)} - Base64 encode",
			"${string.base64Decode(VALUE)} - Base64 decode",
			"${string.urlEncode(VALUE)} - URL encode",
			"${string.urlDecode(VALUE)} - URL decode",
		},
		"Date & Time": {
			"${date.now} - Current date (YYYY-MM-DD)",
			"${date.format(VALUE,'YYYY-MM-DD')} - Format date",
			"${date.addDays(VALUE,7)} - Add days to date",
			"${date.addHours(VALUE,2)} - Add hours to date",
			"${date.year(VALUE)} - Extract year",
			"${date.month(VALUE)} - Extract month",
			"${date.day(VALUE)} - Extract day",
			"${date.hour(VALUE)} - Extract hour",
			"${date.minute(VALUE)} - Extract minute",
			"${date.isoString} - ISO 8601 string",
			"${date.unixTimestamp} - Unix timestamp",
		},
		"Array Operations": {
			"${array.length(VALUE)} - Array length",
			"${array.get(VALUE,0)} - Get element by index",
			"${array.first(VALUE)} - First element",
			"${array.last(VALUE)} - Last element",
			"${array.contains(VALUE,'item')} - Check if contains",
			"${array.join(VALUE,',')} - Join with separator",
			"${array.slice(VALUE,1,3)} - Extract slice",
			"${array.reverse(VALUE)} - Reverse array",
			"${array.sort(VALUE)} - Sort array",
			"${array.random(VALUE)} - Random element",
		},
		"Conditional Logic": {
			"${if(CONDITION,TRUE_VALUE,FALSE_VALUE)} - Conditional expression",
			"${equals(A,B)} - Check equality",
			"${notEquals(A,B)} - Check inequality",
			"${greaterThan(A,B)} - Greater than comparison",
			"${lessThan(A,B)} - Less than comparison",
			"${and(A,B)} - Logical AND",
			"${or(A,B)} - Logical OR",
			"${not(A)} - Logical NOT",
			"${isEmpty(VALUE)} - Check if empty",
			"${isNull(VALUE)} - Check if null",
		},
		"Context & State": {
			"${context.requestCount} - Number of requests processed",
			"${context.sessionId} - Session identifier",
			"${context.userId} - User identifier from context",
			"${state.get('key')} - Get state value",
			"${state.set('key','value')} - Set state value",
			"${state.increment('counter')} - Increment counter",
			"${state.exists('key')} - Check if state exists",
			"${cache.get('key')} - Get cached value",
			"${cache.set('key','value',3600)} - Set cache with TTL",
		},
	}
}

// AdvancedTestingPatterns returns sophisticated testing patterns
func AdvancedTestingPatterns() map[string]TestingPattern {
	return map[string]TestingPattern{
		"Circuit Breaker": {
			Name:        "Circuit Breaker Pattern",
			Description: "Simulate service degradation and recovery patterns",
			Scenarios: []string{
				"Gradual failure increase (10% -> 50% -> 90%)",
				"Immediate failure with exponential recovery",
				"Random failure spikes with baseline stability",
				"Time-based failure windows (business hours)",
			},
			Configuration: map[string]interface{}{
				"failureThreshold": 50,
				"recoveryTime":     30,
				"healthCheckURL":   "/health",
			},
		},
		"Rate Limiting": {
			Name:        "Rate Limiting Scenarios",
			Description: "Test rate limiting algorithms and backoff strategies",
			Scenarios: []string{
				"Token bucket with burst allowance",
				"Sliding window rate limiting",
				"Fixed window with reset timing",
				"User-based vs global rate limits",
				"Rate limit escalation (warnings -> errors)",
			},
			Configuration: map[string]interface{}{
				"requestsPerMinute": 100,
				"burstSize":         10,
				"windowSize":        60,
			},
		},
		"Chaos Engineering": {
			Name:        "Chaos Engineering Patterns",
			Description: "Advanced chaos testing scenarios",
			Scenarios: []string{
				"Latency injection with distribution curves",
				"Intermittent connection drops",
				"Memory pressure simulation",
				"Cascading failure propagation",
				"Byzantine failure patterns",
				"Split-brain scenarios",
			},
			Configuration: map[string]interface{}{
				"chaosPercentage": 15,
				"maxLatency":      5000,
				"minLatency":      100,
			},
		},
		"Load Testing": {
			Name:        "Load Testing Patterns",
			Description: "Performance and scalability testing scenarios",
			Scenarios: []string{
				"Gradual ramp-up with sustained load",
				"Spike testing with quick load increases",
				"Stress testing beyond normal capacity",
				"Volume testing with large datasets",
				"Endurance testing for memory leaks",
				"Breakpoint testing to find limits",
			},
			Configuration: map[string]interface{}{
				"maxConcurrentUsers": 1000,
				"rampUpTime":         300,
				"sustainTime":        600,
			},
		},
		"Security Testing": {
			Name:        "Security Testing Patterns",
			Description: "Security vulnerability testing scenarios",
			Scenarios: []string{
				"Authentication bypass attempts",
				"SQL injection simulation",
				"Cross-site scripting (XSS) patterns",
				"CSRF token validation",
				"Rate limiting bypass attempts",
				"Privilege escalation scenarios",
			},
			Configuration: map[string]interface{}{
				"enableSecurityHeaders": true,
				"logSecurityEvents":     true,
				"blockMaliciousIPs":     true,
			},
		},
		"Resilience Testing": {
			Name:        "Resilience Testing Patterns",
			Description: "System resilience and recovery testing",
			Scenarios: []string{
				"Graceful degradation under load",
				"Automatic failover and recovery",
				"Data consistency during failures",
				"Timeout and retry mechanism testing",
				"Bulkhead isolation effectiveness",
				"Disaster recovery procedures",
			},
			Configuration: map[string]interface{}{
				"maxRetries":       3,
				"backoffStrategy":  "exponential",
				"timeoutThreshold": 30,
			},
		},
	}
}

// TestingPattern represents a comprehensive testing pattern
type TestingPattern struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Scenarios     []string               `json:"scenarios"`
	Configuration map[string]interface{} `json:"configuration"`
	Category      string                 `json:"category,omitempty"`
	Difficulty    string                 `json:"difficulty,omitempty"`
	DocumentURL   string                 `json:"documentUrl,omitempty"`
}

// MockServerFeatures returns advanced MockServer feature configurations
func MockServerFeatures() map[string]MockServerFeature {
	return map[string]MockServerFeature{
		"Response Templating": {
			Name:        "Advanced Response Templating",
			Description: "Dynamic response generation with complex templating",
			Features: []string{
				"Request data interpolation",
				"Mathematical expressions",
				"Conditional logic",
				"String manipulation",
				"Date/time formatting",
				"Random data generation",
				"State management",
			},
			Examples: []string{
				"${if(equals(request.method,'POST'),'created','retrieved')}",
				"${string.toUpperCase(request.pathParameters.name)}",
				"${math.add(request.body.quantity,10)}",
				"${date.format(now,'yyyy-MM-dd HH:mm:ss')}",
			},
		},
		"Request Matching": {
			Name:        "Advanced Request Matching",
			Description: "Sophisticated request matching strategies",
			Features: []string{
				"Regex pattern matching",
				"JSONPath expressions",
				"XPath for XML",
				"Custom JavaScript matchers",
				"Fuzzy matching",
				"Schema validation",
				"Multi-criteria matching",
			},
			Examples: []string{
				"Path: /api/users/[0-9]+",
				"Body: $.user.age > 18",
				"Header: Authorization matches 'Bearer .*'",
				"Query: limit between 1 and 100",
			},
		},
		"Stateful Mocking": {
			Name:        "Stateful Mock Interactions",
			Description: "Maintain state across multiple requests",
			Features: []string{
				"Session state management",
				"Request sequence tracking",
				"Data persistence",
				"State-based responses",
				"Cross-request validation",
				"Workflow simulation",
			},
			Examples: []string{
				"Track user login state",
				"Shopping cart persistence",
				"Multi-step form validation",
				"API rate limit enforcement",
			},
		},
		"Event Streaming": {
			Name:        "Real-time Event Streaming",
			Description: "Simulate real-time event streams and webhooks",
			Features: []string{
				"Server-sent events (SSE)",
				"WebSocket simulation",
				"Webhook delivery",
				"Event scheduling",
				"Message queuing",
				"Event filtering",
			},
			Examples: []string{
				"Real-time notifications",
				"Live data feeds",
				"Chat message streaming",
				"IoT sensor data",
			},
		},
		"API Versioning": {
			Name:        "API Version Simulation",
			Description: "Simulate different API versions and migration scenarios",
			Features: []string{
				"Version-specific responses",
				"Backward compatibility",
				"Deprecation warnings",
				"Migration guidance",
				"Feature flag simulation",
				"A/B testing support",
			},
			Examples: []string{
				"Header: Accept-Version: v2",
				"Path: /v1/users vs /v2/users",
				"Query: ?version=beta",
				"Feature: newUserFields=true",
			},
		},
	}
}

// MockServerFeature represents an advanced MockServer feature
type MockServerFeature struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
	Examples    []string `json:"examples"`
	Category    string   `json:"category,omitempty"`
	Complexity  string   `json:"complexity,omitempty"`
}

// ResponseTemplateExamples returns comprehensive response template examples
func ResponseTemplateExamples() map[string]TemplateExample {
	return map[string]TemplateExample{
		"User Management": {
			Name:        "User Management API Templates",
			Description: "Templates for user CRUD operations",
			Templates: map[string]string{
				"create_user": `{
  "id": "${uuid}",
  "name": "${request.body.name}",
  "email": "${string.toLowerCase(request.body.email)}",
  "createdAt": "${date.isoString}",
  "status": "active",
  "profile": {
    "firstName": "${string.split(request.body.name,' ').0}",
    "lastName": "${string.split(request.body.name,' ').1}",
    "initials": "${string.substring(request.body.name,0,2)}"
  },
  "metadata": {
    "requestId": "${request.headers.x-request-id}",
    "userAgent": "${request.headers.user-agent}",
    "ipAddress": "${request.headers.x-forwarded-for}"
  }
}`,
				"get_user": `{
  "id": "${request.pathParameters.id}",
  "name": "${random.person.firstName} ${random.person.lastName}",
  "email": "${string.toLowerCase(random.person.firstName)}.${string.toLowerCase(random.person.lastName)}@example.com",
  "createdAt": "${date.addDays(date.now,-30)}",
  "lastLoginAt": "${date.addHours(date.now,-2)}",
  "loginCount": "${math.randomInt(1,100)}",
  "isActive": "${random.boolean}",
  "preferences": {
    "theme": "${if(math.random > 0.5,'dark','light')}",
    "notifications": "${random.boolean}",
    "language": "${array.random(['en','es','fr','de'])}"
  }
}`,
			},
			UseCases: []string{
				"User registration flow",
				"Profile management",
				"User lookup and search",
				"Account deactivation",
			},
		},
		"E-commerce": {
			Name:        "E-commerce API Templates",
			Description: "Templates for e-commerce operations",
			Templates: map[string]string{
				"product_catalog": `{
  "products": [
    {
      "id": "product-${math.randomInt(1,1000)}",
      "name": "${random.product.name}",
      "price": "${math.round(math.randomFloat(10,1000),2)}",
      "currency": "USD",
      "inStock": "${random.boolean}",
      "rating": "${math.round(math.randomFloat(1,5),1)}"
    }
  ],
  "pagination": {
    "page": "${request.queryParameters.page || 1}",
    "limit": "${request.queryParameters.limit || 10}",
    "total": "${math.randomInt(100,1000)}"
  }
}`,
			},
			UseCases: []string{
				"Product browsing",
				"Shopping cart management",
				"Order processing",
			},
		},
	}
}

// TemplateExample represents a response template example
type TemplateExample struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Templates   map[string]string `json:"templates"`
	UseCases    []string          `json:"useCases"`
	Category    string            `json:"category,omitempty"`
	Complexity  string            `json:"complexity,omitempty"`
}
