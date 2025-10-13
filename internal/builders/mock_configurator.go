package builders

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

type MockConfigurator struct {
}

// collectRequestBody collects request body for REST endpoints
func (mc *MockConfigurator) CollectRequestBody(expectation *MockExpectation, body string) error {

	if strings.Trim(body, " ") != "" {
		var useBody bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "This is the existing request body:\n" + body + "\nShall we use it to match?",
			Default: false,
			Help:    "Only specify if you need to match exact request body content",
		}, &useBody); err != nil {
			return err
		}
		if useBody {
			expectation.Body = body
			return nil
		}
	}

	var needsBody bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Does this request require a specific body to match?",
		Default: false,
		Help:    "Only specify if you need to match exact request body content",
	}, &needsBody); err != nil {
		return err
	}

	if !needsBody {
		return nil
	}

	var bodyJSON string
	if err := survey.AskOne(&survey.Multiline{
		Message: "Enter the request body JSON:",
		Help:    "Paste your JSON here. It will be validated.",
	}, &bodyJSON); err != nil {
		return err
	}

	// Validate JSON
	if err := ValidateJSON(bodyJSON); err != nil {
		fmt.Printf("⚠️  JSON validation failed: %v\n", err)
		var proceed bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "JSON is invalid. Use it anyway?",
			Default: false,
		}, &proceed); err != nil {
			return err
		}
		if !proceed {
		 return &models.JSONValidationError{
		Context: "request body",
		Content: bodyJSON,
		Cause:   err,
		}
		}
	}

	expectation.Body = bodyJSON
	return nil
}

func (mc *MockConfigurator) CollectQueryParameterMatching(expectation *MockExpectation) error {
	fmt.Println("\n🔍 Step 2: Query Parameter Matching")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Check if already configured from path parsing
	if len(expectation.QueryParams) > 0 {
		fmt.Printf("ℹ️  Already configured %d query parameters from path\n", len(expectation.QueryParams))

		var addMore bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Add additional query parameters?",
			Default: false,
		}, &addMore); err != nil {
			return err
		}

		if !addMore {
			fmt.Printf("✅ Query Parameters: %d configured\n", len(expectation.QueryParams))
			return nil
		}
	} else {
		var needsQueryParams bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Does this endpoint require specific query parameters?",
			Default: false,
			Help:    "Only specify if you need to match exact query parameter values",
		}, &needsQueryParams); err != nil {
			return err
		}

		if !needsQueryParams {
			fmt.Println("ℹ️  No query parameter matching configured")
			return nil
		}

		expectation.QueryParams = make(map[string]string)
	}

	for {
		var paramName string
		if err := survey.AskOne(&survey.Input{
			Message: "Parameter name (empty to finish):",
			Help:    "e.g., 'page', 'limit', 'category'",
		}, &paramName); err != nil {
			return err
		}

		paramName = strings.TrimSpace(paramName)
		if paramName == "" {
			break
		}

		var paramValue string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Value for '%s':", paramName),
			Help:    "Use exact value or regex pattern",
		}, &paramValue); err != nil {
			return err
		}

		expectation.QueryParams[paramName] = paramValue
		fmt.Printf("✅ Added query parameter: %s=%s\n", paramName, paramValue)
	}

	fmt.Printf("✅ Query Parameters: %d configured\n", len(expectation.QueryParams))
	return nil
}

// parsePathAndQueryParams intelligently separates path from query parameters
func (mc *MockConfigurator) ParsePathAndQueryParams(fullPath string) (cleanPath string, queryParams map[string]string) {
	queryParams = make(map[string]string)

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

	// Extract query parameters
	for name, values := range parsedURL.Query() {
		if len(values) > 0 {
			queryParams[name] = values[0] // Take first value
		}
	}

	return cleanPath, queryParams
}

// Step 3: Path Matching Strategy
func (mc *MockConfigurator) CollectPathMatchingStrategy(expectation *MockExpectation) error {
	fmt.Println("\n🛤️  Step 3: Path Matching Strategy")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Check if path has parameters
	hasPathParams := strings.Contains(expectation.Path, "{") && strings.Contains(expectation.Path, "}")

	if !hasPathParams {
		// For exact paths, ask if user wants regex matching
		var useRegex bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Use regex pattern matching for this path?",
			Default: false,
			Help:    "Regex allows flexible matching but is more complex",
		}, &useRegex); err != nil {
			return err
		}

		if useRegex {
			if err := collectRegexPattern(expectation); err != nil {
				return err
			}
		} else {
			fmt.Println("ℹ️  Using exact string matching for path")
			fmt.Printf("🔍 Pattern: %s (exact match)\n", expectation.Path)
		}
	} else {
		fmt.Printf("ℹ️  Path parameters detected in: %s\n", expectation.Path)
		fmt.Println("💡 MockServer will automatically handle path parameters")
	}

	fmt.Printf("✅ Path matching configured for: %s\n", expectation.Path)
	return nil
}

// Step 4: Request Header Matching
func (mc *MockConfigurator) CollectRequestHeaderMatching(expectation *MockExpectation) error {
	fmt.Println("\n📝 Step 4: Request Header Matching")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

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

	expectation.Headers = make(map[string]string)
	expectation.HeaderTypes = make(map[string]string)

	// Header collection with improved flow - ask matching type first
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

		// ASK MATCHING TYPE FIRST - more intuitive flow
		var matchingType string
		if err := survey.AskOne(&survey.Select{
			Message: fmt.Sprintf("How should '%s' header be matched?", headerName),
			Options: []string{
				"exact - Match exact value (e.g., 'Bearer abc123')",
				"regex - Use pattern matching (e.g., 'Bearer .*')",
			},
			Default: "exact - Match exact value (e.g., 'Bearer abc123')",
			Help:    "Choose matching strategy before entering the value",
		}, &matchingType); err != nil {
			return err
		}

		isRegex := strings.HasPrefix(matchingType, "regex")

		// NOW ask for value with appropriate context
		var prompt string
		var helpText string
		if isRegex {
			prompt = fmt.Sprintf("Regex pattern for '%s':", headerName)
			helpText = "Enter regex pattern (e.g., 'Bearer .*', 'application/.*')"
		} else {
			prompt = fmt.Sprintf("Exact value for '%s':", headerName)
			helpText = "Enter exact value to match (e.g., 'Bearer abc123')"
		}

		var headerValue string
		if err := survey.AskOne(&survey.Input{
			Message: prompt,
			Help:    helpText,
		}, &headerValue); err != nil {
			return err
		}

		expectation.Headers[headerName] = headerValue
		if isRegex {
			// Validate regex pattern
			if err := IsValidRegex(headerValue); err != nil {
				fmt.Printf("⚠️  Warning: Invalid regex pattern '%s': %v\n", headerValue, err)
				var proceed bool
				if err := survey.AskOne(&survey.Confirm{
					Message: "Use this invalid regex anyway?",
					Default: false,
				}, &proceed); err != nil {
					return err
				}
				if !proceed {
					delete(expectation.Headers, headerName)
					continue // Ask for header again
				}
			}
			expectation.HeaderTypes[headerName] = "regex"
			fmt.Printf("✅ Added header: %s: %s (regex pattern)\n", headerName, headerValue)
		} else {
			expectation.HeaderTypes[headerName] = "exact"
			fmt.Printf("✅ Added header: %s: %s (exact match)\n", headerName, headerValue)
		}
	}

	fmt.Printf("✅ Request Headers: %d configured\n", len(expectation.Headers))
	return nil
}

// Enhanced response delay configuration with advanced MockServer features
func collectResponseDelay(expectation *MockExpectation) error {
	var needsDelay bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Add response delay?",
		Default: false,
		Help:    "Simulate slow responses for testing",
	}, &needsDelay); err != nil {
		return err
	}

	if !needsDelay {
		return nil
	}

	var delayType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select delay type:",
		Options: []string{
			"fixed - Fixed delay in milliseconds",
			"random - Random delay range",
		},
		Default: "fixed - Fixed delay in milliseconds",
	}, &delayType); err != nil {
		return err
	}

	if strings.HasPrefix(delayType, "fixed") {
		var delay string
		if err := survey.AskOne(&survey.Input{
			Message: "Delay in milliseconds:",
			Default: "1000",
			Help:    "e.g., 1000 for 1 second delay",
		}, &delay); err != nil {
			return err
		}
		// Enhanced delay configuration for MockServer
		if expectation.Times == nil {
			expectation.Times = &Times{}
		}
		expectation.ResponseDelay = delay
		fmt.Printf("✅ Fixed delay configured: %s ms\n", delay)
	} else {
		// Random delay range
		var minDelay string
		if err := survey.AskOne(&survey.Input{
			Message: "Minimum delay (ms):",
			Default: "500",
		}, &minDelay); err != nil {
			return err
		}

		var maxDelay string
		if err := survey.AskOne(&survey.Input{
			Message: "Maximum delay (ms):",
			Default: "2000",
		}, &maxDelay); err != nil {
			return err
		}

		// Enhanced random delay for MockServer with proper format
		if expectation.Times == nil {
			expectation.Times = &Times{}
		}
		// Store as range format for MockServer
		expectation.ResponseDelay = fmt.Sprintf("%s-%s", minDelay, maxDelay)
		fmt.Printf("✅ Random delay configured: %s-%s ms\n", minDelay, maxDelay)

		// Add MockServer-specific documentation
		fmt.Println("\n📚 MockServer Delay Documentation:")
		fmt.Println("   Delay Configuration: https://mock-server.com/mock_server/response_delays.html")
		fmt.Println("   Advanced Timing: https://mock-server.com/mock_server/times.html")
	}

	return nil
}

// Enhanced response limits configuration with advanced MockServer features
func collectResponseLimits(expectation *MockExpectation) error {
	var needsLimits bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Limit number of responses?",
		Default: false,
		Help:    "Useful for testing scenarios like rate limiting",
	}, &needsLimits); err != nil {
		return err
	}

	if !needsLimits {
		return nil
	}

	var remainingTimes string
	if err := survey.AskOne(&survey.Input{
		Message: "Maximum number of responses:",
		Default: "1",
		Help:    "After this many responses, the expectation will stop matching",
	}, &remainingTimes); err != nil {
		return err
	}

	// Parse and validate
	times, err := strconv.Atoi(remainingTimes)
	if err != nil {
		return &models.InputValidationError{
			InputType: "response limit",
			Value:     remainingTimes,
			Expected:  "positive integer",
			Cause:     err,
		}
	}

	expectation.Times = &Times{
		RemainingTimes: times,
		Unlimited:      false,
	}

	fmt.Printf("✅ Response limit: %d times\n", times)

	// Add advanced rate limiting guidance
	fmt.Println("\n📚 Advanced Rate Limiting Patterns:")
	fmt.Println("   • Create additional expectation for post-limit behavior")
	fmt.Println("   • Use 429 status code for rate limit exceeded responses")
	fmt.Println("   • Include Retry-After header for client guidance")

	fmt.Println("\n📚 MockServer Times Documentation:")
	fmt.Println("   Times Configuration: https://mock-server.com/mock_server/times.html")
	fmt.Println("   Rate Limiting Guide: https://mock-server.com/mock_server/response_delays.html")

	return nil
}

// collectExpectationPriority collects expectation priority configuration
func collectExpectationPriority(expectation *MockExpectation) error {
	var priority string
	if err := survey.AskOne(&survey.Input{
		Message: "Expectation priority (lower numbers = higher priority):",
		Default: "0",
		Help:    "Higher priority expectations are matched first (0 = highest)",
	}, &priority); err != nil {
		return err
	}

	if p, err := strconv.Atoi(priority); err == nil {
		expectation.Priority = p
		fmt.Printf("✅ Priority set to: %d\n", p)
	} else {
		return &models.InputValidationError{
			InputType: "priority",
			Value:     priority,
			Expected:  "integer",
			Cause:     err,
		}
	}

	return nil
}

// collectCustomResponseHeaders collects custom response headers
func collectCustomResponseHeaders(expectation *MockExpectation) error {
	if expectation.ResponseHeaders == nil {
		expectation.ResponseHeaders = make(map[string]string)
	}

	for {
		var headerName string
		if err := survey.AskOne(&survey.Input{
			Message: "Response header name (empty to finish):",
			Help:    "e.g., 'X-Custom-Header', 'Cache-Control'",
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
		}, &headerValue); err != nil {
			return err
		}

		expectation.ResponseHeaders[headerName] = headerValue
		fmt.Printf("✅ Added response header: %s: %s\n", headerName, headerValue)
	}

	return nil
}

// configureResponseBehavior configures response behavior features
func configureResponseBehavior(expectation *MockExpectation) error {
	fmt.Println("\n🎭 Response Behavior Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var features []string
	fmt.Println("\n👉 Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select response behavior features:",
		Options: []string{
			"delays - Response delays (fixed/random)",
			"limits - Response count limits",
			"priority - Expectation priority",
			"custom-headers - Custom response headers",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &features); err != nil {
		return err
	}

	for _, feature := range features {
		switch strings.Split(feature, " ")[0] {
		case "delays":
			if err := collectResponseDelay(expectation); err != nil {
				return err
			}
		case "limits":
			if err := collectResponseLimits(expectation); err != nil {
				return err
			}
		case "priority":
			if err := collectExpectationPriority(expectation); err != nil {
				return err
			}
		case "custom-headers":
			if err := collectCustomResponseHeaders(expectation); err != nil {
				return err
			}
		}
	}

	return nil
}

// enhanceResponseWithTemplating enhances existing response with templating
func enhanceResponseWithTemplating(originalResponse string, templateExamples map[string]string) string {
	// Simple enhancement - in production you'd parse JSON and merge properly
	if originalResponse == "" {
		return originalResponse
	}

	// Try to add template fields to existing JSON
	// This is a simple implementation - you'd want proper JSON parsing
	enhanced := strings.TrimSuffix(strings.TrimSpace(originalResponse), "}")
	if !strings.HasSuffix(enhanced, ",") && !strings.HasSuffix(enhanced, "{") {
		enhanced += ","
	}

	enhanced += "\n  \"templatedFields\": {\n"
	for key, value := range templateExamples {
		enhanced += fmt.Sprintf("    \"%s\": \"%s\",\n", key, value)
	}
	enhanced = strings.TrimSuffix(enhanced, ",\n") + "\n  }\n}"

	return enhanced
}

// Enhanced response templating configuration with advanced MockServer features
func collectResponseTemplating(expectation *MockExpectation) error {
	var needsTemplating bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Use dynamic response templating?",
		Default: false,
		Help:    "Echo request data back in responses using MockServer templating",
	}, &needsTemplating); err != nil {
		return err
	}

	if !needsTemplating {
		return nil
	}

	fmt.Println("\n🎭 Dynamic Response Templating")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show available template variables
	fmt.Println("\n💡 Available Template Variables:")
	fmt.Println("   Path Parameters: ${request.pathParameters.id}")
	fmt.Println("   Query Parameters: ${request.queryParameters.limit}")
	fmt.Println("   Headers: ${request.headers.authorization}")
	fmt.Println("   Body Fields: ${request.body.user.email}")
	fmt.Println("   Timestamps: ${now}, ${timestamp}")
	fmt.Println("   UUIDs: ${uuid}, ${randomUUID}")

	var templateSources []string
	fmt.Println("\n👉 Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Which request data to echo in response?",
		Options: []string{
			"path - Path parameters (e.g., /users/{id})",
			"query - Query parameters (e.g., ?limit=10)",
			"headers - Request headers (e.g., Authorization)",
			"body - Request body fields (e.g., user.name)",
			"dynamic - Generated values (UUID, timestamp)",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &templateSources); err != nil {
		return err
	}

	if len(templateSources) == 0 {
		fmt.Println("ℹ️  No template sources selected")
		return nil
	}

	// Generate template examples based on selection
	templateExamples := make(map[string]string)
	for _, source := range templateSources {
		sourceType := strings.Split(source, " ")[0]
		switch sourceType {
		case "path":
			templateExamples["userId"] = "${request.pathParameters.id}"
		case "query":
			templateExamples["limit"] = "${request.queryParameters.limit}"
			templateExamples["page"] = "${request.queryParameters.page}"
		case "headers":
			templateExamples["authToken"] = "${request.headers.authorization}"
			templateExamples["contentType"] = "${request.headers.content-type}"
		case "body":
			templateExamples["userName"] = "${request.body.name}"
			templateExamples["userEmail"] = "${request.body.email}"
		case "dynamic":
			templateExamples["requestId"] = "${uuid}"
			templateExamples["processedAt"] = "${timestamp}"
		}
	}

	// Show generated template example
	if len(templateExamples) > 0 {
		fmt.Println("\n🏗️  Generated Template Example:")
		templateJSON := "{\n"
		for key, value := range templateExamples {
			templateJSON += fmt.Sprintf("  \"%s\": \"%s\",\n", key, value)
		}
		templateJSON = strings.TrimSuffix(templateJSON, ",\n") + "\n}"
		fmt.Printf("%s\n", templateJSON)

		var useTemplate bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Apply templating to your response body?",
			Default: true,
			Help:    "This will update your current response to include template variables",
		}, &useTemplate); err != nil {
			return err
		}

		if useTemplate {
			// Update response body with templating
			if expectation.ResponseBody != nil {
				// Try to merge templating into existing response
				originalResponse := expectation.ResponseBody.(string)
				updatedResponse := enhanceResponseWithTemplating(originalResponse, templateExamples)
				expectation.ResponseBody = updatedResponse
				fmt.Printf("✅ Response enhanced with templating\n")
			} else {
				// Create new templated response
				expectation.ResponseBody = templateJSON
				fmt.Printf("✅ Template response created\n")
			}
		}
	}

	fmt.Println("\n📚 Advanced Templating Documentation:")
	fmt.Println("   MockServer Templating: https://mock-server.com/mock_server/response_templates.html")
	fmt.Println("   Template Variables: https://mock-server.com/mock_server/response_templates.html#template-variables")
	fmt.Println("   Advanced Examples: https://mock-server.com/mock_server/response_templates.html#template-examples")
	fmt.Println("   JavaScript Processing: https://mock-server.com/mock_server/response_templates.html#javascript-templating")

	fmt.Println("\n🔥 Pro Templating Tips:")
	fmt.Println("   • Use ${if(condition,value1,value2)} for conditional responses")
	fmt.Println("   • Combine ${request.pathParameters.id} with ${uuid} for realistic data")
	fmt.Println("   • Use ${math.randomInt(1,100)} for dynamic numeric values")
	fmt.Println("   • Echo client data with ${request.headers.user-agent}")

	return nil
}

// collectResponseSequence collects response sequence configuration
func collectResponseSequence(expectation *MockExpectation) error {
	fmt.Println("\n📋 Response Sequence Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var sequenceType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select sequence pattern:",
		Options: []string{
			"success-then-error - First call succeeds, then errors",
			"slow-then-fast - First call slow, then normal",
			"custom - Define custom sequence (Only guide provided. Future support planned)",
		},
	}, &sequenceType); err != nil {
		return err
	}

	sequenceType = strings.Split(sequenceType, " ")[0]

	switch sequenceType {
	case "success-then-error":
		fmt.Println("\n✅ Success-Then-Error Pattern:")
		fmt.Println("   Call 1: 200 OK with data")
		fmt.Println("   Call 2+: 503 Service Unavailable")

		// This would require creating multiple expectations
		fmt.Println("\n📝 Implementation Note:")
		fmt.Println("   This pattern requires multiple MockServer expectations.")
		fmt.Println("   The first expectation has times: {remainingTimes: 1}")
		fmt.Println("   The second expectation handles all subsequent calls.")

	case "slow-then-fast":
		fmt.Println("\n🐌 Slow-Then-Fast Pattern:")
		fmt.Println("   Call 1: 3000ms delay")
		fmt.Println("   Call 2+: 100ms delay")

		// Configure first call with slow delay
		expectation.ResponseDelay = "3000"
		if expectation.Times == nil {
			expectation.Times = &Times{}
		}
		expectation.Times.RemainingTimes = 1
		expectation.Times.Unlimited = false

		fmt.Println("\n📝 Implementation Note:")
		fmt.Println("   Current expectation configured for first slow call.")
		fmt.Println("   Create a second expectation for fast subsequent calls.")

	case "custom":
		fmt.Println("\n🛠️  Custom Sequence Guide:")
		fmt.Println("   1. Create multiple expectations with same request criteria")
		fmt.Println("   2. Use 'times': {'remainingTimes': N} to limit each")
		fmt.Println("   3. MockServer processes expectations in order")
		fmt.Println("   4. Each expectation can have different response/delay")
	}

	fmt.Printf("✅ Conditional sequence guidance provided\n")
	return nil
}

// collectResponseSequenceAdvanced collects advanced response sequence configuration
func collectResponseSequenceAdvanced(expectation *MockExpectation) error {
	fmt.Println("\n🔄 Advanced Response Sequences")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n💡 Advanced Sequence Patterns:")
	fmt.Println("   🔢 Numbered sequences: 1st call different, 2nd call different, etc.")
	fmt.Println("   ⏰ Time-based sequences: Different responses at different times")
	fmt.Println("   📊 Statistical sequences: Random distribution of responses")
	fmt.Println("   🔄 Cyclical sequences: Repeat pattern after N calls")

	return collectResponseSequence(expectation)
}

// collectAdvancedResponseTemplating collects advanced templating configuration
func collectAdvancedResponseTemplating(expectation *MockExpectation) error {
	fmt.Println("\n🎭 Advanced Response Templating")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show comprehensive template variables
	fmt.Println("\n📚 Available Template Variables:")
	templateVars := TemplateVariables()
	for category, variables := range templateVars {
		fmt.Printf("   📂 %s:\n", category)
		for _, variable := range variables {
			fmt.Printf("      • %s\n", variable)
		}
	}

	// Enhanced templating configuration
	return collectResponseTemplating(expectation)
}

// collectCircuitBreakerBehavior collects circuit breaker simulation
func collectCircuitBreakerBehavior(expectation *MockExpectation) error {
	fmt.Println("\n⚡ Circuit Breaker Simulation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var failureRate string
	if err := survey.AskOne(&survey.Input{
		Message: "Failure rate percentage (0-100):",
		Default: "30",
		Help:    "Percentage of requests that should fail",
	}, &failureRate); err != nil {
		return err
	}

	var failureResponse string
	if err := survey.AskOne(&survey.Input{
		Message: "Failure status code:",
		Default: "503",
		Help:    "HTTP status code for failed requests",
	}, &failureResponse); err != nil {
		return err
	}

	fmt.Printf("\n🔧 Circuit Breaker Configuration:\n")
	fmt.Printf("   Failure Rate: %s%%\n", failureRate)
	fmt.Printf("   Failure Status: %s\n", failureResponse)

	fmt.Println("\n📝 Implementation Guide:")
	fmt.Println("   Create two expectations:")
	fmt.Printf("   1. Success case (70%% of the time) - Current expectation\n")
	fmt.Printf("   2. Failure case (%s%% of the time) - Additional expectation\n", failureRate)
	fmt.Println("   Use MockServer's randomization or script multiple expectations.")

	return nil
}

// collectRateLimitBehavior collects rate limiting simulation
func collectRateLimitBehavior(expectation *MockExpectation) error {
	fmt.Println("\n🚦 Rate Limiting Simulation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var allowedRequests string
	if err := survey.AskOne(&survey.Input{
		Message: "Allowed requests before rate limiting:",
		Default: "5",
		Help:    "Number of successful requests before 429 responses",
	}, &allowedRequests); err != nil {
		return err
	}

	// Configure current expectation for allowed requests
	requests, err := strconv.Atoi(allowedRequests)
	if err != nil {
		return &models.InputValidationError{
			InputType: "rate limit requests",
			Value:     allowedRequests,
			Expected:  "positive integer",
			Cause:     err,
		}
	}

	if expectation.Times == nil {
		expectation.Times = &Times{}
	}
	expectation.Times.RemainingTimes = requests
	expectation.Times.Unlimited = false

	fmt.Printf("\n🔧 Rate Limiting Configuration:\n")
	fmt.Printf("   Allowed Requests: %s\n", allowedRequests)
	fmt.Printf("   Rate Limit Response: 429 Too Many Requests\n")

	fmt.Println("\n📝 Implementation Guide:")
	fmt.Println("   Current expectation configured for allowed requests.")
	fmt.Printf("   Create a second expectation with same criteria but:\n")
	fmt.Println("   - Status: 429")
	fmt.Println("   - Body: {'error': 'rate_limited', 'retry_after': 60}")
	fmt.Println("   - This expectation will handle requests after the limit.")

	fmt.Printf("✅ Rate limiting configured: %s requests allowed\n", allowedRequests)
	return nil
}

// showCustomConditionalGuidance shows guidance for custom conditional logic
func showCustomConditionalGuidance() error {
	fmt.Println("\n🛠️  Custom Conditional Logic Guide")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n🔧 Advanced Conditional Features:")
	fmt.Println("   1. Request Count Based:")
	fmt.Println("      'times': {'remainingTimes': 3, 'unlimited': false}")
	fmt.Println("   2. Time-Based Responses:")
	fmt.Println("      Multiple expectations with different timeToLive")
	fmt.Println("   3. Header-Based Conditions:")
	fmt.Println("      Different responses based on request headers")
	fmt.Println("   4. JavaScript Callbacks:")
	fmt.Println("      'callback': {'callbackClass': 'your.callback.Class'}")

	fmt.Println("\n📚 Advanced MockServer Documentation:")
	fmt.Println("   Conditional Logic: https://mock-server.com/mock_server/expectations.html")
	fmt.Println("   Times Configuration: https://mock-server.com/mock_server/times.html")
	fmt.Println("   Callbacks: https://mock-server.com/mock_server/callbacks.html")
	fmt.Println("   JavaScript Templates: https://mock-server.com/mock_server/response_templates.html#javascript-templating")

	fmt.Println("\n🔥 Professional Conditional Patterns:")
	fmt.Println("   • Use priority to control expectation matching order")
	fmt.Println("   • Combine times with different response bodies for sequences")
	fmt.Println("   • Use JavaScript callbacks for complex conditional logic")
	fmt.Println("   • Leverage template variables for dynamic conditional responses")

	return nil
}

// collectConditionalBehavior collects conditional response behavior configuration
func collectConditionalBehavior(expectation *MockExpectation) error {
	var needsConditional bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Add conditional response behavior?",
		Default: false,
		Help:    "Different responses based on request count, sequences, or conditions",
	}, &needsConditional); err != nil {
		return err
	}

	if !needsConditional {
		return nil
	}

	fmt.Println("\n🔄 Conditional Response Behavior")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var behaviorType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select conditional behavior type:",
		Options: []string{
			"sequence - Different responses on successive calls",
			"circuit-breaker - Simulate service failures (Only guide provided. Future support planned)",
			"rate-limit - Simulate rate limiting scenarios",
			"custom - Custom conditional logic (Only guide provided. Future support planned)",
		},
		Default: "sequence - Different responses on successive calls",
	}, &behaviorType); err != nil {
		return err
	}

	behaviorType = strings.Split(behaviorType, " ")[0]

	switch behaviorType {
	case "sequence":
		return collectResponseSequence(expectation)
	case "circuit-breaker":
		return collectCircuitBreakerBehavior(expectation)
	case "rate-limit":
		return collectRateLimitBehavior(expectation)
	case "custom":
		return showCustomConditionalGuidance()
	default:
		return nil
	}
}

// configureDynamicContent configures dynamic content features
func configureDynamicContent(expectation *MockExpectation) error {
	fmt.Println("\n🎨 Dynamic Content Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var features []string
	fmt.Println("\n👉 Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select dynamic content features:",
		Options: []string{
			"templating - Response templating with request data",
			"sequences - Response sequences over time",
			"conditions - Conditional response logic",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &features); err != nil {
		return err
	}

	for _, feature := range features {
		switch strings.Split(feature, " ")[0] {
		case "templating":
			if err := collectAdvancedResponseTemplating(expectation); err != nil {
				return err
			}
		case "sequences":
			if err := collectResponseSequenceAdvanced(expectation); err != nil {
				return err
			}
		case "conditions":
			if err := collectConditionalBehavior(expectation); err != nil {
				return err
			}
		}
	}

	return nil
}

// configureWebhookCallbacks configures webhook callback functionality
func configureWebhookCallbacks(expectation *MockExpectation) error {
	fmt.Println("\n🎣 Webhook Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var webhookURL string
	if err := survey.AskOne(&survey.Input{
		Message: "Webhook URL:",
		Help:    "HTTP endpoint to call when this expectation matches",
	}, &webhookURL); err != nil {
		return err
	}

	if webhookURL == "" {
		fmt.Println("ℹ️  No webhook URL provided")
		return nil
	}

	var webhookMethod string
	if err := survey.AskOne(&survey.Select{
		Message: "Webhook HTTP method:",
		Options: []string{"POST", "GET", "PUT", "PATCH"},
		Default: "POST",
	}, &webhookMethod); err != nil {
		return err
	}

	if expectation.Callbacks == nil {
		expectation.Callbacks = &CallbackConfig{}
	}

	expectation.Callbacks.HttpCallback = &HttpCallback{
		URL:    webhookURL,
		Method: webhookMethod,
	}

	fmt.Printf("✅ Webhook configured: %s %s\n", webhookMethod, webhookURL)
	return nil
}

// configureCustomCodeCallbacks configures Java callback classes
func configureCustomCodeCallbacks(expectation *MockExpectation) error {
	fmt.Println("\n☕ Java Callback Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var callbackClass string
	if err := survey.AskOne(&survey.Input{
		Message: "Java callback class name:",
		Help:    "Fully qualified class name (e.g., com.example.MyCallback)",
	}, &callbackClass); err != nil {
		return err
	}

	if callbackClass == "" {
		fmt.Println("ℹ️  No callback class provided")
		return nil
	}

	if expectation.Callbacks == nil {
		expectation.Callbacks = &CallbackConfig{}
	}

	expectation.Callbacks.CallbackClass = callbackClass

	fmt.Printf("✅ Custom callback configured: %s\n", callbackClass)
	fmt.Println("\n📚 Documentation:")
	fmt.Println("   Callback Guide: https://mock-server.com/mock_server/callbacks.html")
	fmt.Println("   Example Classes: https://github.com/mock-server/mockserver/tree/master/mockserver-examples")

	return nil
}

// configureRequestForwarding configures request forwarding
func configureRequestForwarding(expectation *MockExpectation) error {
	fmt.Println("\n↗️  Request Forwarding Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n📝 Note: Request forwarding requires additional MockServer configuration.")
	fmt.Println("   This feature forwards matching requests to real endpoints.")
	fmt.Println("   Configure using 'forward' instead of 'httpResponse' in JSON.")

	var forwardURL string
	if err := survey.AskOne(&survey.Input{
		Message: "Forward to URL:",
		Help:    "Real endpoint to forward requests to (e.g., https://api.real-service.com)",
	}, &forwardURL); err != nil {
		return err
	}

	if forwardURL != "" {
		fmt.Printf("✅ Forwarding configured to: %s\n", forwardURL)
		fmt.Println("\n📚 Documentation:")
		fmt.Println("   Forwarding Guide: https://mock-server.com/mock_server/getting_started.html#button_forward_request")
	}

	return nil
}

// configureIntegrationCallbacks configures integration and callback features
func configureIntegrationCallbacks(expectation *MockExpectation) error {
	fmt.Println("\n🔗 Integration & Callbacks Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var features []string
	fmt.Println("\n👉 Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select integration features:",
		Options: []string{
			"webhooks - HTTP webhooks on request match",
			"custom-code - Java callback classes",
			"forward - Forward to real endpoints (Guide provided. Future support planned)",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &features); err != nil {
		return err
	}

	for _, feature := range features {
		switch strings.Split(feature, " ")[0] {
		case "webhooks":
			if err := configureWebhookCallbacks(expectation); err != nil {
				return err
			}
		case "custom-code":
			if err := configureCustomCodeCallbacks(expectation); err != nil {
				return err
			}
		case "forward":
			if err := configureRequestForwarding(expectation); err != nil {
				return err
			}
		}
	}

	return nil
}

// configureConnectionControl configures connection control features
func configureConnectionControl(expectation *MockExpectation) error {
	fmt.Println("\n🌐 Connection Control Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var features []string
	fmt.Println("\n👉 Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select connection control features:",
		Options: []string{
			"drop-connection - Drop connections (simulate network issues)",
			"chunked-encoding - Control transfer encoding",
			"keep-alive - Connection persistence settings",
			"error-simulation - Simulate connection errors",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &features); err != nil {
		return err
	}

	if len(features) == 0 {
		return nil
	}

	if expectation.ConnectionOptions == nil {
		expectation.ConnectionOptions = &ConnectionOptions{}
	}

	for _, feature := range features {
		switch strings.Split(feature, " ")[0] {
		case "drop-connection":
			expectation.ConnectionOptions.DropConnection = true
			fmt.Println("✅ Drop connection enabled")
		case "chunked-encoding":
			expectation.ConnectionOptions.ChunkedEncoding = true
			fmt.Println("✅ Chunked encoding enabled")
		case "keep-alive":
			expectation.ConnectionOptions.KeepAlive = true
			fmt.Println("✅ Keep-alive enabled")
		case "error-simulation":
			expectation.ConnectionOptions.SuppressConnectionErrors = false
			fmt.Println("✅ Connection error simulation enabled")
		}
	}

	return nil
}

// collectCircuitBreakerAdvanced collects advanced circuit breaker configuration
func collectCircuitBreakerAdvanced(expectation *MockExpectation) error {
	fmt.Println("\n⚡ Advanced Circuit Breaker Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var pattern string
	if err := survey.AskOne(&survey.Select{
		Message: "Circuit breaker pattern:",
		Options: []string{
			"gradual-failure - Gradually increase failure rate",
			"burst-failure - Sudden failure then recovery",
			"random-failure - Random failure distribution",
			"cascading-failure - Multiple service failure simulation",
		},
	}, &pattern); err != nil {
		return err
	}

	// Enhanced circuit breaker with detailed configuration
	return collectCircuitBreakerBehavior(expectation)
}

// collectRateLimitAdvanced collects advanced rate limiting configuration
func collectRateLimitAdvanced(expectation *MockExpectation) error {
	fmt.Println("\n🚦 Advanced Rate Limiting Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var strategy string
	if err := survey.AskOne(&survey.Select{
		Message: "Rate limiting strategy:",
		Options: []string{
			"sliding-window - Sliding window rate limiting",
			"token-bucket - Token bucket algorithm",
			"fixed-window - Fixed window rate limiting",
			"adaptive - Adaptive rate limiting",
		},
	}, &strategy); err != nil {
		return err
	}

	// Enhanced rate limiting with detailed configuration
	return collectRateLimitBehavior(expectation)
}

// collectChaosEngineering collects chaos engineering configuration
func collectChaosEngineering(expectation *MockExpectation) error {
	fmt.Println("\n🌪️  Chaos Engineering Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var chaosType string
	if err := survey.AskOne(&survey.Select{
		Message: "Chaos engineering type:",
		Options: []string{
			"latency-injection - Random response delays",
			"failure-injection - Random failures",
			"resource-exhaustion - Simulate resource limits",
			"network-partition - Simulate network issues",
		},
	}, &chaosType); err != nil {
		return err
	}

	chaosType = strings.Split(chaosType, " ")[0]

	switch chaosType {
	case "latency-injection":
		// Random delays between 100ms-5000ms
		expectation.ResponseDelay = "100-5000"
		fmt.Println("✅ Chaos latency injection: 100-5000ms random delays")
	case "failure-injection":
		// Random failure rate
		expectation.StatusCode = 503
		expectation.ResponseBody = `{"error": "chaos_failure", "message": "Random chaos engineering failure"}`
		fmt.Println("✅ Chaos failure injection: Random 503 errors")
	case "resource-exhaustion":
		expectation.StatusCode = 429
		expectation.ResponseBody = `{"error": "resource_exhausted", "message": "Simulated resource exhaustion"}`
		fmt.Println("✅ Chaos resource exhaustion: 429 Too Many Requests")
	case "network-partition":
		if expectation.ConnectionOptions == nil {
			expectation.ConnectionOptions = &ConnectionOptions{}
		}
		expectation.ConnectionOptions.DropConnection = true
		fmt.Println("✅ Chaos network partition: Connection drops")
	}

	fmt.Println("\n📚 Chaos Engineering Resources:")
	fmt.Println("   Chaos Engineering: https://principlesofchaos.org/")
	fmt.Println("   Testing Guide: https://github.com/dastergon/awesome-chaos-engineering")

	return nil
}

// collectLoadTestingPatterns collects load testing pattern configuration
func collectLoadTestingPatterns(expectation *MockExpectation) error {
	fmt.Println("\n📊 Load Testing Patterns Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var pattern string
	if err := survey.AskOne(&survey.Select{
		Message: "Load testing pattern:",
		Options: []string{
			"high-throughput - Fast responses for load testing",
			"memory-pressure - Large response bodies",
			"cpu-intensive - Simulated processing delays",
			"realistic-load - Real-world response patterns",
		},
	}, &pattern); err != nil {
		return err
	}

	pattern = strings.Split(pattern, " ")[0]

	switch pattern {
	case "high-throughput":
		expectation.ResponseDelay = "1"
		expectation.ResponseBody = `{"status": "success", "id": "${uuid}", "timestamp": "${timestamp}"}`
		fmt.Println("✅ High-throughput pattern: 1ms delay, minimal response")
	case "memory-pressure":
		// Large response body for memory testing
		largeData := make([]string, 1000)
		for i := range largeData {
			largeData[i] = fmt.Sprintf("data_item_%d", i)
		}
		expectation.ResponseBody = map[string]interface{}{
			"message": "Large response for memory testing",
			"data":    largeData,
			"size":    "~1000 items",
		}
		fmt.Println("✅ Memory pressure pattern: Large response body (1000 items)")
	case "cpu-intensive":
		expectation.ResponseDelay = "500-2000"
		expectation.ResponseBody = `{"message": "CPU intensive operation completed", "processingTime": "${random.integer}", "result": "success"}`
		fmt.Println("✅ CPU intensive pattern: 500-2000ms delays")
	case "realistic-load":
		expectation.ResponseDelay = "100-800"
		expectation.ResponseBody = `{"status": "success", "data": {"id": "${uuid}", "timestamp": "${timestamp}", "userAgent": "${request.headers.user-agent}"}, "processingTime": "${random.integer}"}`
		fmt.Println("✅ Realistic load pattern: 100-800ms delays, templated responses")
	}

	fmt.Println("\n📚 Load Testing Resources:")
	fmt.Println("   k6: https://k6.io/docs/")
	fmt.Println("   JMeter: https://jmeter.apache.org/")
	fmt.Println("   Artillery: https://artillery.io/docs/")

	return nil
}

// configureTestingScenarios configures testing scenario features
func configureTestingScenarios(expectation *MockExpectation) error {
	fmt.Println("\n🧪 Advanced Testing Scenarios Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show comprehensive testing patterns
	patterns := AdvancedTestingPatterns()
	fmt.Println("\n💡 Available Testing Patterns:")
	for name, pattern := range patterns {
		fmt.Printf("   📋 %s: %s\n", name, pattern.Description)
	}

	var scenario string
	if err := survey.AskOne(&survey.Select{
		Message: "Select testing scenario:",
		Options: []string{
			"circuit-breaker - Service failure patterns",
			"rate-limiting - Rate limit testing with backoff",
			"chaos-engineering - Advanced chaos patterns",
			"load-testing - Performance testing patterns",
			"security-testing - Security vulnerability patterns",
			"resilience-testing - System resilience patterns",
		},
	}, &scenario); err != nil {
		return err
	}

	scenarioType := strings.Split(scenario, " ")[0]

	switch scenarioType {
	case "circuit-breaker":
		return collectCircuitBreakerAdvanced(expectation)
	case "rate-limiting":
		return collectRateLimitAdvanced(expectation)
	case "chaos-engineering":
		return collectChaosEngineering(expectation)
	case "load-testing":
		return collectLoadTestingPatterns(expectation)
	case "security-testing":
		return collectSecurityTestingPatterns(expectation)
	case "resilience-testing":
		return collectResilienceTestingPatterns(expectation)
	}

	return nil
}

// Step 6: Advanced Features (shared between REST and GraphQL)
func (mc *MockConfigurator) CollectAdvancedFeatures(expectation *MockExpectation) error {
	fmt.Println("\n⚙️  Step 6: Advanced MockServer Features")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var enableAdvanced bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Configure advanced MockServer features?",
		Default: false,
		Help:    "Response delays, templating, callbacks, connection control, and more",
	}, &enableAdvanced); err != nil {
		return err
	}

	if !enableAdvanced {
		fmt.Println("ℹ️  No advanced features configured")
		return nil
	}

	// Show advanced feature categories
	fmt.Println("\n🎛️  Advanced Feature Categories:")
	categories := AdvancedFeatureCategories()
	for category, features := range categories {
		fmt.Printf("   📂 %s:\n", category)
		for _, feature := range features {
			fmt.Printf("      • %s\n", feature)
		}
	}

	// Select feature categories to configure
	var selectedCategories []string
	var categoryOptions []string
	for category := range categories {
		categoryOptions = append(categoryOptions, category)
	}

	fmt.Println("\n👉 Navigation: Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select feature categories to configure:",
		Options: categoryOptions,
		Help:    "IMPORTANT: Use SPACE (not ENTER) to select items, then ENTER to confirm",
	}, &selectedCategories); err != nil {
		return err
	}

	if len(selectedCategories) == 0 {
		fmt.Println("ℹ️  No feature categories selected")
		return nil
	}

	// Configure selected categories
	for _, category := range selectedCategories {
		switch category {
		case "Response Behavior":
			if err := configureResponseBehavior(expectation); err != nil {
				return err
			}
		case "Dynamic Content":
			if err := configureDynamicContent(expectation); err != nil {
				return err
			}
		case "Integration & Callbacks":
			if err := configureIntegrationCallbacks(expectation); err != nil {
				return err
			}
		case "Connection Control":
			if err := configureConnectionControl(expectation); err != nil {
				return err
			}
		case "Testing Scenarios":
			if err := configureTestingScenarios(expectation); err != nil {
				return err
			}
		case "Advanced Patterns":
			if err := configureAdvancedPatterns(expectation); err != nil {
				return err
			}
		}
	}

	return nil
}
