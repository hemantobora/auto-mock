package builders

import (
	"encoding/json"
	"fmt"
	"regexp"
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

// RegexPattern represents a regex pattern with documentation
type RegexPattern struct {
	Pattern     string   `json:"pattern"`
	Description string   `json:"description"`
	Examples    []string `json:"examples"`
	Category    string   `json:"category,omitempty"`
	Difficulty  string   `json:"difficulty,omitempty"`
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

func ReviewGraphQLExpectation(exp *MockExpectation) error {
	fmt.Println("\n🔄 Review and Confirm")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

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
		reqHeaderCount = len(exp.HttpRequest.Headers)
	}

	// Work out body match mode & whether variables are present
	bodyMode, hasVars := summarizeGraphQLBody(exp)

	// Display summary
	fmt.Printf("\n📋 GraphQL Expectation Summary:\n")
	if exp.Description != "" {
		fmt.Printf("   Description: %s\n", exp.Description)
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

// GenerateResponseTemplate generates enhanced response templates
func GenerateResponseTemplate(expectation *MockExpectation) error {
	fmt.Println("\n🏷️  Enhanced Response Template Generation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show template options
	var templateType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select template type:",
		Options: []string{
			"smart - Auto-generate based on method & status",
			"rest-api - RESTful API response",
			"microservice - Microservice response",
			"error-response - Comprehensive error response",
			"minimal - Minimal response",
			"custom - Custom template",
		},
		Default: "smart - Auto-generate based on method & status",
	}, &templateType); err != nil {
		return err
	}

	templateType = strings.Split(templateType, " ")[0]

	// Generate template based on selection
	var template string
	switch templateType {
	case "smart":
		switch {
		case expectation.HttpResponse.StatusCode >= 200 && expectation.HttpResponse.StatusCode < 300:
			template = generateEnhancedSuccessTemplate(expectation.HttpRequest.Method)
		case expectation.HttpResponse.StatusCode >= 400 && expectation.HttpResponse.StatusCode < 500:
			template = generateEnhancedClientErrorTemplate(expectation.HttpResponse.StatusCode)
		case expectation.HttpResponse.StatusCode >= 500:
			template = generateEnhancedServerErrorTemplate()
		default:
			template = `{"message": "Response", "timestamp": "$!now_epoch"}`
		}
	case "rest-api":
		template = generateRESTAPITemplate()
	case "microservice":
		template = generateMicroserviceTemplate()
	case "error-response":
		template = generateComprehensiveErrorTemplate(expectation.HttpResponse.StatusCode)
	case "minimal":
		template = generateMinimalTemplate()
	case "custom":
		// Will ask for manual input below
		template = ""
	}

	if template != "" {
		fmt.Printf("💡 Generated %s template:\n%s\n\n", templateType, template)

		var useTemplate bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Use this generated template?",
			Default: true,
		}, &useTemplate); err != nil {
			return err
		}

		if useTemplate {
			expectation.HttpResponse.Body = template
			return nil
		}
	}

	// Manual entry for custom or if user declined generated template
	var manualJSON string
	if err := survey.AskOne(&survey.Multiline{
		Message: "Enter response JSON manually:",
		Help:    "Use $!template.variables for dynamic content",
	}, &manualJSON); err != nil {
		return err
	}
	expectation.HttpResponse.Body = manualJSON
	return nil
}

var gqlOpRegex = regexp.MustCompile(`(?m)^\s*(query|mutation|subscription)\s+([_A-Za-z][_0-9A-Za-z]*)`)

// ExtractGraphQLOperationName returns operation type (query/mutation/subscription)
// and name if present; both are lowercased.
func ExtractGraphQLOperationName(query string) (opType, opName string) {
	query = strings.TrimSpace(query)
	m := gqlOpRegex.FindStringSubmatch(query)
	if len(m) >= 3 {
		return strings.ToLower(m[1]), m[2]
	}
	// fallback if no explicit name (e.g. anonymous operation)
	// detect type keyword only
	for _, kw := range []string{"query", "mutation", "subscription"} {
		if strings.HasPrefix(strings.ToLower(query), kw) {
			return kw, ""
		}
	}
	return "", ""
}

func generateEnhancedSuccessTemplate(method string) string {
	switch method {
	case "POST":
		return `{"id": "$!uuid","message": "Resource created successfully","timestamp": "$!now_epoch","location": "/api/resource/$!uuid","requestId": "$!request.headers['x-request-id'][0]"}`
	case "PUT", "PATCH":
		return `{"id": "$!request.pathParameters['id'][0]","message": "Resource updated successfully","timestamp": "$!now_epoch","version": "$!rand_int_100","requestId": "$!request.headers['x-request-id'][0]"}`
	case "DELETE":
		return `{"message": "Resource deleted successfully","deletedId": "$!request.pathParameters['id'][0]","timestamp": "$!now_epoch","requestId": "$!request.headers['x-request-id'][0]"}`
	default: // GET
		return `{"id": "$!uuid","name": "Sample Resource","status": "active","createdAt": "$!now_epoch","updatedAt": "$!now_epoch","requestId": "$!request.headers['x-request-id'][0]","metadata": {"version": "1.0","source": "mock-server"}}`
	}
}

func generateEnhancedClientErrorTemplate(statusCode int) string {
	switch statusCode {
	case 400:
		return `{"error": {"code": "BAD_REQUEST","message": "Invalid request data provided","details": "Request validation failed","timestamp": "$!now_epoch","requestId": "$!request.headers['x-request-id'][0]","path": "$!request.path"},"validationErrors": [{"field": "example_field","message": "Field is required","code": "REQUIRED"}]}`
	case 401:
		return `{"error": {"code": "UNAUTHORIZED","message": "Authentication required","details": "Please provide valid authentication credentials","timestamp": "$!now_epoch","requestId": "$!request.headers['x-request-id'][0]"},"authMethods": ["Bearer Token", "API Key"]}`
	case 403:
		return `{"error": {"code": "FORBIDDEN","message": "Access denied","details": "Insufficient permissions for this resource","timestamp": "$!now_epoch","requestId": "$!request.headers['x-request-id'][0]","requiredPermissions": ["read:resource"]}}`
	case 404:
		return `{"error": {"code": "NOT_FOUND","message": "Resource not found","details": "The requested resource does not exist","timestamp": "$!now_epoch","requestId": "$!request.headers['x-request-id'][0]","path": "$!request.path","resourceId": "$!request.pathParameters['id'][0]"}}`
	case 429:
		return `{"error": {"code": "RATE_LIMITED","message": "Too many requests","details": "Rate limit exceeded","timestamp": "$!now_epoch","requestId": "$!request.headers['x-request-id'][0]","retryAfter": 60,"limit": 100,"remaining": 0}}`
	default:
		return `{"error": {"code": "CLIENT_ERROR","message": "Client error occurred","timestamp": "$!now_epoch","requestId": "$!request.headers['x-request-id'][0]"}}`
	}
}

func generateEnhancedServerErrorTemplate() string {
	return `{"error": {"code": "INTERNAL_SERVER_ERROR","message": "An internal server error occurred","details": "Please try again later or contact support","timestamp": "$!now_epoch","requestId": "$!request.headers['x-request-id'][0]","traceId": "$!uuid","supportContact": "support@example.com"}}`
}

func generateRESTAPITemplate() string {
	return `{"data": {"id": "$!uuid","type": "resource","attributes": {"name": "Sample Resource","status": "active","createdAt": "$!now_epoch","updatedAt": "$!now_epoch"},"relationships": {"owner": {"data": {"id": "$!rand_bytes_64", "type": "user"}}}},"meta": {"requestId": "$!request.headers['x-request-id'][0]","version": "1.0","timestamp": "$!now_epoch"}}`
}

func generateMicroserviceTemplate() string {
	return `{"serviceInfo": {"name": "mock-service","version": "1.0.0","environment": "mock","region": "us-east-1"},"data": {"id": "$!uuid","status": "success","timestamp": "$!now_epoch","processingTimeMs": "$!rand_int_100"},"metadata": {"requestId": "$!request.headers['x-request-id'][0]","correlationId": "$!uuid","traceId": "$!uuid","spanId": "$!rand_bytes_64"}}`
}

func generateComprehensiveErrorTemplate(statusCode int) string {
	return `{"error": {"code": "ERROR_CODE","message": "Human-readable error message","details": "Detailed error description","timestamp": "$!now_epoch","requestId": "$!request.headers['x-request-id'][0]","traceId": "$!uuid","path": "$!request.path","method": "$!request.method","statusCode": ` + fmt.Sprintf("%d", statusCode) + `},"context": {"userAgent": "$!request.headers['user-agent'][0]}","clientIp": "$!request.headers['x-forwarded-for'][0]","timestamp": "$!now_epoch"},"support": {"documentation": "https://docs.example.com/errors","contact": "support@example.com","statusPage": "https://status.example.com"}}`
}

func generateMinimalTemplate() string {
	return `{"success": true, "timestamp": "$!now_epoch"}`
}
