package collections

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/builders"
	"github.com/hemantobora/auto-mock/internal/models"
)

// CollectionProcessor handles import and processing of API collections
type CollectionProcessor struct {
	projectName    string
	collectionType string
}

// APIRequest represents a single API request from collection
type APIRequest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	QueryParams map[string]string `json:"query_params"`
	PreScript   string            `json:"pre_script"`
	PostScript  string            `json:"post_script"`
	Variables   map[string]string `json:"variables"`
}

// APIResponse represents recorded response
type APIResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Cookies    map[string]string `json:"cookies"`
	Duration   time.Duration     `json:"duration"`
}

// ExecutionNode represents a node in the execution DAG
type ExecutionNode struct {
	API           APIRequest    `json:"api"`
	Dependencies  []string      `json:"dependencies"`
	Variables     []string      `json:"variables_provided"`
	Response      *APIResponse  `json:"response,omitempty"`
	ExecutionType ExecutionType `json:"-"`
}

type ExecutionType string

const (
	GRAPHQL ExecutionType = "GRAPHQL"
	REST    ExecutionType = "REST"
)

// NewCollectionProcessor creates a new collection processor
func NewCollectionProcessor(projectName, collectionType string) (*CollectionProcessor, error) {

	return &CollectionProcessor{
		projectName:    projectName,
		collectionType: collectionType,
	}, nil
}

// ProcessCollection handles the complete collection import workflow
func (cp *CollectionProcessor) ProcessCollection(filePath string) (string, error) {
	fmt.Printf("📂 COLLECTION IMPORT: %s\n", strings.ToUpper(cp.collectionType))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Step 1: Show disclaimer
	if err := cp.showDisclaimer(); err != nil {
		return "", err
	}

	// Step 2: Parse collection file
	apis, err := cp.ParseCollectionFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse collection: %w", err)
	}

	fmt.Printf("✅ Found %d API endpoints in collection\n", len(apis))

	// Step 3: Build execution DAG
	executionNodes, err := cp.buildExecutionDAG(apis)
	if err != nil {
		return "", fmt.Errorf("failed to build execution order: %w", err)
	}

	// Step 4: Execute APIs and record responses
	if err := cp.executeAPIs(executionNodes); err != nil {
		return "", fmt.Errorf("failed to execute APIs: %w", err)
	}

	// Step 5: Enhanced scenario detection and matching criteria configuration
	fmt.Println("\n🔍 ANALYZING APIs FOR SCENARIOS...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Enhanced GraphQL-aware scenario detection
	expectations, err := cp.configureExpectationCriteria(executionNodes)
	if err != nil {
		return "", fmt.Errorf("failed to configure matching: %w", err)
	}

	fmt.Printf("\n✅ Configured %d mock expectations from collection\n", len(expectations))
	expectations = builders.ExtendExpectationsForProgressive(expectations)

	// Step 6: Enhanced review and validation with save option
	if err := cp.reviewExpectations(expectations); err != nil {
		return "", fmt.Errorf("review failed: %w", err)
	}

	return builders.ExpectationsToMockServerJSON(expectations), nil
}

// Step 1: Show security disclaimer
func (cp *CollectionProcessor) showDisclaimer() error {
	fmt.Println("\n🔐 SECURITY & ENVIRONMENT DISCLAIMER")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("⚠️  IMPORTANT SECURITY NOTICE:")
	fmt.Println("   • You are responsible for maintaining secrets/credentials")
	fmt.Println("   • Remove sensitive data from collection before import")
	fmt.Println("   • Environment variables should be provided via -e or --env-file")
	fmt.Println("   • Pre/post scripts will be processed automatically")
	fmt.Println("   • Variables from collection will be extracted and managed")
	fmt.Println("\n🔧 ENVIRONMENT SETUP:")
	fmt.Println("   • Ensure all required environment variables are set")
	fmt.Println("   • If variables are missing, quit and restart after setup")
	fmt.Println("   • API execution will fail if dependencies are not met")
	fmt.Println("   • Ensure the API order in the collection is correct")
	fmt.Println("   • The tool will assume the order is correct and execute sequentially")

	var proceed bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Assuming you agree to the above, is the order of APIs in the collection correct? Continue?",
		Default: true,
	}, &proceed); err != nil {
		return err
	}

	if !proceed {
		return fmt.Errorf("user cancelled after disclaimer")
	}

	return nil
}

// Step 2: Parse collection file based on type
func (cp *CollectionProcessor) ParseCollectionFile(filePath string) ([]APIRequest, error) {
	fmt.Printf("\n📄 Parsing %s collection file: %s\n", cp.collectionType, filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, &models.CollectionParsingError{
			CollectionType: cp.collectionType,
			FilePath:       filePath,
			Cause:          err,
		}
	}

	switch cp.collectionType {
	case "postman":
		return cp.parsePostmanCollection(data)
	case "bruno":
		return cp.parseBrunoCollection(data)
	case "insomnia":
		return cp.parseInsomniaCollection(data)
	default:
		return nil, &models.CollectionParsingError{
			CollectionType: cp.collectionType,
			FilePath:       filePath,
			Cause:          fmt.Errorf("unsupported collection type: %s", cp.collectionType),
		}
	}
}

// configureIndividualMatching handles non-scenario APIs (renamed from original)
func (cp *CollectionProcessor) configureIndividualMatching(nodes []ExecutionNode) ([]builders.MockExpectation, error) {
	fmt.Println("\n🔧 Individual API Matching Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Configure matching for individual APIs")

	var expectations []builders.MockExpectation
	var mock_configurator builders.MockConfigurator

	for _, node := range nodes {
		if node.Response == nil {
			continue
		}

		fmt.Printf("\n🔧 Configuring: %s %s - %s\n", node.API.Method, cp.extractPath(node.API.URL), node.API.Name)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		// Build base expectation
		path, queryParams := mock_configurator.ParsePathAndQueryParams(node.API.URL)
		expectation := builders.MockExpectation{
			HttpRequest: &builders.HttpRequest{
				Method:                node.API.Method,
				Path:                  cp.extractPath(path),
				QueryStringParameters: queryParams,
				Headers:               map[string][]any{},
			},
			HttpResponse: &builders.HttpResponse{
				StatusCode: node.Response.StatusCode,
				Headers:    make(map[string][]string),
			},
		}

		if node.ExecutionType == GRAPHQL {
			// Decide transport by method and where query lives
			method := strings.ToUpper(node.API.Method)
			isGET := method == "GET"

			// Try to parse body as GraphQL envelope if present
			var gql map[string]any
			if node.API.Body != "" {
				_ = json.Unmarshal([]byte(node.API.Body), &gql) // ok if fails; we’ll fallback
			}
			query, _ := gql["query"].(string)
			vars, _ := gql["variables"].(map[string]any)

			if isGET {
				// GET: put fields in queryStringParameters.
				if expectation.HttpRequest.QueryStringParameters == nil {
					expectation.HttpRequest.QueryStringParameters = map[string][]string{}
				}
				if query != "" {
					expectation.HttpRequest.QueryStringParameters["query"] = []string{query}
				}
				if vars != nil {
					b, _ := json.Marshal(vars)
					expectation.HttpRequest.QueryStringParameters["variables"] = []string{string(b)}
				}
			} else {

				// If body couldn’t be parsed but we still have a literal, try a minimal envelope
				if query == "" && node.API.Body != "" && json.Valid([]byte(node.API.Body)) {
					// Accept any JSON that contains query/variables the user chooses
					var tmp any
					_ = json.Unmarshal([]byte(node.API.Body), &tmp)
					if m, ok := tmp.(map[string]any); ok {
						query, _ = m["query"].(string)
						vars, _ = m["variables"].(map[string]any)
					}
				}

				envelope := map[string]any{}
				if query != "" {
					envelope["query"] = query
				}
				if vars != nil {
					envelope["variables"] = vars
				}

				// Let the user choose STRICT vs ONLY_MATCHING_FIELDS vs REGEX (full-body)
				var mode string
				if err := survey.AskOne(&survey.Select{
					Message: "GraphQL POST body match mode:",
					Options: []string{"ONLY_MATCHING_FIELDS", "STRICT"},
					Default: "ONLY_MATCHING_FIELDS",
				}, &mode); err != nil {
					mode = "ONLY_MATCHING_FIELDS"
				}

				switch {
				case mode == "STRICT":
					expectation.HttpRequest.Body = map[string]any{
						"type":      "JSON",
						"json":      envelope,
						"matchType": "STRICT",
					}
				default: // ONLY_MATCHING_FIELDS
					expectation.HttpRequest.Body = map[string]any{
						"type":      "JSON",
						"json":      envelope,
						"matchType": "ONLY_MATCHING_FIELDS",
					}
				}
			}

			if err := mock_configurator.CollectRequestHeaderMatching(&expectation); err != nil {
				return nil, err
			}

			if err := mock_configurator.CollectAdvancedFeatures(&expectation); err != nil {
				return nil, err
			}

		} else {
			// Request body for methods that typically have bodies
			if node.API.Method == "POST" || node.API.Method == "PUT" || node.API.Method == "PATCH" {
				if err := mock_configurator.CollectRequestBody(&expectation, node.API.Body); err != nil {
					return nil, err
				}
			}

			// Configure matching criteria for this individual API
			if err := mock_configurator.CollectQueryParameterMatching(&expectation); err != nil {
				return nil, err
			}

			if err := mock_configurator.CollectPathMatchingStrategy(&expectation); err != nil {
				return nil, err
			}
		}

		if err := mock_configurator.CollectRequestHeaderMatching(&expectation); err != nil {
			return nil, err
		}

		if err := mock_configurator.CollectAdvancedFeatures(&expectation); err != nil {
			return nil, err
		}

		var v any
		if err := json.Unmarshal([]byte(node.Response.Body), &v); err != nil {
			return nil, err
		}
		// Set body wrapper
		expectation.HttpResponse.Body = map[string]any{
			"type": "JSON",
			"json": v,
		}
		fmt.Println("✅ Configured response body")

		for k, v := range node.Response.Headers {
			expectation.HttpResponse.Headers[k] = append(expectation.HttpResponse.Headers[k], v)
		}

		expectations = append(expectations, expectation)
	}

	return expectations, nil
}

// Step 3: Execute APIs sequentially with variable resolution
func (cp *CollectionProcessor) buildExecutionDAG(apis []APIRequest) ([]ExecutionNode, error) {
	fmt.Println("\n🔗 SEQUENTIAL EXECUTION SETUP")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Preparing %d APIs for execution in collection order...\n\n", len(apis))

	// Display all APIs in order
	for i, api := range apis {
		fmt.Printf("%d. %s %s - %s\n", i+1, api.Method, api.URL, api.Name)
	}

	// Create execution nodes in order (no dependency analysis needed)
	var executionNodes []ExecutionNode
	for _, api := range apis {
		node := ExecutionNode{
			API:          api,
			Dependencies: []string{},
			Variables:    []string{}, // Will be populated during execution
		}
		executionNodes = append(executionNodes, node)
	}

	fmt.Println("\n✅ APIs will execute in the order shown above")
	return executionNodes, nil
}

// Step 4: Execute APIs sequentially with runtime variable resolution and loading indicators
func (cp *CollectionProcessor) executeAPIs(nodes []ExecutionNode) error {
	fmt.Println("\n🚀 EXECUTING APIs SEQUENTIALLY")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📊 Progress: [")
	for i := 0; i < len(nodes); i++ {
		fmt.Printf(" ")
	}
	fmt.Printf("] 0/%d\n", len(nodes))

	// In-memory variable map (cleared after all executions)
	variables := make(map[string]string)

	// Process each API in order
	for i := range nodes {
		node := &nodes[i]

		// Update progress indicator
		progressBar := "📊 Progress: ["
		for j := 0; j < len(nodes); j++ {
			if j < i {
				progressBar += "✓"
			} else if j == i {
				progressBar += "⏳"
			} else {
				progressBar += " "
			}
		}
		progressBar += fmt.Sprintf("] %d/%d", i, len(nodes))

		fmt.Printf("\r%s", progressBar)
		fmt.Printf("\n\n▶️  [%d/%d] Executing: %s\n", i+1, len(nodes), node.API.Name)
		fmt.Println("   " + strings.Repeat("─", 50))

		// Step 1: Identify variables needed
		neededVars := cp.ExtractVariablesFromAPI(&node.API, true)
		if len(neededVars) > 0 {
			fmt.Printf("   📋 Variables needed: %v\n", neededVars)
		} else {
			fmt.Printf("   📋 No variables needed\n")
		}

		// Step 2: Run pre-script if available (before variable resolution)
		if node.API.PreScript != "" {
			fmt.Printf("   🔧 Running pre-script...\n")
			// Execute pre-script with collection-type awareness
			preScriptVars := cp.executePreScript(node.API.PreScript, node.API, variables)
			if len(preScriptVars) > 0 {
				fmt.Printf("   📦 Pre-script set variables: ")
				for k, v := range preScriptVars {
					variables[k] = v
					fmt.Printf("%s=%s ", k, v)
				}
				fmt.Println()
			} else {
				fmt.Printf("   ⚠️  Pre-script did not set any variables\n")
				fmt.Printf("   💡 Script content:\n%s\n", node.API.PreScript)
			}
		}

		// Step 3-5: Resolve variables
		if err := cp.resolveVariables(&node.API, neededVars, variables); err != nil {
			fmt.Printf("   ❌ Variable resolution failed: %v\n", err)

			var continueOnError bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Continue with remaining APIs?",
				Default: true,
			}, &continueOnError); err != nil {
				return err
			}

			if !continueOnError {
				return fmt.Errorf("execution stopped due to variable resolution error")
			}
			continue
		}

		// Step 6: Execute the API with loading indicator
		fmt.Printf("   ⏳ Making API call")

		// Enhanced loading animation with better visual feedback
		done := make(chan bool)
		start := time.Now()
		go func() {
			spinner := []string{"🔄", "⚙️", "🚀", "⚡", "🌐", "📶"}
			dots := []string{"", ".", "..", "..."}
			i := 0
			for {
				select {
				case <-done:
					return
				default:
					elapsed := time.Since(start).Truncate(time.Second)
					fmt.Printf("\r   %s Making API call%s [%s]",
						spinner[i%len(spinner)],
						dots[i%len(dots)],
						elapsed)
					i++
					time.Sleep(250 * time.Millisecond)
				}
			}
		}()

		response, err := cp.executeAPI(node.API, variables)
		done <- true
		fmt.Printf("\r   ")
		if err != nil {
			fmt.Printf("   ❌ API execution failed: %v\n", err)

			var continueOnError bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Continue with remaining APIs?",
				Default: true,
			}, &continueOnError); err != nil {
				return err
			}

			if !continueOnError {
				return fmt.Errorf("execution stopped on API error")
			}

			// Create mock response for failed request
			response = &APIResponse{
				StatusCode: 500,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"error": "API execution failed during collection import"}`,
				Cookies:    map[string]string{},
				Duration:   0,
			}
		}

		node.Response = response
		fmt.Printf("   ✅ Response: %d, Duration: %dms\n", response.StatusCode, response.Duration.Milliseconds())

		// Show FULL response for user to pick variables from
		fmt.Println("   ──────────────────────────────────────────────────")
		fmt.Println("   📄 FULL RESPONSE BODY (for variable extraction):")
		fmt.Println("   ──────────────────────────────────────────────────")
		// Pretty print JSON if possible
		var jsonData interface{}
		if err := json.Unmarshal([]byte(response.Body), &jsonData); err == nil {
			if prettyJSON, err := json.MarshalIndent(jsonData, "   ", "  "); err == nil {
				fmt.Println(string(prettyJSON))
			} else {
				fmt.Printf("   %s\n", response.Body)
			}
		} else {
			// Not JSON, show as-is
			fmt.Printf("   %s\n", response.Body)
		}
		fmt.Println("   ──────────────────────────────────────────────────")

		// Step 7: Run post-script to populate variables (collection-type aware)
		if node.API.PostScript != "" {
			fmt.Printf("   🔧 Running post-script...\n")
			extractedVars := cp.executePostScript(node.API.PostScript, node.API, response, variables)
			if len(extractedVars) > 0 {
				fmt.Printf("   📦 Variables extracted from response: ")
				for k, v := range extractedVars {
					variables[k] = v
					node.Variables = append(node.Variables, k)
					fmt.Printf("%s=%s ", k, v)
				}
				fmt.Println()
			} else {
				fmt.Printf("   ⚠️  Post-script did not extract any variables\n")
				fmt.Printf("   💡 Script content:\n%s\n", node.API.PostScript)
			}
		}
	}

	fmt.Printf("\n🎉 Executed %d APIs successfully!\n", len(nodes))
	fmt.Println("\n🧹 Clearing in-memory variables...")
	variables = nil // Clear the map

	return nil
}

// classifyAPIsByType separates REST and GraphQL APIs
func (cp *CollectionProcessor) classifyAPIsByType(nodes []ExecutionNode) ([]ExecutionNode, []ExecutionNode) {
	var restNodes, graphqlNodes []ExecutionNode

	// Iterate by index to mutate the original slice elements so the ExecutionType persists
	for i := range nodes {
		if nodes[i].Response == nil {
			continue
		}

		if cp.isGraphQLRequest(nodes[i].API) {
			nodes[i].ExecutionType = GRAPHQL
			graphqlNodes = append(graphqlNodes, nodes[i])
		} else {
			nodes[i].ExecutionType = REST
			restNodes = append(restNodes, nodes[i])
		}
	}

	return restNodes, graphqlNodes
}

// isGraphQLRequest determines if an API request is GraphQL
func (cp *CollectionProcessor) isGraphQLRequest(api APIRequest) bool {
	urlLower := strings.ToLower(api.URL)

	// 1️⃣ Path heuristic
	if strings.Contains(urlLower, "/graphql") || strings.Contains(urlLower, "/gql") {
		return true
	}

	// 2️⃣ Header heuristic
	for k, v := range api.Headers {
		if strings.EqualFold(k, "Content-Type") &&
			strings.Contains(strings.ToLower(v), "graphql") {
			return true
		}
		if strings.EqualFold(k, "X-GraphQL-Operation-Name") {
			return true
		}
	}

	// 3️⃣ Body inspection
	body := strings.TrimSpace(api.Body)
	if body == "" {
		return false
	}

	// Try JSON form
	var bodyMap map[string]interface{}
	if err := json.Unmarshal([]byte(body), &bodyMap); err == nil {
		if _, hasQuery := bodyMap["query"]; hasQuery {
			return true
		}
		if _, hasOpName := bodyMap["operationName"]; hasOpName {
			return true
		}
	}

	// 4️⃣ Raw GraphQL text fallback
	bodyLower := strings.ToLower(body)
	if strings.Contains(bodyLower, "query ") ||
		strings.Contains(bodyLower, "mutation ") ||
		strings.Contains(bodyLower, "subscription ") {
		return true
	}

	// 5️⃣ GET-style queries (?query=...)
	if strings.Contains(urlLower, "?query=") {
		return true
	}

	return false
}

// configureExpectationCriteria handles scenario detection and configuration
func (cp *CollectionProcessor) configureExpectationCriteria(nodes []ExecutionNode) ([]builders.MockExpectation, error) {
	fmt.Println("\n🎯 TYPE-AWARE EXPECTATION CONFIGURATION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Analyzing APIs for type intelligence matching...")

	// Step 1: Classify APIs by type (REST vs GraphQL)
	restNodes, graphqlNodes := cp.classifyAPIsByType(nodes)

	if len(graphqlNodes) > 0 {
		fmt.Printf("\n🔍 API Classification:\n")
		fmt.Printf("   • REST APIs: %d\n", len(restNodes))
		fmt.Printf("   • GraphQL APIs: %d\n\n", len(graphqlNodes))
	}
	return cp.configureIndividualMatching(nodes)
}

// Step 6: Enhanced Review expectations with configuration options
func (cp *CollectionProcessor) reviewExpectations(expectations []builders.MockExpectation) error {
	fmt.Println("\n📋 ENHANCED EXPECTATIONS REVIEW")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show summary without overwhelming JSON output
	fmt.Printf("✨ Successfully Generated %d Expectations!\n\n", len(expectations))
	fmt.Println("📊 Summary:")
	for i, exp := range expectations {
		method := exp.HttpRequest.Method
		path := exp.HttpRequest.Path
		status := exp.HttpResponse.StatusCode
		fmt.Printf("   [%d] %s %s → %d\n", i+1, method, path, status)
	}
	return nil
}

func (cp *CollectionProcessor) parsePostmanCollection(data []byte) ([]APIRequest, error) {
	// Add panic recovery for JSON parsing
	var parseErr error
	defer func() {
		if r := recover(); r != nil {
			parseErr = &models.CollectionParsingError{
				CollectionType: cp.collectionType,
				FilePath:       "",
				Cause:          fmt.Errorf("panic during collection parsing: %v", r),
			}
		}
	}()

	var collection map[string]interface{}
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, &models.CollectionParsingError{
			CollectionType: cp.collectionType,
			FilePath:       "", // Not available at this level
			Cause:          fmt.Errorf("invalid JSON: %w", err),
		}
	}

	var apis []APIRequest

	// Navigate Postman collection structure
	if info, ok := collection["info"].(map[string]interface{}); ok {
		fmt.Printf("Collection: %s\n", info["name"])
	}

	if items, ok := collection["item"].([]interface{}); ok {
		apis = append(apis, cp.parsePostmanItems(items)...)
	}

	if parseErr != nil {
		return nil, parseErr
	}

	return apis, nil
}

func (cp *CollectionProcessor) parsePostmanItems(items []interface{}) []APIRequest {
	var apis []APIRequest

	for _, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Handle nested folders
			if nestedItems, exists := itemMap["item"].([]interface{}); exists {
				apis = append(apis, cp.parsePostmanItems(nestedItems)...)
				continue
			}

			// Handle individual requests
			if request, exists := itemMap["request"].(map[string]interface{}); exists {
				api := APIRequest{
					ID:   fmt.Sprintf("postman_%d", len(apis)),
					Name: cp.getString(itemMap, "name"),
				}

				// Extract method
				api.Method = cp.getString(request, "method")

				// Extract URL
				if url, ok := request["url"].(map[string]interface{}); ok {
					api.URL = cp.getString(url, "raw")
				} else if urlStr, ok := request["url"].(string); ok {
					api.URL = urlStr
				}

				// Extract headers
				api.Headers = cp.extractPostmanHeaders(request)

				// Extract body
				if body, ok := request["body"].(map[string]interface{}); ok {
					// Check for raw body
					if raw := cp.getString(body, "raw"); raw != "" {
						api.Body = raw
					}

					// Check for urlencoded body
					if urlencoded, ok := body["urlencoded"].([]interface{}); ok {
						if api.QueryParams == nil {
							api.QueryParams = make(map[string]string)
						}
						var formPairs []string
						for _, item := range urlencoded {
							if itemMap, ok := item.(map[string]interface{}); ok {
								key := cp.getString(itemMap, "key")
								value := cp.getString(itemMap, "value")
								disabled := false
								if d, ok := itemMap["disabled"].(bool); ok {
									disabled = d
								}
								if !disabled && key != "" {
									// Store in QueryParams for variable extraction
									api.QueryParams[key] = value
									formPairs = append(formPairs, fmt.Sprintf("%s=%s", key, value))
								}
							}
						}
						// Store as body for execution
						api.Body = strings.Join(formPairs, "&")
					}

					// Check for formdata body
					if formdata, ok := body["formdata"].([]interface{}); ok {
						if api.QueryParams == nil {
							api.QueryParams = make(map[string]string)
						}
						var formPairs []string
						for _, item := range formdata {
							if itemMap, ok := item.(map[string]interface{}); ok {
								key := cp.getString(itemMap, "key")
								value := cp.getString(itemMap, "value")
								if key != "" {
									api.QueryParams[key] = value
									formPairs = append(formPairs, fmt.Sprintf("%s=%s", key, value))
								}
							}
						}
						api.Body = strings.Join(formPairs, "&")
					}
				}

				// Extract pre-request script
				if event, ok := itemMap["event"].([]interface{}); ok {
					for _, e := range event {
						if eventMap, ok := e.(map[string]interface{}); ok {
							if cp.getString(eventMap, "listen") == "prerequest" {
								if script, ok := eventMap["script"].(map[string]interface{}); ok {
									if exec, ok := script["exec"].([]interface{}); ok {
										var lines []string
										for _, line := range exec {
											if lineStr, ok := line.(string); ok {
												lines = append(lines, lineStr)
											}
										}
										api.PreScript = strings.Join(lines, "\n")
									}
								}
							}
							if cp.getString(eventMap, "listen") == "test" {
								if script, ok := eventMap["script"].(map[string]interface{}); ok {
									if exec, ok := script["exec"].([]interface{}); ok {
										var lines []string
										for _, line := range exec {
											if lineStr, ok := line.(string); ok {
												lines = append(lines, lineStr)
											}
										}
										api.PostScript = strings.Join(lines, "\n")
									}
								}
							}
						}
					}
				}

				apis = append(apis, api)
			}
		}
	}

	return apis
}

func (cp *CollectionProcessor) parseBrunoCollection(data []byte) ([]APIRequest, error) {
	// Try to parse as JSON first (bruno.json export)
	var collection map[string]interface{}
	if err := json.Unmarshal(data, &collection); err != nil {
		// If JSON parsing fails, try .bru file format
		return cp.parseBruFiles(string(data))
	}

	var apis []APIRequest
	fmt.Printf("🔧 Processing Bruno JSON collection...\n")

	// Handle Bruno JSON collection structure
	if items, ok := collection["items"].([]interface{}); ok {
		apis = append(apis, cp.parseBrunoItems(items)...)
	} else if requests, ok := collection["requests"].([]interface{}); ok {
		// Alternative structure
		for _, req := range requests {
			if reqMap, ok := req.(map[string]interface{}); ok {
				api := cp.parseBrunoRequest(reqMap)
				apis = append(apis, api)
			}
		}
	} else {
		// Try to parse as single request
		api := cp.parseBrunoRequest(collection)
		if api.Method != "" {
			apis = append(apis, api)
		}
	}

	fmt.Printf("✅ Parsed %d Bruno API requests\n", len(apis))
	return apis, nil
}

// parseBruFiles handles .bru file format parsing
func (cp *CollectionProcessor) parseBruFiles(content string) ([]APIRequest, error) {
	var apis []APIRequest
	fmt.Println("📜 Parsing .bru file format...")

	// Bruno uses a structured text format with sections
	// Split by meta blocks (each request starts with 'meta {')
	requestBlocks := cp.splitBrunoRequests(content)

	for i, reqBlock := range requestBlocks {
		if reqBlock == "" {
			continue
		}

		api := cp.parseSingleBruRequest(reqBlock, i+1)
		if api.Method != "" {
			apis = append(apis, api)
		}
	}

	fmt.Printf("✅ Parsed %d .bru requests\n", len(apis))
	return apis, nil
}

// splitBrunoRequests splits content into individual request blocks
func (cp *CollectionProcessor) splitBrunoRequests(content string) []string {
	// Bruno format uses 'meta {' to denote the start of a request
	var requests []string
	var currentRequest strings.Builder
	inRequest := false

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// Check if this is the start of a new request
		if strings.HasPrefix(strings.TrimSpace(line), "meta {") {
			// Save previous request if any
			if inRequest && currentRequest.Len() > 0 {
				requests = append(requests, currentRequest.String())
				currentRequest.Reset()
			}
			inRequest = true
		}

		if inRequest {
			currentRequest.WriteString(line)
			currentRequest.WriteString("\n")
		}
	}

	// Add the last request
	if currentRequest.Len() > 0 {
		requests = append(requests, currentRequest.String())
	}

	return requests
}

// parseSingleBruRequest parses a single .bru request block
func (cp *CollectionProcessor) parseSingleBruRequest(content string, index int) APIRequest {
	api := APIRequest{
		ID:          fmt.Sprintf("bruno_%d", index),
		Headers:     make(map[string]string),
		QueryParams: make(map[string]string),
	}

	lines := strings.Split(content, "\n")
	var currentSection string
	var sectionContent []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Detect section start
		if strings.HasSuffix(line, "{") && !strings.HasPrefix(line, "//") {
			// Save previous section
			if currentSection != "" {
				cp.parseBrunoSection(currentSection, sectionContent, &api)
			}
			// Start new section
			currentSection = strings.TrimSpace(strings.TrimSuffix(line, "{"))
			sectionContent = []string{}
			continue
		}

		// Detect section end
		if line == "}" {
			if currentSection != "" {
				cp.parseBrunoSection(currentSection, sectionContent, &api)
				currentSection = ""
				sectionContent = []string{}
			}
			continue
		}

		// Collect section content
		if currentSection != "" && line != "" {
			sectionContent = append(sectionContent, line)
		}
	}

	// Process any remaining section
	if currentSection != "" {
		cp.parseBrunoSection(currentSection, sectionContent, &api)
	}

	return api
}

// parseBrunoSection parses a specific section of a Bruno request
func (cp *CollectionProcessor) parseBrunoSection(section string, content []string, api *APIRequest) {
	switch section {
	case "meta":
		// Parse metadata (name, type, seq)
		for _, line := range content {
			if strings.HasPrefix(line, "name:") {
				api.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			}
		}

	case "get", "post", "put", "patch", "delete", "head", "options":
		// HTTP method and URL
		api.Method = strings.ToUpper(section)
		for _, line := range content {
			if strings.HasPrefix(line, "url:") {
				api.URL = strings.TrimSpace(strings.TrimPrefix(line, "url:"))
			}
		}

	case "query":
		// Query parameters
		for _, line := range content {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					// Remove ~ prefix if present (disabled param)
					if !strings.HasPrefix(key, "~") {
						api.QueryParams[key] = value
					}
				}
			}
		}

	case "headers":
		// Headers
		for _, line := range content {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					// Remove ~ prefix if present (disabled header)
					if !strings.HasPrefix(key, "~") {
						api.Headers[key] = value
					}
				}
			}
		}

	case "body", "body:json", "body:text", "body:xml", "body:form-urlencoded", "body:multipart-form":
		// Request body
		api.Body = strings.Join(content, "\n")

		// Handle form-urlencoded
		if section == "body:form-urlencoded" {
			var formPairs []string
			for _, line := range content {
				if strings.Contains(line, ":") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.TrimSpace(parts[1])
						if !strings.HasPrefix(key, "~") {
							if api.QueryParams == nil {
								api.QueryParams = make(map[string]string)
							}
							api.QueryParams[key] = value
							formPairs = append(formPairs, fmt.Sprintf("%s=%s", key, value))
						}
					}
				}
			}
			// Set the body as form-urlencoded string
			if len(formPairs) > 0 {
				api.Body = strings.Join(formPairs, "&")
				// Ensure Content-Type is set for form data
				if api.Headers == nil {
					api.Headers = make(map[string]string)
				}
				if api.Headers["Content-Type"] == "" {
					api.Headers["Content-Type"] = "application/x-www-form-urlencoded"
				}
			}
		}

	case "auth", "auth:basic", "auth:bearer", "auth:apikey":
		// Authentication
		for _, line := range content {
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])

					if section == "auth:bearer" && key == "token" {
						api.Headers["Authorization"] = "Bearer " + value
					} else if section == "auth:basic" {
						if key == "username" || key == "password" {
							// Store for later basic auth construction
							api.Headers["X-Auth-"+key] = value
						}
					}
				}
			}
		}

	case "script:pre-request":
		// Pre-request script
		api.PreScript = strings.Join(content, "\n")

	case "script:post-response", "tests":
		// Post-response script / tests
		api.PostScript = strings.Join(content, "\n")

	case "docs":
		// Documentation - ignore for now

	default:
		// Unknown section - log for debugging
		fmt.Printf("   ⚠️  Unknown Bruno section: %s\n", section)
	}
}

// parseBrunoItems handles Bruno JSON items array
func (cp *CollectionProcessor) parseBrunoItems(items []interface{}) []APIRequest {
	var apis []APIRequest

	for _, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Handle nested folders
			if nestedItems, exists := itemMap["items"].([]interface{}); exists {
				apis = append(apis, cp.parseBrunoItems(nestedItems)...)
				continue
			}

			// Handle individual request
			api := cp.parseBrunoRequest(itemMap)
			if api.Method != "" {
				apis = append(apis, api)
			}
		}
	}

	return apis
}

// parseBrunoRequest parses a Bruno JSON request object
func (cp *CollectionProcessor) parseBrunoRequest(reqMap map[string]interface{}) APIRequest {
	api := APIRequest{
		ID:          cp.getString(reqMap, "uid"),
		Name:        cp.getString(reqMap, "name"),
		Headers:     make(map[string]string),
		QueryParams: make(map[string]string),
	}

	// Bruno JSON export: 'type' field indicates request type (e.g., "http", "graphql")
	// The actual HTTP method is in request.method field, so we'll get it from there

	// Extract request section
	if request, ok := reqMap["request"].(map[string]interface{}); ok {
		// Get URL
		api.URL = cp.getString(request, "url")

		// Get method from request if not found in top level
		if api.Method == "" {
			api.Method = strings.ToUpper(cp.getString(request, "method"))
		}

		// Extract headers from request
		if headers, ok := request["headers"].([]interface{}); ok {
			for _, h := range headers {
				if header, ok := h.(map[string]interface{}); ok {
					name := cp.getString(header, "name")
					value := cp.getString(header, "value")
					enabled := true
					if e, ok := header["enabled"].(bool); ok {
						enabled = e
					}
					if enabled && name != "" {
						api.Headers[name] = value
					}
				}
			}
		}

		// Extract query parameters
		if params, ok := request["params"].([]interface{}); ok {
			for _, p := range params {
				if param, ok := p.(map[string]interface{}); ok {
					name := cp.getString(param, "name")
					value := cp.getString(param, "value")
					enabled := true
					if e, ok := param["enabled"].(bool); ok {
						enabled = e
					}
					if enabled && name != "" {
						api.QueryParams[name] = value
					}
				}
			}
		}

		// Extract body
		if body, ok := request["body"].(map[string]interface{}); ok {
			mode := cp.getString(body, "mode")

			switch mode {
			case "json":
				api.Body = cp.getString(body, "json")
			case "text":
				api.Body = cp.getString(body, "text")
			case "xml":
				api.Body = cp.getString(body, "xml")
			case "formUrlEncoded":
				if formData, ok := body["formUrlEncoded"].([]interface{}); ok {
					var formPairs []string
					if api.QueryParams == nil {
						api.QueryParams = make(map[string]string)
					}
					if api.Headers == nil {
						api.Headers = make(map[string]string)
					}
					for _, item := range formData {
						if formItem, ok := item.(map[string]interface{}); ok {
							name := cp.getString(formItem, "name")
							value := cp.getString(formItem, "value")
							enabled := true
							if e, ok := formItem["enabled"].(bool); ok {
								enabled = e
							}
							if enabled && name != "" {
								api.QueryParams[name] = value
								formPairs = append(formPairs, fmt.Sprintf("%s=%s", name, value))
							}
						}
					}
					if len(formPairs) > 0 {
						api.Body = strings.Join(formPairs, "&")
						if api.Headers["Content-Type"] == "" {
							api.Headers["Content-Type"] = "application/x-www-form-urlencoded"
						}
					}
				}
			case "multipartForm":
				if formData, ok := body["multipartForm"].([]interface{}); ok {
					for _, item := range formData {
						if formItem, ok := item.(map[string]interface{}); ok {
							name := cp.getString(formItem, "name")
							value := cp.getString(formItem, "value")
							enabled := true
							if e, ok := formItem["enabled"].(bool); ok {
								enabled = e
							}
							if enabled && name != "" {
								api.QueryParams[name] = value
							}
						}
					}
				}
			case "graphql":
				if graphqlBody, ok := body["graphql"].(map[string]interface{}); ok {
					query := cp.getString(graphqlBody, "query")
					variables := cp.getString(graphqlBody, "variables")

					graphqlRequest := map[string]interface{}{
						"query": query,
					}
					if variables != "" {
						var varsObj interface{}
						if err := json.Unmarshal([]byte(variables), &varsObj); err == nil {
							graphqlRequest["variables"] = varsObj
						}
					}
					if bodyBytes, err := json.Marshal(graphqlRequest); err == nil {
						api.Body = string(bodyBytes)
					}
				}
				api.Headers["Content-Type"] = "application/json"
			default:
				// Try to get body as string
				if bodyStr := cp.getString(body, "text"); bodyStr != "" {
					api.Body = bodyStr
				} else if bodyStr := cp.getString(body, "json"); bodyStr != "" {
					api.Body = bodyStr
				}
			}
		}

		// Extract authentication
		if auth, ok := request["auth"].(map[string]interface{}); ok {
			cp.extractBrunoAuth(auth, &api)
		}

		// Extract scripts
		if script, ok := request["script"].(map[string]interface{}); ok {
			// Pre-request script
			if preReq := cp.getString(script, "req"); preReq != "" {
				api.PreScript = preReq
			}
			// Post-response script
			if postRes := cp.getString(script, "res"); postRes != "" {
				api.PostScript = postRes
			}
		}
	}

	// Also check for scripts at top level (alternative Bruno export format)
	if script, ok := reqMap["script"].(map[string]interface{}); ok {
		if api.PreScript == "" {
			if preReq := cp.getString(script, "req"); preReq != "" {
				api.PreScript = preReq
			}
		}
		if api.PostScript == "" {
			if postRes := cp.getString(script, "res"); postRes != "" {
				api.PostScript = postRes
			}
		}
	}

	// Check for tests (Bruno sometimes uses this for post-response scripts)
	if tests := cp.getString(reqMap, "tests"); tests != "" && api.PostScript == "" {
		api.PostScript = tests
	}

	return api
}

func (cp *CollectionProcessor) parseInsomniaCollection(data []byte) ([]APIRequest, error) {
	var collection map[string]interface{}
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var apis []APIRequest
	// Store environment variables for template resolution
	envVars := make(map[string]string)
	fmt.Printf("🔧 Processing Insomnia collection...\n")

	// Handle multiple possible Insomnia export formats
	if resources, ok := collection["resources"].([]interface{}); ok {
		// First pass: extract environment variables
		for _, resource := range resources {
			if resourceMap, ok := resource.(map[string]interface{}); ok {
				resourceType := cp.getString(resourceMap, "_type")
				if resourceType == "environment" {
					if data, ok := resourceMap["data"].(map[string]interface{}); ok {
						for k, v := range data {
							if strVal, ok := v.(string); ok {
								envVars[k] = strVal
							}
						}
					}
				}
			}
		}

		// Second pass: parse requests with environment context
		for _, resource := range resources {
			if resourceMap, ok := resource.(map[string]interface{}); ok {
				resourceType := cp.getString(resourceMap, "_type")
				if resourceType == "request" {
					api := cp.parseInsomniaRequest(resourceMap)
					// Resolve Insomnia template tags
					cp.resolveInsomniaTemplateTags(&api, envVars)
					apis = append(apis, api)
				} else if resourceType == "grpc_request" {
					// Handle gRPC requests
					api := cp.parseInsomniaGrpcRequest(resourceMap)
					if api.Method != "" {
						cp.resolveInsomniaTemplateTags(&api, envVars)
						apis = append(apis, api)
					}
				} else if resourceType == "graphql_request" {
					// Handle GraphQL requests
					api := cp.parseInsomniaGraphQLRequest(resourceMap)
					if api.Method != "" {
						cp.resolveInsomniaTemplateTags(&api, envVars)
						apis = append(apis, api)
					}
				}
			}
		}
	} else if requests, ok := collection["requests"].([]interface{}); ok {
		// Alternative format
		for _, req := range requests {
			if reqMap, ok := req.(map[string]interface{}); ok {
				api := cp.parseInsomniaRequest(reqMap)
				cp.resolveInsomniaTemplateTags(&api, envVars)
				apis = append(apis, api)
			}
		}
	} else {
		// Try to parse as single request
		api := cp.parseInsomniaRequest(collection)
		if api.Method != "" {
			cp.resolveInsomniaTemplateTags(&api, envVars)
			apis = append(apis, api)
		}
	}

	fmt.Printf("✅ Parsed %d Insomnia API requests\n", len(apis))
	return apis, nil
}

// Utility helper methods

func (cp *CollectionProcessor) getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func (cp *CollectionProcessor) extractPostmanHeaders(request map[string]interface{}) map[string]string {
	headers := make(map[string]string)
	if headerList, ok := request["header"].([]interface{}); ok {
		for _, h := range headerList {
			if header, ok := h.(map[string]interface{}); ok {
				key := cp.getString(header, "key")
				value := cp.getString(header, "value")
				if key != "" && value != "" {
					headers[key] = value
				}
			}
		}
	}
	return headers
}

// parseInsomniaRequest parses a single Insomnia request resource
func (cp *CollectionProcessor) parseInsomniaRequest(resourceMap map[string]interface{}) APIRequest {
	api := APIRequest{
		ID:          cp.getString(resourceMap, "_id"),
		Name:        cp.getString(resourceMap, "name"),
		Method:      strings.ToUpper(cp.getString(resourceMap, "method")),
		URL:         cp.getString(resourceMap, "url"),
		Headers:     make(map[string]string),
		QueryParams: make(map[string]string),
	}

	// Extract headers
	if headers, ok := resourceMap["headers"].([]interface{}); ok {
		api.Headers = cp.extractInsomniaHeaders(headers)
	}

	// Extract body - properly handle different body types
	if body, ok := resourceMap["body"].(map[string]interface{}); ok {
		// Get the mimeType to determine how to parse body
		mimeType := cp.getString(body, "mimeType")

		// Extract actual body content based on type
		if text := cp.getString(body, "text"); text != "" {
			api.Body = text
		} else if params, ok := body["params"].([]interface{}); ok {
			// Handle form-urlencoded or form-data
			if api.QueryParams == nil {
				api.QueryParams = make(map[string]string)
			}
			var formPairs []string
			for _, p := range params {
				if param, ok := p.(map[string]interface{}); ok {
					name := cp.getString(param, "name")
					value := cp.getString(param, "value")
					disabled := false
					if d, ok := param["disabled"].(bool); ok {
						disabled = d
					}
					if !disabled && name != "" {
						api.QueryParams[name] = value
						formPairs = append(formPairs, fmt.Sprintf("%s=%s", name, value))
					}
				}
			}
			api.Body = strings.Join(formPairs, "&")
			if api.Headers["Content-Type"] == "" {
				api.Headers["Content-Type"] = "application/x-www-form-urlencoded"
			}
		} else if fileName := cp.getString(body, "fileName"); fileName != "" {
			// File upload - store filename as body indicator
			api.Body = fmt.Sprintf("[FILE: %s]", fileName)
		}

		// Set Content-Type if specified
		if mimeType != "" && api.Headers["Content-Type"] == "" {
			api.Headers["Content-Type"] = mimeType
		}
	} else if bodyStr, ok := resourceMap["body"].(string); ok {
		api.Body = bodyStr
	}

	// Extract parameters (query params)
	if parameters, ok := resourceMap["parameters"].([]interface{}); ok {
		if api.QueryParams == nil {
			api.QueryParams = make(map[string]string)
		}
		for _, p := range parameters {
			if param, ok := p.(map[string]interface{}); ok {
				name := cp.getString(param, "name")
				value := cp.getString(param, "value")
				disabled := false
				if d, ok := param["disabled"].(bool); ok {
					disabled = d
				}
				if !disabled && name != "" {
					api.QueryParams[name] = value
				}
			}
		}
	}

	// Extract authentication with expanded support
	if auth, ok := resourceMap["authentication"].(map[string]interface{}); ok {
		cp.extractInsomniaAuth(auth, &api)
	}

	// Extract Insomnia scripts/hooks
	// Insomnia doesn't have built-in pre/post scripts like Postman,
	// but some plugins or custom exports might include them
	if hooks, ok := resourceMap["hooks"].(map[string]interface{}); ok {
		if preRequest := cp.getString(hooks, "beforeRequest"); preRequest != "" {
			api.PreScript = preRequest
		}
		if postResponse := cp.getString(hooks, "afterResponse"); postResponse != "" {
			api.PostScript = postResponse
		}
	}

	// Alternative script locations
	if preRequestScript := cp.getString(resourceMap, "preRequestScript"); preRequestScript != "" {
		api.PreScript = preRequestScript
	}
	if testScript := cp.getString(resourceMap, "testScript"); testScript != "" {
		api.PostScript = testScript
	}

	return api
}

// parseInsomniaGrpcRequest handles gRPC requests from Insomnia
func (cp *CollectionProcessor) parseInsomniaGrpcRequest(resourceMap map[string]interface{}) APIRequest {
	// Convert gRPC to REST-like representation for processing
	api := APIRequest{
		ID:     cp.getString(resourceMap, "_id"),
		Name:   cp.getString(resourceMap, "name") + " (gRPC)",
		Method: "POST",                                   // gRPC is typically POST-based
		URL:    cp.getString(resourceMap, "protoFileId"), // Use proto file as identifier
		Headers: map[string]string{
			"Content-Type": "application/grpc+proto",
		},
	}

	// Extract body from gRPC request
	if body, ok := resourceMap["body"].(map[string]interface{}); ok {
		api.Body = cp.getString(body, "text")
	}

	return api
}

// parseInsomniaGraphQLRequest handles GraphQL requests from Insomnia
func (cp *CollectionProcessor) parseInsomniaGraphQLRequest(resourceMap map[string]interface{}) APIRequest {
	api := APIRequest{
		ID:     cp.getString(resourceMap, "_id"),
		Name:   cp.getString(resourceMap, "name") + " (GraphQL)",
		Method: "POST",
		URL:    "/graphql", // Standard GraphQL endpoint
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	// Extract GraphQL query and variables
	if body, ok := resourceMap["body"].(map[string]interface{}); ok {
		query := cp.getString(body, "query")
		variables := cp.getString(body, "variables")
		operationName := cp.getString(body, "operationName")

		// Create GraphQL request body
		graphqlBody := map[string]interface{}{
			"query": query,
		}

		if operationName != "" {
			graphqlBody["operationName"] = operationName
		}

		if variables != "" {
			// Try to parse variables as JSON
			var varsObj interface{}
			if err := json.Unmarshal([]byte(variables), &varsObj); err == nil {
				graphqlBody["variables"] = varsObj
			} else {
				// If not valid JSON, store as string
				graphqlBody["variables"] = variables
			}
		}

		// Convert to JSON string
		if bodyBytes, err := json.Marshal(graphqlBody); err == nil {
			api.Body = string(bodyBytes)
		}
	}

	return api
}

// extractInsomniaAuth extracts authentication information
func (cp *CollectionProcessor) extractInsomniaAuth(auth map[string]interface{}, api *APIRequest) {
	authType := cp.getString(auth, "type")
	switch authType {
	case "bearer":
		token := cp.getString(auth, "token")
		if token != "" {
			api.Headers["Authorization"] = "Bearer " + token
		}
	case "basic":
		username := cp.getString(auth, "username")
		password := cp.getString(auth, "password")
		if username != "" || password != "" {
			api.Headers["Authorization"] = "Basic " + username + ":" + password
		}
	case "apikey", "api-key":
		// Insomnia supports both "apikey" and "api-key"
		key := cp.getString(auth, "key")
		value := cp.getString(auth, "value")
		addTo := cp.getString(auth, "addTo")

		if key != "" && value != "" {
			switch addTo {
			case "header":
				api.Headers[key] = value
			case "query":
				if api.QueryParams == nil {
					api.QueryParams = make(map[string]string)
				}
				api.QueryParams[key] = value
			default:
				// Default to header
				api.Headers[key] = value
			}
		}
	case "oauth2":
		// OAuth2 - check if access token is available
		if accessToken := cp.getString(auth, "accessToken"); accessToken != "" {
			api.Headers["Authorization"] = "Bearer " + accessToken
		} else if token := cp.getString(auth, "token"); token != "" {
			api.Headers["Authorization"] = "Bearer " + token
		}
	case "hawk":
		// Hawk authentication - store for later processing
		api.Headers["X-Auth-Type"] = "hawk"
	case "awsv4", "aws-iam":
		// AWS Signature v4 - store for later processing
		api.Headers["X-Auth-Type"] = "awsv4"
	case "ntlm":
		// NTLM authentication - store for later processing
		api.Headers["X-Auth-Type"] = "ntlm"
	case "digest":
		// Digest authentication - store for later processing
		api.Headers["X-Auth-Type"] = "digest"
		username := cp.getString(auth, "username")
		password := cp.getString(auth, "password")
		if username != "" {
			api.Headers["X-Auth-Username"] = username
		}
		if password != "" {
			api.Headers["X-Auth-Password"] = password
		}
	}
}

func (cp *CollectionProcessor) extractInsomniaHeaders(headers []interface{}) map[string]string {
	result := make(map[string]string)
	for _, h := range headers {
		if header, ok := h.(map[string]interface{}); ok {
			key := cp.getString(header, "name")
			value := cp.getString(header, "value")
			if key != "" && value != "" {
				result[key] = value
			}
		}
	}
	return result
}

func (cp *CollectionProcessor) executeAPI(api APIRequest, variables map[string]string) (*APIResponse, error) {
	start := time.Now()

	// Replace variables in URL
	url := cp.replaceVariables(api.URL, variables)

	// Create HTTP request
	var body io.Reader
	if api.Body != "" {
		bodyContent := cp.replaceVariables(api.Body, variables)
		body = strings.NewReader(bodyContent)
	}

	req, err := http.NewRequest(api.Method, url, body)
	if err != nil {
		return nil, &models.APIExecutionError{
			APIName: api.Name,
			Method:  api.Method,
			URL:     api.URL,
			Cause:   fmt.Errorf("failed to create request: %w", err),
		}
	}

	// Add headers
	for k, v := range api.Headers {
		req.Header.Set(k, cp.replaceVariables(v, variables))
	}

	// Set Content-Type for form data if body is urlencoded format
	if api.Body != "" && strings.Contains(api.Body, "=") && !strings.Contains(api.Body, "{") {
		// Looks like form data
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &models.APIExecutionError{
			APIName: api.Name,
			Method:  api.Method,
			URL:     api.URL,
			Cause:   fmt.Errorf("request failed: %w", err),
		}
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &models.APIExecutionError{
			APIName:    api.Name,
			Method:     api.Method,
			URL:        api.URL,
			StatusCode: resp.StatusCode,
			Cause:      fmt.Errorf("failed to read response: %w", err),
		}
	}

	// Extract headers
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	// Extract cookies
	cookies := make(map[string]string)
	for _, cookie := range resp.Cookies() {
		cookies[cookie.Name] = cookie.Value
	}

	return &APIResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       string(respBody),
		Cookies:    cookies,
		Duration:   time.Since(start),
	}, nil
}

func (cp *CollectionProcessor) replaceVariables(text string, variables map[string]string) string {
	for k, v := range variables {
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
		text = strings.ReplaceAll(text, "${"+k+"}", v)
	}
	return text
}

func (cp *CollectionProcessor) extractPath(url string) string {
	// Extract path from full URL
	if strings.Contains(url, "://") {
		parts := strings.SplitN(url, "://", 2)
		if len(parts) > 1 {
			remaining := parts[1]
			if idx := strings.Index(remaining, "/"); idx != -1 {
				return remaining[idx:]
			}
		}
	}
	return url
}

// resolveVariables performs the 5-step variable resolution process
func (cp *CollectionProcessor) resolveVariables(api *APIRequest, neededVars []string, variables map[string]string) error {
	for _, varName := range neededVars {
		// Check if already resolved
		if _, exists := variables[varName]; exists {
			fmt.Printf("   ✅ %s (from previous API)\n", varName)
			continue
		}

		// Step 2: Check environment
		if envVal := os.Getenv(varName); envVal != "" {
			variables[varName] = envVal
			fmt.Printf("   ✅ %s (from environment)\n", varName)
			continue
		}

		// Step 3: Run pre-script if available
		if api.PreScript != "" {
			fmt.Printf("   🔧 Running pre-script for %s...\n", varName)
			if val := cp.executePreScriptForVariable(api.PreScript, varName, variables); val != "" {
				variables[varName] = val
				fmt.Printf("   ✅ %s (from pre-script)\n", varName)
				continue
			}
		}

		// Step 5: Ask user and confirm
		fmt.Printf("\n   ⚠️  Variable '%s' not found in environment or scripts\n", varName)

		var value string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Enter value for '%s':", varName),
			Help:    "This variable is needed to execute the API. Enter the value or press Ctrl+C to cancel.",
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

		variables[varName] = value
		fmt.Printf("   ✅ %s (user input)\n", varName)
	}

	return nil
}

// executePreScriptForVariable extracts a specific variable from pre-script execution
func (cp *CollectionProcessor) executePreScriptForVariable(preScript string, varName string, existingVars map[string]string) string {
	// Look for direct assignment patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`pm\.environment\.set\(["']%s["'],\s*["']([^"']+)["']`, varName)),
		regexp.MustCompile(fmt.Sprintf(`pm\.globals\.set\(["']%s["'],\s*["']([^"']+)["']`, varName)),
		regexp.MustCompile(fmt.Sprintf(`pm\.collectionVariables\.set\(["']%s["'],\s*["']([^"']+)["']`, varName)),
		regexp.MustCompile(fmt.Sprintf(`%s\s*=\s*["']([^"']+)["']`, varName)),
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(preScript); len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// normalizeScript converts collection-specific script syntax to Postman-compatible syntax
func (cp *CollectionProcessor) normalizeScript(script string) string {
	switch cp.collectionType {
	case "bruno":
		// First convert Bruno-specific constructs to Postman-compatible
		converted := cp.convertBrunoScriptToPostman(script)
		// Then apply generic normalizations common across sources
		return cp.applyGenericScriptNormalizations(converted)
	case "insomnia":
		// Convert Insomnia constructs and then apply generic normalizations
		converted := cp.convertInsomniaScriptToPostman(script)
		return cp.applyGenericScriptNormalizations(converted)
	case "postman":
		// Even Postman scripts sometimes use non-standard patterns like pm.response.body
		return cp.applyGenericScriptNormalizations(script)
	default:
		return cp.applyGenericScriptNormalizations(script)
	}
}

// executePreScript executes pre-script and extracts variables using JavaScript engine
func (cp *CollectionProcessor) executePreScript(preScript string, api APIRequest, existingVars map[string]string) map[string]string {
	// Normalize script based on collection type
	normalizedScript := cp.normalizeScript(preScript)

	fmt.Printf("   🔍 Executing pre-script with JavaScript engine...\n")

	// Create script engine
	engine := NewScriptEngine(existingVars)
	// Provide request context for scripts
	engine.SetRequestData(api.Method, api.URL, api.Body, api.Headers)

	// Execute the script
	err := engine.Execute(normalizedScript)
	if err != nil {
		fmt.Printf("   ⚠️  Script execution error: %v\n", err)
		fmt.Printf("   💡 Script content:\n%s\n", normalizedScript)
		// Return empty map on error instead of crashing
		return make(map[string]string)
	}

	// Get extracted variables
	extractedVars := engine.GetExtractedVariables()

	if len(extractedVars) == 0 {
		fmt.Printf("   ⚠️  No variables extracted from pre-script\n")
	} else {
		fmt.Printf("   ✅ Extracted %d variable(s) from pre-script\n", len(extractedVars))
	}

	return extractedVars
}

// executePostScript executes post-script and extracts variables using JavaScript engine
func (cp *CollectionProcessor) executePostScript(postScript string, api APIRequest, response *APIResponse, existingVars map[string]string) map[string]string {
	// Parse response body as JSON for script context
	var jsonData interface{}
	if err := json.Unmarshal([]byte(response.Body), &jsonData); err != nil {
		fmt.Printf("   ⚠️  Failed to parse response as JSON: %v\n", err)
		// Try to work with response body as string
		jsonData = response.Body
	}

	// Normalize script based on collection type
	normalizedScript := cp.normalizeScript(postScript)

	fmt.Printf("   🔍 Executing post-script with JavaScript engine...\n")

	// Create script engine
	engine := NewScriptEngine(existingVars)
	// Provide request context for scripts
	engine.SetRequestData(api.Method, api.URL, api.Body, api.Headers)

	// Set response data (json, text, status, headers)
	engine.SetResponseData(jsonData, response.Body, response.StatusCode, response.Headers)

	// Execute the script
	err := engine.Execute(normalizedScript)
	if err != nil {
		fmt.Printf("   ⚠️  Script execution error: %v\n", err)
		fmt.Printf("   💡 Script content:\n%s\n", normalizedScript)
		return make(map[string]string)
	}

	// Get extracted variables
	extractedVars := engine.GetExtractedVariables()

	if len(extractedVars) == 0 {
		fmt.Printf("   ⚠️  No variables extracted from post-script\n")
	} else {
		fmt.Printf("   ✅ Extracted %d variable(s) from post-script\n", len(extractedVars))
	}

	return extractedVars
}

// extractVariablesFromAPI extracts all {{...}} and ${...} placeholders from a single API
func (cp *CollectionProcessor) ExtractVariablesFromAPI(api *APIRequest, includeAuth bool) []string {
	varSet := make(map[string]bool)

	// Extract from URL
	for _, match := range cp.findPlaceholders(api.URL) {
		varSet[match] = true
	}

	// Extract from headers
	for k, v := range api.Headers {
		if includeAuth || (strings.EqualFold(k, "Authorization") && strings.EqualFold(k, "authorization")) { // Skip Authorization header
			for _, match := range cp.findPlaceholders(v) {
				varSet[match] = true
			}
		}
	}

	// Extract from body
	for _, match := range cp.findPlaceholders(api.Body) {
		varSet[match] = true
	}

	// Extract from query params (includes form data)
	for _, v := range api.QueryParams {
		for _, match := range cp.findPlaceholders(v) {
			varSet[match] = true
		}
	}

	// Also check pre-script for variable declarations
	for _, match := range cp.findVariableDeclarations(api.PreScript) {
		varSet[match] = true
	}

	result := []string{}
	for v := range varSet {
		result = append(result, v)
	}
	return result
}

// findPlaceholders extracts {{variable}} and ${variable} patterns from text
func (cp *CollectionProcessor) findPlaceholders(text string) []string {
	var matches []string
	varSet := make(map[string]bool) // Prevent duplicates

	// Match both {{var}} and ${var} patterns
	re1 := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	re2 := regexp.MustCompile(`\$\{([^}]+)\}`)

	for _, re := range []*regexp.Regexp{re1, re2} {
		for _, match := range re.FindAllStringSubmatch(text, -1) {
			if len(match) > 1 {
				variable := strings.TrimSpace(match[1])
				// Filter out common template functions that aren't variables
				if !cp.isTemplateFunction(variable) && !varSet[variable] {
					varSet[variable] = true
					matches = append(matches, variable)
				}
			}
		}
	}

	return matches
}

// isTemplateFunction checks if a variable is actually a template function
func (cp *CollectionProcessor) isTemplateFunction(variable string) bool {
	templateFunc := []string{
		"uuid", "timestamp", "randomInt", "randomAlpha", "guid",
		"$randomInt", "$randomString", "$timestamp", "$guid",
	}
	for _, fn := range templateFunc {
		if variable == fn {
			return true
		}
	}
	return false
}

// findVariableDeclarations extracts variable names from script declarations
func (cp *CollectionProcessor) findVariableDeclarations(script string) []string {
	var variables []string
	if script == "" {
		return variables
	}

	// Look for pm.environment.set, pm.globals.set patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`pm\.environment\.set\(["']([^"']+)["']`),
		regexp.MustCompile(`pm\.globals\.set\(["']([^"']+)["']`),
		regexp.MustCompile(`pm\.collectionVariables\.set\(["']([^"']+)["']`),
		// Also look for variable assignments
		regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\s*=`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(script, -1)
		for _, match := range matches {
			if len(match) > 1 {
				variables = append(variables, match[1])
			}
		}
	}

	return variables
}

// extractBrunoAuth extracts authentication information from Bruno format
func (cp *CollectionProcessor) extractBrunoAuth(auth map[string]interface{}, api *APIRequest) {
	authType := cp.getString(auth, "type")
	switch authType {
	case "bearer":
		token := cp.getString(auth, "token")
		if token != "" {
			api.Headers["Authorization"] = "Bearer " + token
		}
	case "basic":
		username := cp.getString(auth, "username")
		password := cp.getString(auth, "password")
		if username != "" || password != "" {
			api.Headers["Authorization"] = "Basic " + username + ":" + password
		}
	case "apikey":
		key := cp.getString(auth, "key")
		value := cp.getString(auth, "value")
		if key != "" && value != "" {
			api.Headers[key] = value
		}
	case "awsv4":
		// AWS Signature v4 - store for later processing
		api.Headers["X-Auth-Type"] = "awsv4"
	case "oauth2":
		// OAuth2 - check if access token is available
		if accessToken := cp.getString(auth, "accessToken"); accessToken != "" {
			api.Headers["Authorization"] = "Bearer " + accessToken
		}
	}
}

// convertBrunoScriptToPostman converts Bruno script syntax to Postman-compatible syntax
func (cp *CollectionProcessor) convertBrunoScriptToPostman(brunoScript string) string {
	// Convert Bruno's req/res to Postman's request/response FIRST (before bru. conversion)
	postmanScript := strings.ReplaceAll(brunoScript, "res.getBody()", "pm.response.json()")
	postmanScript = strings.ReplaceAll(postmanScript, "req.", "pm.request.")
	postmanScript = strings.ReplaceAll(postmanScript, "res.", "pm.response.")

	// Handle common Bruno pattern res.body.<prop> -> pm.response.json().<prop>
	// Do this before converting bru.* so that any occurrences get normalized properly
	postmanScript = regexp.MustCompile(`\bpm\.response\.body\b`).ReplaceAllString(postmanScript, "pm.response.json()")

	// Convert Bruno's getEnvVar/setEnvVar to Postman's environment.get/set
	postmanScript = regexp.MustCompile(`bru\.getEnvVar\(([^)]+)\)`).ReplaceAllString(postmanScript, "pm.environment.get($1)")
	postmanScript = regexp.MustCompile(`bru\.setEnvVar\(([^,]+),\s*([^)]+)\)`).ReplaceAllString(postmanScript, "pm.environment.set($1, $2)")

	// Convert older Bruno's getVar/setVar to Postman's environment.get/set
	postmanScript = regexp.MustCompile(`bru\.getVar\(([^)]+)\)`).ReplaceAllString(postmanScript, "pm.environment.get($1)")
	postmanScript = regexp.MustCompile(`bru\.setVar\(([^,]+),\s*([^)]+)\)`).ReplaceAllString(postmanScript, "pm.environment.set($1, $2)")

	// Finally, convert any remaining bru. prefix to pm.
	postmanScript = strings.ReplaceAll(postmanScript, "bru.", "pm.")

	return postmanScript
}

// convertInsomniaScriptToPostman converts Insomnia script syntax to Postman-compatible syntax
func (cp *CollectionProcessor) convertInsomniaScriptToPostman(insomniaScript string) string {
	// Insomnia uses different variable access patterns
	postmanScript := insomniaScript

	// Convert Insomnia's insomnia.environment.get/set to pm.environment.get/set
	postmanScript = strings.ReplaceAll(postmanScript, "insomnia.environment.", "pm.environment.")
	postmanScript = strings.ReplaceAll(postmanScript, "insomnia.globals.", "pm.globals.")

	// Convert response access
	postmanScript = strings.ReplaceAll(postmanScript, "insomnia.response.", "pm.response.")
	postmanScript = strings.ReplaceAll(postmanScript, "insomnia.request.", "pm.request.")

	// Convert _.variable syntax to pm.environment.get('variable')
	re := regexp.MustCompile(`_\.([a-zA-Z_][a-zA-Z0-9_]*)`)
	postmanScript = re.ReplaceAllString(postmanScript, "pm.environment.get('$1')")

	// Convert getEnvironmentVariable to pm.environment.get
	postmanScript = regexp.MustCompile(`getEnvironmentVariable\(([^)]+)\)`).ReplaceAllString(postmanScript, "pm.environment.get($1)")
	postmanScript = regexp.MustCompile(`setEnvironmentVariable\(([^,]+),\s*([^)]+)\)`).ReplaceAllString(postmanScript, "pm.environment.set($1, $2)")

	return postmanScript
}

// applyGenericScriptNormalizations fixes common non-standard patterns across all sources
func (cp *CollectionProcessor) applyGenericScriptNormalizations(script string) string {
	s := script

	// Normalize any usage of pm.response.body -> pm.response.json()
	s = regexp.MustCompile(`\bpm\.response\.body\b`).ReplaceAllString(s, "pm.response.json()")

	// Normalize Bruno-style alias that may have slipped through: res.body -> pm.response.json()
	s = regexp.MustCompile(`\bres\.body\b`).ReplaceAllString(s, "pm.response.json()")

	// Normalize response status access
	// res.status -> pm.response.code()
	s = regexp.MustCompile(`\bres\.status\b`).ReplaceAllString(s, "pm.response.code()")
	// pm.response.status -> pm.response.code()
	s = regexp.MustCompile(`\bpm\.response\.status\b`).ReplaceAllString(s, "pm.response.code()")

	// Normalize bracket header access to .get()
	// pm.response.headers["X"] -> pm.response.headers.get("X")
	s = regexp.MustCompile(`pm\.response\.headers\s*\[\s*(["'][^"']+["'])\s*\]`).ReplaceAllString(s, "pm.response.headers.get($1)")
	// pm.request.headers["X"] -> pm.request.headers.get("X")
	s = regexp.MustCompile(`pm\.request\.headers\s*\[\s*(["'][^"']+["'])\s*\]`).ReplaceAllString(s, "pm.request.headers.get($1)")

	return s
}

// resolveInsomniaTemplateTags resolves Insomnia template tags like _.variableName
func (cp *CollectionProcessor) resolveInsomniaTemplateTags(api *APIRequest, envVars map[string]string) {
	// Resolve in URL
	api.URL = cp.resolveInsomniaTemplates(api.URL, envVars)

	// Resolve in headers
	for k, v := range api.Headers {
		api.Headers[k] = cp.resolveInsomniaTemplates(v, envVars)
	}

	// Resolve in body
	if api.Body != "" {
		api.Body = cp.resolveInsomniaTemplates(api.Body, envVars)
	}

	// Resolve in query params
	for k, v := range api.QueryParams {
		api.QueryParams[k] = cp.resolveInsomniaTemplates(v, envVars)
	}
}

// resolveInsomniaTemplates resolves Insomnia-style templates
func (cp *CollectionProcessor) resolveInsomniaTemplates(text string, envVars map[string]string) string {
	// Insomnia uses _.variableName or {{ _.variableName }} syntax
	re := regexp.MustCompile(`\{\{\s*_\.([^}\s]+)\s*\}\}`)
	text = re.ReplaceAllStringFunc(text, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) > 1 {
			varName := submatches[1]
			if val, exists := envVars[varName]; exists {
				return val
			}
		}
		return match
	})

	// Also handle direct _.variableName references (less common)
	re2 := regexp.MustCompile(`_\.([a-zA-Z_][a-zA-Z0-9_]*)`)
	text = re2.ReplaceAllStringFunc(text, func(match string) string {
		submatches := re2.FindStringSubmatch(match)
		if len(submatches) > 1 {
			varName := submatches[1]
			if val, exists := envVars[varName]; exists {
				return val
			}
		}
		return match
	})

	return text
}
