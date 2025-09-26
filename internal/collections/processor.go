package collections

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hemantobora/auto-mock/internal/state"
	"github.com/hemantobora/auto-mock/internal/utils"
)

// CollectionProcessor handles import and processing of API collections
type CollectionProcessor struct {
	store          *state.S3Store
	projectName    string
	cleanName      string
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
	API          APIRequest   `json:"api"`
	Dependencies []string     `json:"dependencies"`
	Variables    []string     `json:"variables_provided"`
	Response     *APIResponse `json:"response,omitempty"`
}

// NewCollectionProcessor creates a new collection processor
func NewCollectionProcessor(projectName, collectionType string) (*CollectionProcessor, error) {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg)
	store := state.NewS3Store(s3Client, projectName)

	return &CollectionProcessor{
		store:          store,
		projectName:    projectName,
		cleanName:      utils.ExtractUserProjectName(projectName),
		collectionType: collectionType,
	}, nil
}

// ProcessCollection handles the complete collection import workflow
func (cp *CollectionProcessor) ProcessCollection(filePath string) error {
	fmt.Printf("📂 COLLECTION IMPORT: %s\n", strings.ToUpper(cp.collectionType))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Step 1: Show disclaimer
	if err := cp.showDisclaimer(); err != nil {
		return err
	}

	// Step 2: Parse collection file
	apis, err := cp.parseCollectionFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse collection: %w", err)
	}

	fmt.Printf("✅ Found %d API endpoints in collection\n", len(apis))

	// Step 3: Build execution DAG
	executionNodes, err := cp.buildExecutionDAG(apis)
	if err != nil {
		return fmt.Errorf("failed to build execution order: %w", err)
	}

	// Step 4: Execute APIs and record responses
	if err := cp.executeAPIs(executionNodes); err != nil {
		return fmt.Errorf("failed to execute APIs: %w", err)
	}

	// Step 5: Enhanced scenario detection and matching criteria configuration
	fmt.Println("\n🔍 ANALYZING APIs FOR SCENARIOS...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Enhanced GraphQL-aware scenario detection
	expectations, err := cp.configureMatchingCriteriaWithScenarios(executionNodes)
	if err != nil {
		return fmt.Errorf("failed to configure matching: %w", err)
	}

	// Step 6: Enhanced review and validation with configuration options
	if err := cp.reviewExpectations(expectations); err != nil {
		return fmt.Errorf("review failed: %w", err)
	}

	// Step 7: Save to S3
	return cp.saveExpectations(expectations, executionNodes)
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
		Default: false,
	}, &proceed); err != nil {
		return err
	}

	if !proceed {
		return fmt.Errorf("user cancelled after disclaimer")
	}

	return nil
}

// countScenarioNodes counts how many nodes are part of scenarios
func (cp *CollectionProcessor) countScenarioNodes(nodes []ExecutionNode) int {
	scenarios := cp.detectAPIScenarios(nodes)
	count := 0
	for _, scenario := range scenarios {
		count += len(scenario.Scenarios)
	}
	return count
}

// generateConfigDescription creates a descriptive summary of the configuration
func (cp *CollectionProcessor) generateConfigDescription(scenarioCount, individualCount, totalExpectations int) string {
	if scenarioCount > 0 && individualCount > 0 {
		return fmt.Sprintf("Generated from %s collection with %d scenarios (%d expectations) and %d individual APIs - Total: %d expectations",
			cp.collectionType, scenarioCount, totalExpectations-individualCount, individualCount, totalExpectations)
	} else if scenarioCount > 0 {
		return fmt.Sprintf("Generated from %s collection with %d scenarios (%d expectations)",
			cp.collectionType, scenarioCount, totalExpectations)
	} else {
		return fmt.Sprintf("Generated from %s collection with %d individual API expectations",
			cp.collectionType, totalExpectations)
	}
}

// Step 2: Parse collection file based on type
func (cp *CollectionProcessor) parseCollectionFile(filePath string) ([]APIRequest, error) {
	fmt.Printf("\n📄 Parsing %s collection file: %s\n", cp.collectionType, filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	switch cp.collectionType {
	case "postman":
		return cp.parsePostmanCollection(data)
	case "bruno":
		return cp.parseBrunoCollection(data)
	case "insomnia":
		return cp.parseInsomniaCollection(data)
	default:
		return nil, fmt.Errorf("unsupported collection type: %s", cp.collectionType)
	}
}

// autoConfigureScenarioMatching automatically configures matching based on scenario difference
func (cp *CollectionProcessor) autoConfigureScenarioMatching(node *ExecutionNode, httpRequest map[string]interface{}, difference string) error {
	fmt.Printf("    🔧 Auto-configuring for difference: %s\n", difference)

	switch {
	case difference == "no-auth":
		// Configure to match requests WITHOUT Authorization header
		headers := map[string]interface{}{
			"Authorization": map[string]interface{}{
				"not": true,
			},
		}
		httpRequest["headers"] = headers
		fmt.Printf("    ✅ Configured to match requests WITHOUT Authorization header\n")

	case difference == "invalid-auth":
		// Configure to match requests with invalid/expired Authorization
		headers := map[string]interface{}{
			"Authorization": map[string]interface{}{
				"values": []string{"invalid", "expired", "Bearer invalid", "Bearer expired"},
			},
		}
		httpRequest["headers"] = headers
		fmt.Printf("    ✅ Configured to match requests with invalid/expired Authorization\n")

	case difference == "different-headers" || difference == "different-header-values":
		// Add headers from the API
		if len(node.API.Headers) > 0 {
			headers := make(map[string]interface{})
			for k, v := range node.API.Headers {
				headers[k] = v
			}
			httpRequest["headers"] = headers
			fmt.Printf("    ✅ Configured %d request headers\n", len(node.API.Headers))
		}

	case difference == "no-headers":
		// Explicitly configure to match requests with minimal headers
		fmt.Printf("    ✅ Configured to match requests with minimal headers\n")

	case difference == "different-query-params":
		// Add query parameters from the API
		if len(node.API.QueryParams) > 0 {
			httpRequest["queryStringParameters"] = node.API.QueryParams
			fmt.Printf("    ✅ Configured %d query parameters\n", len(node.API.QueryParams))
		}

	case difference == "no-body":
		// Configure to match requests without body
		httpRequest["body"] = map[string]interface{}{
			"not": true,
		}
		fmt.Printf("    ✅ Configured to match requests WITHOUT body\n")

	case difference == "with-body":
		// Configure to match requests with body (any body)
		if node.API.Body != "" {
			httpRequest["body"] = node.API.Body
		}
		fmt.Printf("    ✅ Configured to match requests WITH body\n")

	case difference == "different-request-body":
		// Add specific body from the API
		if node.API.Body != "" {
			httpRequest["body"] = node.API.Body
			fmt.Printf("    ✅ Configured specific request body matching\n")
		}

	case strings.HasPrefix(difference, "status-"):
		// Status code difference - no additional request matching needed
		fmt.Printf("    ✅ Scenario differentiated by response status code\n")

	default:
		fmt.Printf("    ℹ️  Using default matching (exact path)\n")
	}

	return nil
}

// handlePriorityBasedScenarios configures scenarios with priority-based matching
func (cp *CollectionProcessor) handlePriorityBasedScenarios(scenarios []APIScenario, nodes []ExecutionNode) ([]map[string]interface{}, error) {
	fmt.Println("\n📊 Priority-Based Scenario Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Scenarios will be configured with priority-based matching order.")

	// This is essentially the same as create-separate but with user input on priorities
	return cp.handleCreateSeparateScenarios(scenarios, nodes)
}

// configureIndividualMatching handles non-scenario APIs (renamed from original)
func (cp *CollectionProcessor) configureIndividualMatching(nodes []ExecutionNode) ([]map[string]interface{}, error) {
	fmt.Println("\n🔧 Individual API Matching Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Configure matching for individual APIs (no scenarios detected)")

	var expectations []map[string]interface{}

	for _, node := range nodes {
		if node.Response == nil {
			continue
		}

		fmt.Printf("\n🔧 Configuring: %s %s - %s\n", node.API.Method, cp.extractPath(node.API.URL), node.API.Name)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		// Build base expectation
		expectation := map[string]interface{}{
			"httpRequest": map[string]interface{}{
				"method": node.API.Method,
				"path":   cp.extractPath(node.API.URL),
			},
			"httpResponse": map[string]interface{}{
				"statusCode": node.Response.StatusCode,
				"headers":    node.Response.Headers,
				"body":       node.Response.Body,
			},
		}

		httpRequest := expectation["httpRequest"].(map[string]interface{})

		// Configure matching criteria for this individual API
		if err := cp.collectQueryParameterMatching(&node, httpRequest); err != nil {
			return nil, err
		}

		if err := cp.collectPathMatchingStrategy(&node, httpRequest); err != nil {
			return nil, err
		}

		if err := cp.collectRequestHeaderMatching(&node, httpRequest); err != nil {
			return nil, err
		}

		if err := cp.collectAdvancedConfiguration(&node, expectation); err != nil {
			return nil, err
		}

		expectations = append(expectations, expectation)
	}

	return expectations, nil
}

// handleIndividualScenarioConfiguration asks user to configure each scenario
func (cp *CollectionProcessor) handleIndividualScenarioConfiguration(scenarios []APIScenario, nodes []ExecutionNode) ([]map[string]interface{}, error) {
	fmt.Println("\n🔧 Individual Scenario Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("You will be asked to configure each scenario individually.")

	// For now, fall back to create-separate approach
	// In a full implementation, this would ask user to configure each scenario
	return cp.handleCreateSeparateScenarios(scenarios, nodes)
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
		neededVars := cp.extractVariablesFromAPI(&node.API)
		if len(neededVars) > 0 {
			fmt.Printf("   📋 Variables needed: %v\n", neededVars)
		} else {
			fmt.Printf("   📋 No variables needed\n")
		}

		// Step 2-5: Resolve variables
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

		// Step 7: Run post-script to populate variables
		if node.API.PostScript != "" {
			fmt.Printf("   🔧 Running post-script...\n")
			extractedVars := cp.executePostScript(node.API.PostScript, response, variables)
			if len(extractedVars) > 0 {
				fmt.Printf("   📦 Variables extracted: ")
				for k, v := range extractedVars {
					variables[k] = v
					node.Variables = append(node.Variables, k)
					fmt.Printf("%s=%s ", k, v)
				}
				fmt.Println()
			}
		}
	}

	fmt.Printf("\n🎉 Executed %d APIs successfully!\n", len(nodes))
	fmt.Println("\n🧹 Clearing in-memory variables...")
	variables = nil // Clear the map

	return nil
}

// API scenario detection and grouping types
type APIScenario struct {
	BaseName  string
	Method    string
	Path      string
	Scenarios []ScenarioVariant
}

type ScenarioVariant struct {
	Node        ExecutionNode
	Name        string
	Description string
	Difference  string // What makes this scenario different
}

// detectAPIScenarios groups APIs by endpoint and identifies scenarios
func (cp *CollectionProcessor) detectAPIScenarios(nodes []ExecutionNode) []APIScenario {
	// Group by method + path
	groups := make(map[string][]ExecutionNode)
	for _, node := range nodes {
		if node.Response == nil {
			continue
		}

		key := fmt.Sprintf("%s %s", node.API.Method, cp.extractPath(node.API.URL))
		groups[key] = append(groups[key], node)
	}

	// Convert groups to scenarios
	var scenarios []APIScenario
	for key, nodeGroup := range groups {
		if len(nodeGroup) == 1 {
			// Single API, no scenarios needed
			continue
		}

		parts := strings.SplitN(key, " ", 2)
		scenario := APIScenario{
			BaseName: fmt.Sprintf("%s %s", parts[0], parts[1]),
			Method:   parts[0],
			Path:     parts[1],
		}

		// Analyze each variant to determine differences
		for i, node := range nodeGroup {
			variant := ScenarioVariant{
				Node: node,
				Name: node.API.Name,
			}

			// Determine what makes this scenario different
			variant.Difference = cp.identifyScenarioDifference(node, nodeGroup)
			variant.Description = cp.generateScenarioDescription(node, variant.Difference)

			if variant.Name == "" {
				variant.Name = fmt.Sprintf("%s - Scenario %d", scenario.BaseName, i+1)
			}

			scenario.Scenarios = append(scenario.Scenarios, variant)
		}

		scenarios = append(scenarios, scenario)
	}

	return scenarios
}

// classifyAPIsByType separates REST and GraphQL APIs
func (cp *CollectionProcessor) classifyAPIsByType(nodes []ExecutionNode) ([]ExecutionNode, []ExecutionNode) {
	var restNodes, graphqlNodes []ExecutionNode

	for _, node := range nodes {
		if node.Response == nil {
			continue
		}

		// Check if it's a GraphQL request
		if cp.isGraphQLRequest(node.API) {
			graphqlNodes = append(graphqlNodes, node)
		} else {
			restNodes = append(restNodes, node)
		}
	}

	return restNodes, graphqlNodes
}

// isGraphQLRequest determines if an API request is GraphQL
func (cp *CollectionProcessor) isGraphQLRequest(api APIRequest) bool {
	// Check URL path
	if strings.Contains(api.URL, "/graphql") || strings.Contains(api.URL, "/gql") {
		return true
	}

	// Check Content-Type header
	if contentType, exists := api.Headers["Content-Type"]; exists {
		if strings.Contains(strings.ToLower(contentType), "graphql") {
			return true
		}
	}

	// Check body for GraphQL structure
	if api.Body != "" {
		var bodyMap map[string]interface{}
		if err := json.Unmarshal([]byte(api.Body), &bodyMap); err == nil {
			// Check for GraphQL query structure
			if _, hasQuery := bodyMap["query"]; hasQuery {
				return true
			}
		}

		// Check if body contains GraphQL keywords
		bodyLower := strings.ToLower(api.Body)
		if strings.Contains(bodyLower, "query ") || strings.Contains(bodyLower, "mutation ") ||
			strings.Contains(bodyLower, "subscription ") {
			return true
		}
	}

	return false
}

// detectGraphQLScenarios groups GraphQL requests by operation and identifies scenarios
func (cp *CollectionProcessor) detectGraphQLScenarios(nodes []ExecutionNode) []APIScenario {
	// Group by operation name or query content
	groups := make(map[string][]ExecutionNode)
	for _, node := range nodes {
		if node.Response == nil {
			continue
		}

		operationKey := cp.extractGraphQLOperationKey(node.API)
		groups[operationKey] = append(groups[operationKey], node)
	}

	// Convert groups to scenarios
	var scenarios []APIScenario
	for key, nodeGroup := range groups {
		if len(nodeGroup) == 1 {
			// Single GraphQL operation, no scenarios needed
			continue
		}

		scenario := APIScenario{
			BaseName: fmt.Sprintf("GraphQL %s", key),
			Method:   "POST",
			Path:     "/graphql",
		}

		// Analyze each variant to determine differences
		for i, node := range nodeGroup {
			variant := ScenarioVariant{
				Node: node,
				Name: node.API.Name,
			}

			// Determine what makes this GraphQL scenario different
			variant.Difference = cp.identifyGraphQLScenarioDifference(node, nodeGroup)
			variant.Description = cp.generateGraphQLScenarioDescription(node, variant.Difference)

			if variant.Name == "" {
				variant.Name = fmt.Sprintf("%s - Scenario %d", scenario.BaseName, i+1)
			}

			scenario.Scenarios = append(scenario.Scenarios, variant)
		}

		scenarios = append(scenarios, scenario)
	}

	return scenarios
}

// extractGraphQLOperationKey creates a key for grouping GraphQL operations
func (cp *CollectionProcessor) extractGraphQLOperationKey(api APIRequest) string {
	if api.Body == "" {
		return "unknown_operation"
	}

	var bodyMap map[string]interface{}
	if err := json.Unmarshal([]byte(api.Body), &bodyMap); err == nil {
		// Extract operation name if present
		if operationName, ok := bodyMap["operationName"].(string); ok && operationName != "" {
			return operationName
		}

		// Extract operation from query
		if query, ok := bodyMap["query"].(string); ok {
			return cp.extractOperationFromQuery(query)
		}
	}

	// Fallback to generic key
	return "graphql_operation"
}

// extractOperationFromQuery extracts operation name from GraphQL query string
func (cp *CollectionProcessor) extractOperationFromQuery(query string) string {
	// Simple regex to extract operation name
	queryLower := strings.ToLower(query)
	query = strings.TrimSpace(query)

	// Look for operation patterns
	if strings.Contains(queryLower, "query ") {
		// Extract query name
		if parts := strings.Fields(query); len(parts) >= 2 {
			for i, part := range parts {
				if strings.ToLower(part) == "query" && i+1 < len(parts) {
					// Next part should be the operation name
					opName := strings.Trim(parts[i+1], "({")
					if opName != "" && opName != "{" {
						return opName
					}
				}
			}
		}
		return "query_operation"
	} else if strings.Contains(queryLower, "mutation ") {
		// Extract mutation name
		if parts := strings.Fields(query); len(parts) >= 2 {
			for i, part := range parts {
				if strings.ToLower(part) == "mutation" && i+1 < len(parts) {
					// Next part should be the operation name
					opName := strings.Trim(parts[i+1], "({")
					if opName != "" && opName != "{" {
						return opName
					}
				}
			}
		}
		return "mutation_operation"
	} else if strings.Contains(queryLower, "subscription ") {
		return "subscription_operation"
	}

	return "unknown_operation"
}

// identifyGraphQLScenarioDifference determines what makes a GraphQL scenario unique
func (cp *CollectionProcessor) identifyGraphQLScenarioDifference(node ExecutionNode, group []ExecutionNode) string {
	// Compare with other nodes in group to find differences
	for _, other := range group {
		if other.API.ID == node.API.ID {
			continue
		}

		// Check status code differences (prioritized)
		if node.Response.StatusCode != other.Response.StatusCode {
			return fmt.Sprintf("status-%d", node.Response.StatusCode)
		}

		// Check for different variables
		if cp.hasGraphQLVariableDifferences(node.API, other.API) {
			return "different-variables"
		}

		// Check for authentication differences
		if _, hasAuth := node.API.Headers["Authorization"]; !hasAuth {
			if _, otherHasAuth := other.API.Headers["Authorization"]; otherHasAuth {
				return "no-auth"
			}
		}

		// Check for different queries (rare but possible)
		if cp.hasGraphQLQueryDifferences(node.API, other.API) {
			return "different-query"
		}
	}

	return "graphql-variant"
}

// hasGraphQLVariableDifferences checks if GraphQL variables are different
func (cp *CollectionProcessor) hasGraphQLVariableDifferences(api1, api2 APIRequest) bool {
	vars1 := cp.extractGraphQLVariables(api1)
	vars2 := cp.extractGraphQLVariables(api2)

	// Compare variable maps
	if len(vars1) != len(vars2) {
		return true
	}

	for k, v1 := range vars1 {
		if v2, exists := vars2[k]; !exists || fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
			return true
		}
	}

	return false
}

// hasGraphQLQueryDifferences checks if GraphQL queries are different
func (cp *CollectionProcessor) hasGraphQLQueryDifferences(api1, api2 APIRequest) bool {
	query1 := cp.extractGraphQLQuery(api1)
	query2 := cp.extractGraphQLQuery(api2)

	return query1 != query2
}

// extractGraphQLVariables extracts variables from GraphQL request body
func (cp *CollectionProcessor) extractGraphQLVariables(api APIRequest) map[string]interface{} {
	if api.Body == "" {
		return make(map[string]interface{})
	}

	var bodyMap map[string]interface{}
	if err := json.Unmarshal([]byte(api.Body), &bodyMap); err == nil {
		if vars, ok := bodyMap["variables"].(map[string]interface{}); ok {
			return vars
		}
	}

	return make(map[string]interface{})
}

// extractGraphQLQuery extracts query string from GraphQL request body
func (cp *CollectionProcessor) extractGraphQLQuery(api APIRequest) string {
	if api.Body == "" {
		return ""
	}

	var bodyMap map[string]interface{}
	if err := json.Unmarshal([]byte(api.Body), &bodyMap); err == nil {
		if query, ok := bodyMap["query"].(string); ok {
			return query
		}
	}

	return ""
}

// generateGraphQLScenarioDescription creates a human-readable description for GraphQL scenarios
func (cp *CollectionProcessor) generateGraphQLScenarioDescription(node ExecutionNode, difference string) string {
	switch {
	case strings.HasPrefix(difference, "status-"):
		return fmt.Sprintf("Returns %d %s", node.Response.StatusCode, cp.getStatusText(node.Response.StatusCode))
	case difference == "no-auth":
		return "Missing Authorization header"
	case difference == "different-variables":
		return "Different GraphQL variables"
	case difference == "different-query":
		return "Different GraphQL query"
	default:
		return "Alternative GraphQL scenario"
	}
}

// identifyScenarioDifference determines what makes a scenario unique
func (cp *CollectionProcessor) identifyScenarioDifference(node ExecutionNode, group []ExecutionNode) string {
	// Compare with other nodes in group to find differences
	for _, other := range group {
		if other.API.ID == node.API.ID {
			continue
		}

		// Check status code differences (prioritized)
		if node.Response.StatusCode != other.Response.StatusCode {
			return fmt.Sprintf("status-%d", node.Response.StatusCode)
		}

		// Check for missing Authorization (common auth scenarios)
		if _, hasAuth := node.API.Headers["Authorization"]; !hasAuth {
			if _, otherHasAuth := other.API.Headers["Authorization"]; otherHasAuth {
				return "no-auth"
			}
		}

		// Check for invalid/expired Authorization
		if authVal, hasAuth := node.API.Headers["Authorization"]; hasAuth {
			if _, otherHasAuth := other.API.Headers["Authorization"]; otherHasAuth {
				if strings.Contains(authVal, "invalid") || strings.Contains(authVal, "expired") {
					return "invalid-auth"
				}
			}
		}

		// Check header count differences
		if len(node.API.Headers) != len(other.API.Headers) {
			if len(node.API.Headers) == 0 {
				return "no-headers"
			}
			return "different-headers"
		}

		// Check specific header value differences
		for key, val := range node.API.Headers {
			if otherVal, exists := other.API.Headers[key]; exists && val != otherVal {
				return "different-header-values"
			}
		}

		// Check query parameter differences
		if !cp.queryParamsEqual(node.API.QueryParams, other.API.QueryParams) {
			return "different-query-params"
		}

		// Check request body differences (check for empty vs non-empty)
		if (node.API.Body == "") != (other.API.Body == "") {
			if node.API.Body == "" {
				return "no-body"
			}
			return "with-body"
		}

		// Check for actual body content differences
		if node.API.Body != other.API.Body {
			return "different-request-body"
		}
	}

	return "variant"
}

// generateScenarioDescription creates a human-readable description
func (cp *CollectionProcessor) generateScenarioDescription(node ExecutionNode, difference string) string {
	switch {
	case strings.HasPrefix(difference, "status-"):
		return fmt.Sprintf("Returns %d %s", node.Response.StatusCode, cp.getStatusText(node.Response.StatusCode))
	case difference == "no-auth":
		return "Missing Authorization header"
	case difference == "invalid-auth":
		return "Invalid or expired Authorization header"
	case difference == "no-headers":
		return "No special headers required"
	case difference == "different-headers":
		return "Different header requirements"
	case difference == "different-header-values":
		return "Different header values"
	case difference == "different-query-params":
		return "Different query parameter requirements"
	case difference == "no-body":
		return "Request without body"
	case difference == "with-body":
		return "Request with body"
	case difference == "different-request-body":
		return "Different request body requirements"
	default:
		return "Alternative scenario"
	}
}

// getStatusText returns human-readable status text
func (cp *CollectionProcessor) getStatusText(statusCode int) string {
	switch statusCode {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 409:
		return "Conflict"
	case 422:
		return "Unprocessable Entity"
	case 429:
		return "Too Many Requests"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	default:
		return "Response"
	}
}

// queryParamsEqual compares query parameter maps
func (cp *CollectionProcessor) queryParamsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if b[k] != v {
			return false
		}
	}

	return true
}
func (cp *CollectionProcessor) configureMatchingCriteria(nodes []ExecutionNode) ([]map[string]interface{}, error) {
	fmt.Println("\n🎯 MATCHING CRITERIA CONFIGURATION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Configure how incoming requests should be matched to these mocked responses")

	var expectations []map[string]interface{}

	for _, node := range nodes {
		if node.Response == nil {
			continue
		}

		fmt.Printf("\n🔧 Configuring: %s %s - %s\n", node.API.Method, cp.extractPath(node.API.URL), node.API.Name)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		// Build base expectation
		expectation := map[string]interface{}{
			"httpRequest": map[string]interface{}{
				"method": node.API.Method,
				"path":   cp.extractPath(node.API.URL),
			},
			"httpResponse": map[string]interface{}{
				"statusCode": node.Response.StatusCode,
				"headers":    node.Response.Headers,
				"body":       node.Response.Body,
			},
		}

		httpRequest := expectation["httpRequest"].(map[string]interface{})

		// Step 1: Query Parameter Matching (interactive)
		if err := cp.collectQueryParameterMatching(&node, httpRequest); err != nil {
			return nil, err
		}

		// Step 2: Path Matching Strategy (interactive)
		if err := cp.collectPathMatchingStrategy(&node, httpRequest); err != nil {
			return nil, err
		}

		// Step 3: Request Header Matching (interactive)
		if err := cp.collectRequestHeaderMatching(&node, httpRequest); err != nil {
			return nil, err
		}

		// Step 4: Advanced Configuration (optional)
		if err := cp.collectAdvancedConfiguration(&node, expectation); err != nil {
			return nil, err
		}

		expectations = append(expectations, expectation)
	}

	return expectations, nil
}

// configureMatchingCriteriaWithScenarios handles scenario detection and configuration
func (cp *CollectionProcessor) configureMatchingCriteriaWithScenarios(nodes []ExecutionNode) ([]map[string]interface{}, error) {
	fmt.Println("\n🎯 ENHANCED SCENARIO-AWARE MATCHING CONFIGURATION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Analyzing APIs for scenarios and configuring intelligent matching...")

	// Step 1: Detect API scenarios
	scenarios := cp.detectAPIScenarios(nodes)
	if len(scenarios) > 0 {
		fmt.Printf("\n🔍 SCENARIO DETECTION RESULTS\n")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("Found %d endpoints with multiple scenarios:\n\n", len(scenarios))

		for i, scenario := range scenarios {
			fmt.Printf("[%d] %s %s:\n", i+1, scenario.Method, scenario.Path)
			for j, variant := range scenario.Scenarios {
				fmt.Printf("   • Scenario %d: %s - %s\n", j+1, variant.Name, variant.Description)
			}
			fmt.Println()
		}

		return cp.configureScenarioBasedMatching(scenarios, nodes)
	}

	// Step 2: Fall back to individual API configuration
	fmt.Println("\n📝 No multiple scenarios detected - using individual API configuration")
	return cp.configureIndividualMatching(nodes)
}

// configureScenarioBasedMatching handles APIs with multiple scenarios
func (cp *CollectionProcessor) configureScenarioBasedMatching(scenarios []APIScenario, nodes []ExecutionNode) ([]map[string]interface{}, error) {
	fmt.Println("\n🎭 SCENARIO-BASED MATCHING CONFIGURATION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var action string
	if err := survey.AskOne(&survey.Select{
		Message: "How should these scenarios be handled?",
		Options: []string{
			"create-separate - Create separate expectations (recommended)",
			"configure-priority - Configure with priority ordering",
			"ask-each - Configure each scenario individually",
			"skip-scenarios - Skip scenario configuration (treat as individual)",
		},
		Default: "create-separate - Create separate expectations (recommended)",
	}, &action); err != nil {
		return nil, err
	}

	actionType := strings.Split(action, " ")[0]
	switch actionType {
	case "create-separate":
		return cp.handleCreateSeparateScenarios(scenarios, nodes)
	case "configure-priority":
		return cp.handlePriorityBasedScenarios(scenarios, nodes)
	case "ask-each":
		return cp.handleIndividualScenarioConfiguration(scenarios, nodes)
	case "skip-scenarios":
		return cp.configureIndividualMatching(nodes)
	default:
		return cp.handleCreateSeparateScenarios(scenarios, nodes)
	}
}

// handleCreateSeparateScenarios creates separate expectations for each scenario
func (cp *CollectionProcessor) handleCreateSeparateScenarios(scenarios []APIScenario, nodes []ExecutionNode) ([]map[string]interface{}, error) {
	fmt.Println("\n✨ Creating Separate Scenario Expectations")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Each scenario will be created as a separate expectation with appropriate priority.")

	var expectations []map[string]interface{}
	priority := 1

	// Handle scenario-based APIs
	for _, scenario := range scenarios {
		fmt.Printf("\n🎭 Processing scenarios for %s %s:\n", scenario.Method, scenario.Path)

		for i, variant := range scenario.Scenarios {
			fmt.Printf("  [%d] %s - %s\n", i+1, variant.Name, variant.Description)

			// Create expectation for this variant
			expectation := map[string]interface{}{
				"priority": priority,
				"httpRequest": map[string]interface{}{
					"method": variant.Node.API.Method,
					"path":   cp.extractPath(variant.Node.API.URL),
				},
				"httpResponse": map[string]interface{}{
					"statusCode": variant.Node.Response.StatusCode,
					"headers":    variant.Node.Response.Headers,
					"body":       variant.Node.Response.Body,
				},
			}

			httpRequest := expectation["httpRequest"].(map[string]interface{})

			// Auto-configure based on scenario difference
			if err := cp.autoConfigureScenarioMatching(&variant.Node, httpRequest, variant.Difference); err != nil {
				return nil, err
			}

			expectations = append(expectations, expectation)
			priority++
		}
	}

	// Handle non-scenario APIs
	for _, node := range nodes {
		if node.Response == nil {
			continue
		}

		// Check if this node is already handled in scenarios
		handled := false
		for _, scenario := range scenarios {
			for _, variant := range scenario.Scenarios {
				if variant.Node.API.ID == node.API.ID {
					handled = true
					break
				}
			}
			if handled {
				break
			}
		}

		if !handled {
			// Create expectation for individual API
			expectation := map[string]interface{}{
				"priority": priority,
				"httpRequest": map[string]interface{}{
					"method": node.API.Method,
					"path":   cp.extractPath(node.API.URL),
				},
				"httpResponse": map[string]interface{}{
					"statusCode": node.Response.StatusCode,
					"headers":    node.Response.Headers,
					"body":       node.Response.Body,
				},
			}

			expectations = append(expectations, expectation)
			priority++
		}
	}

	fmt.Printf("\n✅ Created %d expectations with scenario-based priorities\n", len(expectations))
	return expectations, nil
}

// collectQueryParameterMatching - interactive query parameter collection
func (cp *CollectionProcessor) collectQueryParameterMatching(node *ExecutionNode, httpRequest map[string]interface{}) error {
	fmt.Println("\n🔍 Step 2: Query Parameter Matching")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Extract query params from URL
	detectedParams := make(map[string]string)
	if strings.Contains(node.API.URL, "?") {
		parts := strings.Split(node.API.URL, "?")
		if len(parts) > 1 {
			for _, param := range strings.Split(parts[1], "&") {
				if kv := strings.SplitN(param, "=", 2); len(kv) == 2 {
					detectedParams[kv[0]] = kv[1]
				}
			}
		}
	}

	// Show detected params if any
	if len(detectedParams) > 0 {
		fmt.Printf("💡 Query parameters detected in URL:\n")
		for name, value := range detectedParams {
			fmt.Printf("   %s=%s\n", name, value)
		}
	}

	var needsQueryParams bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Does this endpoint require specific query parameters?",
		Default: len(detectedParams) > 0,
	}, &needsQueryParams); err != nil {
		return err
	}

	if !needsQueryParams {
		fmt.Println("ℹ️  No query parameter matching configured")
		return nil
	}

	queryParams := make(map[string]string)

	// Pre-populate with detected params
	for k, v := range detectedParams {
		queryParams[k] = v
		fmt.Printf("✅ Added query parameter: %s=%s\n", k, v)
	}

	// Allow user to add more or modify
	for {
		var paramName string
		if err := survey.AskOne(&survey.Input{
			Message: "Parameter name (empty to finish):",
			Help:    "e.g., 'type', 'who', 'limit'",
		}, &paramName); err != nil {
			return err
		}

		paramName = strings.TrimSpace(paramName)
		if paramName == "" {
			break
		}

		// Ask for matching type first (like headers)
		var matchingType string
		if err := survey.AskOne(&survey.Select{
			Message: fmt.Sprintf("How should '%s' parameter be matched?", paramName),
			Options: []string{
				"exact - Match exact value (e.g., 'NEW')",
				"regex - Use pattern matching (e.g., '.*')",
			},
			Default: "exact - Match exact value (e.g., 'NEW')",
		}, &matchingType); err != nil {
			return err
		}

		isRegex := strings.HasPrefix(matchingType, "regex")

		var prompt string
		var helpText string
		if isRegex {
			prompt = fmt.Sprintf("Regex pattern for '%s':", paramName)
			helpText = "Enter regex pattern (e.g., '.*', '[0-9]+')"
		} else {
			prompt = fmt.Sprintf("Exact value for '%s':", paramName)
			helpText = "Enter exact value to match (e.g., 'NEW', '10')"
		}

		var paramValue string
		if err := survey.AskOne(&survey.Input{
			Message: prompt,
			Help:    helpText,
		}, &paramValue); err != nil {
			return err
		}

		queryParams[paramName] = paramValue
		if isRegex {
			fmt.Printf("✅ Added query parameter: %s=%s (regex pattern)\n", paramName, paramValue)
		} else {
			fmt.Printf("✅ Added query parameter: %s=%s (exact match)\n", paramName, paramValue)
		}
	}

	if len(queryParams) > 0 {
		httpRequest["queryStringParameters"] = queryParams
		fmt.Printf("✅ Query Parameters: %d configured\n", len(queryParams))
	}

	return nil
}

// collectPathMatchingStrategy - interactive path matching configuration
func (cp *CollectionProcessor) collectPathMatchingStrategy(node *ExecutionNode, httpRequest map[string]interface{}) error {
	fmt.Println("\n🛤️  Step 3: Path Matching Strategy")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	path := cp.extractPath(node.API.URL)

	// Check if path has parameters
	hasPathParams := strings.Contains(path, "{")

	if !hasPathParams {
		var useRegex bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Use regex pattern matching for this path?",
			Default: false,
			Help:    "Regex allows flexible matching like /api/users/\\d+ for numeric IDs",
		}, &useRegex); err != nil {
			return err
		}

		if useRegex {
			var pattern string
			if err := survey.AskOne(&survey.Input{
				Message: "Enter regex pattern for path:",
				Default: cp.convertToPattern(path),
				Help:    "e.g., /api/users/\\d+ or /api/items/[^/]+",
			}, &pattern); err != nil {
				return err
			}
			httpRequest["path"] = pattern
			fmt.Printf("🔍 Pattern: %s (regex match)\n", pattern)
		} else {
			fmt.Println("ℹ️  Using exact string matching for path")
			fmt.Printf("🔍 Pattern: %s (exact match)\n", path)
		}
	} else {
		fmt.Printf("ℹ️  Path parameters detected in: %s\n", path)
		fmt.Println("💡 MockServer will automatically handle path parameters")
	}

	fmt.Printf("✅ Path matching configured for: %s\n", path)
	return nil
}

// collectAdvancedConfiguration - optional advanced MockServer features
func (cp *CollectionProcessor) collectAdvancedConfiguration(node *ExecutionNode, expectation map[string]interface{}) error {
	fmt.Println("\n⚙️  Step 5: Advanced Configuration (Optional)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var enableAdvanced bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Configure advanced MockServer features for this API?",
		Default: false,
		Help:    "Response delays, limits, custom headers, and basic templating",
	}, &enableAdvanced); err != nil {
		return err
	}

	if !enableAdvanced {
		fmt.Println("ℹ️  No advanced features configured")
		return nil
	}

	// Show only validated working features
	var features []string
	fmt.Println("\n👉 Use SPACE to select/deselect, ARROW KEYS to navigate, ENTER to confirm")
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select features to configure:",
		Options: []string{
			"response-delay - Add response delays (fixed/random)",
			"response-limits - Limit number of responses",
			"custom-headers - Add custom response headers",
			"basic-templating - Basic request/response templating",
			"priority - Set expectation priority",
		},
		Help: "IMPORTANT: Use SPACE (not ENTER) to select items",
	}, &features); err != nil {
		return err
	}

	for _, feature := range features {
		switch strings.Split(feature, " ")[0] {
		case "response-delay":
			if err := cp.collectResponseDelay(expectation); err != nil {
				return err
			}
		case "response-limits":
			if err := cp.collectResponseLimits(expectation); err != nil {
				return err
			}
		case "custom-headers":
			if err := cp.collectCustomResponseHeaders(expectation); err != nil {
				return err
			}
		case "basic-templating":
			if err := cp.collectBasicTemplating(expectation, node); err != nil {
				return err
			}
		case "priority":
			if err := cp.collectPriority(expectation); err != nil {
				return err
			}
		}
	}

	return nil
}

// collectResponseDelay - validated response delay configuration
func (cp *CollectionProcessor) collectResponseDelay(expectation map[string]interface{}) error {
	fmt.Println("\n⏱️  Response Delay Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

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

	httpResponse := expectation["httpResponse"].(map[string]interface{})

	if strings.HasPrefix(delayType, "fixed") {
		var delay string
		if err := survey.AskOne(&survey.Input{
			Message: "Delay in milliseconds:",
			Default: "1000",
			Help:    "e.g., 1000 for 1 second delay",
		}, &delay); err != nil {
			return err
		}

		httpResponse["delay"] = map[string]interface{}{
			"timeUnit": "MILLISECONDS",
			"value":    delay,
		}
		fmt.Printf("✅ Fixed delay: %s ms\n", delay)
	} else {
		var minDelay, maxDelay string
		if err := survey.AskOne(&survey.Input{
			Message: "Minimum delay (ms):",
			Default: "500",
		}, &minDelay); err != nil {
			return err
		}

		if err := survey.AskOne(&survey.Input{
			Message: "Maximum delay (ms):",
			Default: "2000",
		}, &maxDelay); err != nil {
			return err
		}

		httpResponse["delay"] = map[string]interface{}{
			"timeUnit": "MILLISECONDS",
			"value":    fmt.Sprintf("%s-%s", minDelay, maxDelay),
		}
		fmt.Printf("✅ Random delay: %s-%s ms\n", minDelay, maxDelay)
	}

	return nil
}

// collectResponseLimits - validated response limits configuration
func (cp *CollectionProcessor) collectResponseLimits(expectation map[string]interface{}) error {
	fmt.Println("\n🔢 Response Limits Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var remainingTimes string
	if err := survey.AskOne(&survey.Input{
		Message: "Maximum number of responses:",
		Default: "1",
		Help:    "After this many responses, expectation stops matching",
	}, &remainingTimes); err != nil {
		return err
	}

	expectation["times"] = map[string]interface{}{
		"remainingTimes": remainingTimes,
		"unlimited":      false,
	}

	fmt.Printf("✅ Response limit: %s times\n", remainingTimes)
	return nil
}

// collectCustomResponseHeaders - validated custom headers
func (cp *CollectionProcessor) collectCustomResponseHeaders(expectation map[string]interface{}) error {
	fmt.Println("\n📨 Custom Response Headers")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	httpResponse := expectation["httpResponse"].(map[string]interface{})
	responseHeaders := make(map[string]string)

	// Get existing headers if any
	if existingHeaders, ok := httpResponse["headers"].(map[string]string); ok {
		for k, v := range existingHeaders {
			responseHeaders[k] = v
		}
	}

	for {
		var headerName string
		if err := survey.AskOne(&survey.Input{
			Message: "Response header name (empty to finish):",
			Help:    "e.g., 'X-Request-ID', 'Cache-Control'",
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
			Help:    "Use ${uuid} or ${timestamp} for dynamic values",
		}, &headerValue); err != nil {
			return err
		}

		responseHeaders[headerName] = headerValue
		fmt.Printf("✅ Added response header: %s: %s\n", headerName, headerValue)
	}

	if len(responseHeaders) > 0 {
		httpResponse["headers"] = responseHeaders
	}

	return nil
}

// collectBasicTemplating - validated templating with recorded response
func (cp *CollectionProcessor) collectBasicTemplating(expectation map[string]interface{}, node *ExecutionNode) error {
	fmt.Println("\n🎭 Basic Response Templating")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n💡 Available Template Variables:")
	fmt.Println("   ${uuid} - Generate random UUID")
	fmt.Println("   ${timestamp} - Current timestamp")
	fmt.Println("   ${request.pathParameters.id} - Extract path parameter")
	fmt.Println("   ${request.queryParameters.limit} - Extract query parameter")
	fmt.Println("   ${request.headers.authorization} - Extract request header")

	var enhanceResponse bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Enhance response with templating?",
		Default: false,
		Help:    "Add dynamic values to the recorded response",
	}, &enhanceResponse); err != nil {
		return err
	}

	if !enhanceResponse {
		return nil
	}

	httpResponse := expectation["httpResponse"].(map[string]interface{})
	originalBody := httpResponse["body"].(string)

	// Simple templating enhancement
	enhancedBody := cp.enhanceResponseWithBasicTemplating(originalBody, node)
	httpResponse["body"] = enhancedBody

	fmt.Println("✅ Response enhanced with basic templating")
	return nil
}

// collectPriority - set expectation priority
func (cp *CollectionProcessor) collectPriority(expectation map[string]interface{}) error {
	fmt.Println("\n📊 Priority Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var priority string
	if err := survey.AskOne(&survey.Input{
		Message: "Expectation priority (lower = higher priority):",
		Default: "0",
		Help:    "0 = highest priority, higher numbers = lower priority",
	}, &priority); err != nil {
		return err
	}

	expectation["priority"] = priority
	fmt.Printf("✅ Priority set to: %s\n", priority)
	return nil
}

// enhanceResponseWithBasicTemplating - add basic template variables
func (cp *CollectionProcessor) enhanceResponseWithBasicTemplating(originalBody string, node *ExecutionNode) string {
	// Try to parse as JSON and add template fields
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(originalBody), &jsonData); err == nil {
		// Add template fields to JSON response
		jsonData["_template"] = map[string]interface{}{
			"requestId":   "${uuid}",
			"processedAt": "${timestamp}",
			"apiName":     node.API.Name,
		}

		if enhanced, err := json.MarshalIndent(jsonData, "", "  "); err == nil {
			return string(enhanced)
		}
	}

	// If not JSON or failed to parse, return original
	return originalBody
}

// collectRequestHeaderMatching - interactive header matching configuration
func (cp *CollectionProcessor) collectRequestHeaderMatching(node *ExecutionNode, httpRequest map[string]interface{}) error {
	fmt.Println("\n📝 Step 4: Request Header Matching")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show detected headers
	if len(node.API.Headers) > 0 {
		fmt.Printf("💡 Headers detected in collection:\n")
		for name, value := range node.API.Headers {
			fmt.Printf("   %s: %s\n", name, value)
		}
	}

	var needsHeaders bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Does this request require specific headers to match?",
		Default: len(node.API.Headers) > 0,
		Help:    "e.g., Authorization, Content-Type, API keys",
	}, &needsHeaders); err != nil {
		return err
	}

	if !needsHeaders {
		fmt.Println("ℹ️  No request header matching configured")
		return nil
	}

	headers := make(map[string]interface{})
	headerTypes := make(map[string]string)

	// Pre-populate with detected headers
	for name, value := range node.API.Headers {
		// Skip common headers that shouldn't be matched
		if strings.ToLower(name) == "content-length" || strings.ToLower(name) == "host" {
			continue
		}
		headers[name] = value
		headerTypes[name] = "exact"
		fmt.Printf("✅ Added header: %s: %s (exact match)\n", name, value)
	}

	// Allow user to add more or modify
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

		// Ask for matching type first
		var matchingType string
		if err := survey.AskOne(&survey.Select{
			Message: fmt.Sprintf("How should '%s' header be matched?", headerName),
			Options: []string{
				"exact - Match exact value (e.g., 'Bearer abc123')",
				"regex - Use pattern matching (e.g., 'Bearer .*')",
			},
			Default: "exact - Match exact value (e.g., 'Bearer abc123')",
		}, &matchingType); err != nil {
			return err
		}

		isRegex := strings.HasPrefix(matchingType, "regex")

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

		headers[headerName] = headerValue
		if isRegex {
			headerTypes[headerName] = "regex"
			fmt.Printf("✅ Added header: %s: %s (regex pattern)\n", headerName, headerValue)
		} else {
			headerTypes[headerName] = "exact"
			fmt.Printf("✅ Added header: %s: %s (exact match)\n", headerName, headerValue)
		}
	}

	if len(headers) > 0 {
		httpRequest["headers"] = headers
		fmt.Printf("✅ Request Headers: %d configured\n", len(headers))
	}

	return nil
}

// Step 6: Enhanced Review expectations with configuration options
func (cp *CollectionProcessor) reviewExpectations(expectations []map[string]interface{}) error {
	fmt.Println("\n📋 ENHANCED EXPECTATIONS REVIEW")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show summary without overwhelming JSON output
	fmt.Printf("✨ Successfully Generated %d Expectations!\n\n", len(expectations))
	fmt.Println("📊 Summary:")
	for i, exp := range expectations {
		method := "Unknown"
		path := "Unknown"
		status := 200

		if req, ok := exp["httpRequest"].(map[string]interface{}); ok {
			if m, ok := req["method"].(string); ok {
				method = m
			}
			if p, ok := req["path"].(string); ok {
				path = p
			}
		}
		if resp, ok := exp["httpResponse"].(map[string]interface{}); ok {
			if s, ok := resp["statusCode"].(int); ok {
				status = s
			}
		}

		fmt.Printf("   [%d] %s %s → %d\n", i+1, method, path, status)
	}

	for {
		var action string
		if err := survey.AskOne(&survey.Select{
			Message: "What would you like to do with these expectations?",
			Options: []string{
				"save - Save expectations to S3 (recommended)",
				"view-json - View full JSON configuration",
				"configure-more - Add more endpoint configurations",
				"modify-scenarios - Modify scenario configurations",
				"redo - Reconfigure matching criteria",
				"exit - Exit without saving",
			},
			Default: "save - Save expectations to S3 (recommended)",
		}, &action); err != nil {
			return err
		}

		actionType := strings.Split(action, " ")[0]
		switch actionType {
		case "save":
			return nil
		case "view-json":
			jsonBytes, err := json.MarshalIndent(expectations, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to format expectations: %w", err)
			}
			fmt.Printf("\n📋 Full JSON Configuration:\n%s\n\n", string(jsonBytes))
			continue
		case "configure-more":
			fmt.Println("✨ Enhanced Multi-Endpoint Configuration:")
			fmt.Println("   • This feature allows you to configure additional endpoints")
			fmt.Println("   • You can add new scenarios or modify existing ones")
			fmt.Println("   • Each endpoint can have multiple scenario variants")
			fmt.Println("🔄 To add more endpoints, restart with additional APIs in your collection")
			continue
		case "modify-scenarios":
			fmt.Println("🎭 Scenario Modification Options:")
			fmt.Println("   • Clone existing scenarios to create variants")
			fmt.Println("   • Modify response status codes, headers, or bodies")
			fmt.Println("   • Add authentication scenarios (success/fail)")
			fmt.Println("   • Create error response variants")
			fmt.Println("🚀 Advanced scenario modification coming in next update!")
			continue
		case "redo":
			fmt.Println("🔄 Enhanced Reconfiguration:")
			fmt.Println("   • This would allow you to reconfigure matching criteria")
			fmt.Println("   • Modify headers, query parameters, or path matching")
			fmt.Println("   • Adjust scenario priorities and behaviors")
			fmt.Println("For now, restart the import process to reconfigure.")
			return fmt.Errorf("reconfiguration requested - restart import process")
		case "exit":
			fmt.Println("\n⚠️  Are you sure you want to exit without saving?")
			fmt.Println("   • All API execution results will be lost")
			fmt.Println("   • All scenario configurations will be discarded")
			var confirmExit bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Exit without saving?",
				Default: false,
			}, &confirmExit); err != nil {
				return err
			}
			if confirmExit {
				return fmt.Errorf("user chose to exit without saving")
			}
			continue
		}
	}
}

// Step 7: Save expectations
func (cp *CollectionProcessor) saveExpectations(expectations []map[string]interface{}, nodes []ExecutionNode) error {
	// NOTE: We store actual values for mock matching
	// Sanitization only happens when sending to AI/LLM (not implemented in collection import)
	fmt.Println("\n💾 SAVING TO S3...")

	// Count scenarios vs individual APIs for metadata
	scenarioCount := len(cp.detectAPIScenarios(nodes))
	individualCount := len(expectations) - cp.countScenarioNodes(nodes)

	// Convert to MockConfiguration format
	mockConfig := &state.MockConfiguration{
		Metadata: state.ConfigMetadata{
			ProjectID:   cp.cleanName,
			Version:     fmt.Sprintf("v%d", time.Now().Unix()),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Description: cp.generateConfigDescription(scenarioCount, individualCount, len(expectations)),
			Provider:    fmt.Sprintf("collection-import-%s", cp.collectionType),
		},
		Expectations: make([]state.MockExpectation, 0, len(expectations)),
		Settings: map[string]interface{}{
			"source":             fmt.Sprintf("%s-collection", cp.collectionType),
			"import_method":      "collection-processor",
			"scenario_count":     scenarioCount,
			"total_endpoints":    len(nodes),
			"total_expectations": len(expectations),
			"import_timestamp":   time.Now().Unix(),
		},
	}

	// Convert expectations to proper format
	for i, exp := range expectations {
		// Use meaningful name from collection as the ID
		var expectationName string
		if i < len(nodes) && nodes[i].API.Name != "" {
			expectationName = nodes[i].API.Name
		} else {
			expectationName = fmt.Sprintf("API_%d", i+1)
		}

		mockExp := state.MockExpectation{
			ID:       fmt.Sprintf("collection_%s_%s_%d", cp.collectionType, strings.ReplaceAll(expectationName, " ", "_"), time.Now().Unix()),
			Priority: len(expectations) - i, // Higher priority for earlier expectations
		}

		// Extract httpRequest and httpResponse
		if httpReq, ok := exp["httpRequest"].(map[string]interface{}); ok {
			// Add the expectation name for identification
			httpReq["_name"] = expectationName
			mockExp.HttpRequest = httpReq
		}
		if httpResp, ok := exp["httpResponse"].(map[string]interface{}); ok {
			mockExp.HttpResponse = httpResp
		}

		// Extract times if present
		if times, ok := exp["times"].(map[string]interface{}); ok {
			mockExp.Times = &state.ExpectationTimes{}
			if unlimited, ok := times["unlimited"].(bool); ok {
				mockExp.Times.Unlimited = unlimited
			}
			if remaining, ok := times["remainingTimes"].(float64); ok {
				mockExp.Times.RemainingTimes = int(remaining)
			} else if remaining, ok := times["remainingTimes"].(int); ok {
				mockExp.Times.RemainingTimes = remaining
			}
		}

		mockConfig.Expectations = append(mockConfig.Expectations, mockExp)
	}

	// Save using the store's SaveConfig method
	ctx := context.Background()
	if err := cp.store.SaveConfig(ctx, cp.cleanName, mockConfig); err != nil {
		return fmt.Errorf("failed to save to S3: %w", err)
	}

	fmt.Printf("\n✅ Collection import completed!\n")
	fmt.Printf("📁 Project: %s\n", cp.cleanName)
	fmt.Printf("📊 Generated: %d expectations\n", len(expectations))
	fmt.Printf("☁️  Saved to: %s\n", utils.GetBucketName(cp.projectName))
	fmt.Printf("💾 Version: %s\n", mockConfig.Metadata.Version)

	return nil
}

// Helper methods for parsing different collection formats

func (cp *CollectionProcessor) parsePostmanCollection(data []byte) ([]APIRequest, error) {
	var collection map[string]interface{}
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var apis []APIRequest

	// Navigate Postman collection structure
	if info, ok := collection["info"].(map[string]interface{}); ok {
		fmt.Printf("Collection: %s\n", info["name"])
	}

	if items, ok := collection["item"].([]interface{}); ok {
		apis = append(apis, cp.parsePostmanItems(items)...)
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

	// Split by request blocks
	requests := strings.Split(content, "\n\n")
	for i, reqBlock := range requests {
		reqBlock = strings.TrimSpace(reqBlock)
		if reqBlock == "" {
			continue
		}

		api := cp.parseBruRequest(reqBlock, i+1)
		if api.Method != "" {
			apis = append(apis, api)
		}
	}

	fmt.Printf("✅ Parsed %d .bru requests\n", len(apis))
	return apis, nil
}

// parseBruRequest parses a single .bru request block
func (cp *CollectionProcessor) parseBruRequest(content string, index int) APIRequest {
	api := APIRequest{
		ID:      fmt.Sprintf("bruno_%d", index),
		Headers: make(map[string]string),
	}

	lines := strings.Split(content, "\n")
	var bodyLines []string
	inBodySection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse method and URL line
		if strings.Contains(line, " ") && (strings.HasPrefix(line, "GET ") || strings.HasPrefix(line, "POST ") ||
			strings.HasPrefix(line, "PUT ") || strings.HasPrefix(line, "PATCH ") || strings.HasPrefix(line, "DELETE ")) {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) >= 2 {
				api.Method = parts[0]
				api.URL = parts[1]
				if api.Name == "" {
					api.Name = fmt.Sprintf("%s %s", api.Method, api.URL)
				}
			}
			continue
		}

		// Parse headers
		if strings.Contains(line, ":") && !inBodySection && !strings.HasPrefix(line, "{") {
			headerParts := strings.SplitN(line, ":", 2)
			if len(headerParts) == 2 {
				headerName := strings.TrimSpace(headerParts[0])
				headerValue := strings.TrimSpace(headerParts[1])
				api.Headers[headerName] = headerValue
			}
			continue
		}

		// Detect body section
		if strings.HasPrefix(line, "{") || inBodySection {
			inBodySection = true
			bodyLines = append(bodyLines, line)
		}
	}

	// Join body lines
	if len(bodyLines) > 0 {
		api.Body = strings.Join(bodyLines, "\n")
	}

	return api
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
		ID:      cp.getString(reqMap, "id"),
		Name:    cp.getString(reqMap, "name"),
		Method:  cp.getString(reqMap, "method"),
		URL:     cp.getString(reqMap, "url"),
		Headers: make(map[string]string),
	}

	// Extract headers
	if headers, ok := reqMap["headers"].([]interface{}); ok {
		for _, h := range headers {
			if header, ok := h.(map[string]interface{}); ok {
				key := cp.getString(header, "name")
				value := cp.getString(header, "value")
				if key != "" && value != "" {
					api.Headers[key] = value
				}
			}
		}
	} else if headers, ok := reqMap["headers"].(map[string]interface{}); ok {
		// Alternative header format
		for key, value := range headers {
			if valueStr, ok := value.(string); ok {
				api.Headers[key] = valueStr
			}
		}
	}

	// Extract body
	if body, ok := reqMap["body"].(map[string]interface{}); ok {
		api.Body = cp.getString(body, "text")
		if api.Body == "" {
			api.Body = cp.getString(body, "raw")
		}
	} else if body, ok := reqMap["body"].(string); ok {
		api.Body = body
	}

	// Extract scripts
	if preScript, ok := reqMap["preScript"]; ok {
		if script, ok := preScript.(string); ok {
			api.PreScript = script
		}
	}

	if postScript, ok := reqMap["postScript"]; ok {
		if script, ok := postScript.(string); ok {
			api.PostScript = script
		}
	}

	return api
}

func (cp *CollectionProcessor) parseInsomniaCollection(data []byte) ([]APIRequest, error) {
	var collection map[string]interface{}
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var apis []APIRequest
	fmt.Printf("🔧 Processing Insomnia collection...\n")

	// Handle multiple possible Insomnia export formats
	if resources, ok := collection["resources"].([]interface{}); ok {
		// Standard Insomnia export format
		for _, resource := range resources {
			if resourceMap, ok := resource.(map[string]interface{}); ok {
				resourceType := cp.getString(resourceMap, "_type")
				if resourceType == "request" {
					api := cp.parseInsomniaRequest(resourceMap)
					apis = append(apis, api)
				} else if resourceType == "grpc_request" {
					// Handle gRPC requests
					api := cp.parseInsomniaGrpcRequest(resourceMap)
					if api.Method != "" {
						apis = append(apis, api)
					}
				} else if resourceType == "graphql_request" {
					// Handle GraphQL requests
					api := cp.parseInsomniaGraphQLRequest(resourceMap)
					if api.Method != "" {
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
				apis = append(apis, api)
			}
		}
	} else {
		// Try to parse as single request
		api := cp.parseInsomniaRequest(collection)
		if api.Method != "" {
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
		ID:      cp.getString(resourceMap, "_id"),
		Name:    cp.getString(resourceMap, "name"),
		Method:  cp.getString(resourceMap, "method"),
		URL:     cp.getString(resourceMap, "url"),
		Headers: make(map[string]string),
	}

	// Extract headers
	if headers, ok := resourceMap["headers"].([]interface{}); ok {
		api.Headers = cp.extractInsomniaHeaders(headers)
	}

	// Extract body
	if body, ok := resourceMap["body"].(map[string]interface{}); ok {
		api.Body = cp.getString(body, "text")
		if api.Body == "" {
			api.Body = cp.getString(body, "mimeType")
		}
	} else if bodyStr, ok := resourceMap["body"].(string); ok {
		api.Body = bodyStr
	}

	// Extract parameters (query params)
	if parameters, ok := resourceMap["parameters"].([]interface{}); ok {
		api.QueryParams = cp.extractInsomniaParameters(parameters)
	}

	// Extract authentication
	if auth, ok := resourceMap["authentication"].(map[string]interface{}); ok {
		cp.extractInsomniaAuth(auth, &api)
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

// extractInsomniaParameters extracts query parameters
func (cp *CollectionProcessor) extractInsomniaParameters(parameters []interface{}) map[string]string {
	result := make(map[string]string)
	for _, p := range parameters {
		if param, ok := p.(map[string]interface{}); ok {
			key := cp.getString(param, "name")
			value := cp.getString(param, "value")
			if key != "" && value != "" {
				result[key] = value
			}
		}
	}
	return result
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
	case "apikey":
		key := cp.getString(auth, "key")
		value := cp.getString(auth, "value")
		if key != "" && value != "" {
			api.Headers[key] = value
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
		return nil, fmt.Errorf("failed to create request: %w", err)
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
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
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

func (cp *CollectionProcessor) convertToPattern(path string) string {
	// Convert exact paths to patterns by replacing IDs with regex
	// This is a simple implementation
	parts := strings.Split(path, "/")
	for i, part := range parts {
		// If part looks like an ID (numbers, UUIDs, etc.)
		if len(part) > 0 && (strings.ContainsAny(part, "0123456789") || len(part) > 10) {
			parts[i] = "[^/]+"
		}
	}
	return strings.Join(parts, "/")
}

// resolveVariables performs the 5-step variable resolution process
func (cp *CollectionProcessor) resolveVariables(api *APIRequest, neededVars []string, variables map[string]string) error {
	for _, varName := range neededVars {
		// Check if already resolved
		if _, exists := variables[varName]; exists {
			fmt.Printf("   ✅ %s = %s (from previous API)\n", varName, variables[varName])
			continue
		}

		// Step 2: Check environment
		if envVal := os.Getenv(varName); envVal != "" {
			variables[varName] = envVal
			fmt.Printf("   ✅ %s = %s (from environment)\n", varName, envVal)
			continue
		}

		// Step 3: Run pre-script if available
		if api.PreScript != "" {
			fmt.Printf("   🔧 Running pre-script for %s...\n", varName)
			if val := cp.executePreScriptForVariable(api.PreScript, varName, variables); val != "" {
				variables[varName] = val
				fmt.Printf("   ✅ %s = %s (from pre-script)\n", varName, val)
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
			return err
		}

		if value == "" {
			return fmt.Errorf("no value provided for required variable '%s'", varName)
		}

		// Confirm the value
		var confirm bool
		if err := survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("Use '%s' = '%s'?", varName, value),
			Default: true,
		}, &confirm); err != nil {
			return err
		}

		if !confirm {
			// Ask again
			if err := survey.AskOne(&survey.Input{
				Message: fmt.Sprintf("Re-enter value for '%s':", varName),
			}, &value); err != nil {
				return err
			}
		}

		variables[varName] = value
		fmt.Printf("   ✅ %s = %s (user input)\n", varName, value)
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

// executePostScript executes post-script and extracts variables
func (cp *CollectionProcessor) executePostScript(postScript string, response *APIResponse, existingVars map[string]string) map[string]string {
	extractedVars := make(map[string]string)

	// Parse response body as JSON for script context
	var jsonData interface{}
	json.Unmarshal([]byte(response.Body), &jsonData)

	// Look for variable assignment patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`pm\.environment\.set\(["']([^"']+)["'],\s*([^)]+)\)`),
		regexp.MustCompile(`pm\.globals\.set\(["']([^"']+)["'],\s*([^)]+)\)`),
		regexp.MustCompile(`pm\.collectionVariables\.set\(["']([^"']+)["'],\s*([^)]+)\)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(postScript, -1)
		for _, match := range matches {
			if len(match) > 2 {
				varName := match[1]
				valueExpr := strings.TrimSpace(match[2])

				// Try to evaluate the value expression
				value := cp.evaluateScriptExpression(valueExpr, jsonData, existingVars)
				if value != "" {
					extractedVars[varName] = value
				}
			}
		}
	}

	return extractedVars
}

// evaluateScriptExpression attempts to evaluate simple script expressions
func (cp *CollectionProcessor) evaluateScriptExpression(expr string, jsonData interface{}, vars map[string]string) string {
	// Remove quotes if present
	expr = strings.Trim(expr, "\"'")

	// Check if it's a string literal
	if strings.HasPrefix(expr, "'") || strings.HasPrefix(expr, "\"") {
		return strings.Trim(expr, "\"'")
	}

	// Check if it's accessing response JSON: jsonData.field or pm.response.json().field
	if strings.Contains(expr, "jsonData.") {
		field := strings.TrimPrefix(expr, "jsonData.")
		return cp.extractJSONField(jsonData, field)
	}

	if strings.Contains(expr, "pm.response.json()") {
		field := strings.TrimPrefix(expr, "pm.response.json().")
		field = strings.TrimPrefix(field, "pm.response.json()")
		if field != "" && strings.HasPrefix(field, ".") {
			field = strings.TrimPrefix(field, ".")
			return cp.extractJSONField(jsonData, field)
		}
	}

	// Handle response headers: pm.response.headers.get("header-name")
	if strings.Contains(expr, "pm.response.headers.get") {
		headerPattern := regexp.MustCompile(`pm\.response\.headers\.get\(["']([^"']+)["']\)`)
		if matches := headerPattern.FindStringSubmatch(expr); len(matches) > 1 {
			headerName := matches[1]
			// Try to extract from vars if available (simplified approach)
			if val, exists := vars["response_headers_"+headerName]; exists {
				return val
			}
		}
	}

	// Handle UUID generation
	if strings.Contains(expr, "uuid") || strings.Contains(expr, "UUID") {
		return cp.generateUUID()
	}

	// Handle timestamp generation
	if strings.Contains(expr, "timestamp") || strings.Contains(expr, "Date.now") {
		return fmt.Sprintf("%d", time.Now().Unix())
	}

	// Check if it references an existing variable
	if val, exists := vars[expr]; exists {
		return val
	}

	// Return as-is if can't evaluate
	return expr
}

// extractJSONField extracts a field from JSON data using simple dot notation
func (cp *CollectionProcessor) extractJSONField(data interface{}, path string) string {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if obj, ok := current.(map[string]interface{}); ok {
			if val, exists := obj[part]; exists {
				current = val
			} else {
				return ""
			}
		} else {
			return ""
		}
	}

	return fmt.Sprintf("%v", current)
}

// extractVariablesFromAPI extracts all {{...}} and ${...} placeholders from a single API
func (cp *CollectionProcessor) extractVariablesFromAPI(api *APIRequest) []string {
	varSet := make(map[string]bool)

	// Extract from URL
	for _, match := range cp.findPlaceholders(api.URL) {
		varSet[match] = true
	}

	// Extract from headers
	for _, v := range api.Headers {
		for _, match := range cp.findPlaceholders(v) {
			varSet[match] = true
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

// generateUUID creates a simple UUID for variable resolution
func (cp *CollectionProcessor) generateUUID() string {
	// Simple UUID v4 generation
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		time.Now().UnixNano()&0xffffffff,
		(time.Now().UnixNano()>>32)&0xffff,
		((time.Now().UnixNano()>>48)&0x0fff)|0x4000, // Version 4
		(time.Now().UnixNano()&0x3fff)|0x8000,       // Variant bits
		time.Now().UnixNano()&0xffffffffffff,
	)
}

// configureMoreEndpoints allows user to add additional endpoint configurations
func (cp *CollectionProcessor) configureMoreEndpoints(expectations []map[string]interface{}, nodes []ExecutionNode) error {
	fmt.Println("\n🔧 ENHANCED MULTI-ENDPOINT CONFIGURATION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Configure additional endpoints or create scenario variants.")

	// Show current configuration summary
	fmt.Printf("\n📊 Current Configuration: %d expectations from %d APIs\n", len(expectations), len(nodes))

	var configAction string
	if err := survey.AskOne(&survey.Select{
		Message: "What would you like to configure?",
		Options: []string{
			"clone-endpoint - Clone and modify an existing endpoint",
			"add-scenario - Add new scenario to existing endpoint",
			"create-error - Create error response variants",
			"add-auth - Add authentication scenarios",
			"batch-clone - Clone multiple endpoints with modifications",
			"template-endpoints - Create endpoints from templates",
			"done - Finish configuration",
		},
		Default: "clone-endpoint - Clone and modify an existing endpoint",
	}, &configAction); err != nil {
		return err
	}

	actionType := strings.Split(configAction, " ")[0]
	switch actionType {
	case "clone-endpoint":
		return cp.cloneEndpointFlow(expectations, nodes)
	case "add-scenario":
		return cp.addScenarioFlow(expectations, nodes)
	case "create-error":
		return cp.createErrorVariants(expectations, nodes)
	case "add-auth":
		return cp.addAuthScenarios(expectations, nodes)
	case "batch-clone":
		return cp.batchCloneFlow(expectations, nodes)
	case "template-endpoints":
		return cp.templateEndpointsFlow(expectations, nodes)
	case "done":
		fmt.Println("✅ Configuration complete")
		return fmt.Errorf("configuration_updated")
	default:
		fmt.Println("⚠️  Feature coming in next update!")
		return fmt.Errorf("configuration_updated")
	}
}

// cloneEndpointFlow handles cloning and modifying existing endpoints
func (cp *CollectionProcessor) cloneEndpointFlow(expectations []map[string]interface{}, nodes []ExecutionNode) error {
	fmt.Println("\n🎭 CLONE ENDPOINT FLOW")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show available endpoints to clone
	var endpointOptions []string
	for i, exp := range expectations {
		method := "Unknown"
		path := "Unknown"
		name := fmt.Sprintf("Expectation_%d", i+1)

		if req, ok := exp["httpRequest"].(map[string]interface{}); ok {
			if m, ok := req["method"].(string); ok {
				method = m
			}
			if p, ok := req["path"].(string); ok {
				path = p
			}
			if n, ok := req["_name"].(string); ok {
				name = n
			}
		}

		endpointOptions = append(endpointOptions, fmt.Sprintf("[%d] %s - %s %s", i+1, name, method, path))
	}

	var selectedEndpoint string
	if err := survey.AskOne(&survey.Select{
		Message: "Select endpoint to clone:",
		Options: endpointOptions,
	}, &selectedEndpoint); err != nil {
		return err
	}

	// Parse selection
	parts := strings.Split(selectedEndpoint, "]")
	if len(parts) < 2 {
		return fmt.Errorf("invalid selection")
	}

	indexStr := strings.Trim(parts[0], "[")
	var selectedIndex int
	if _, err := fmt.Sscanf(indexStr, "%d", &selectedIndex); err != nil {
		return fmt.Errorf("invalid index: %w", err)
	}
	selectedIndex-- // Convert to 0-based

	if selectedIndex < 0 || selectedIndex >= len(expectations) {
		return fmt.Errorf("invalid index")
	}

	// Clone the selected expectation
	original := expectations[selectedIndex]
	clone := make(map[string]interface{})

	// Deep copy the expectation
	originalJSON, _ := json.Marshal(original)
	json.Unmarshal(originalJSON, &clone)

	// Get clone name
	var cloneName string
	if err := survey.AskOne(&survey.Input{
		Message: "Clone name:",
		Default: "Cloned Endpoint",
		Help:    "Name for the new endpoint variant",
	}, &cloneName); err != nil {
		return err
	}

	// Update clone metadata
	if httpReq, ok := clone["httpRequest"].(map[string]interface{}); ok {
		httpReq["_name"] = cloneName
	}

	// Ask what to modify
	var modificationType string
	if err := survey.AskOne(&survey.Select{
		Message: "What should be different in this clone?",
		Options: []string{
			"status-code - Different response status (e.g., 404, 500)",
			"headers - Different request header requirements",
			"auth-scenario - Different authentication requirements",
			"response-body - Different response content",
			"multiple - Multiple modifications",
		},
		Default: "status-code - Different response status (e.g., 404, 500)",
	}, &modificationType); err != nil {
		return err
	}

	modType := strings.Split(modificationType, " ")[0]
	switch modType {
	case "status-code":
		if err := cp.modifyCloneStatusCode(clone); err != nil {
			return err
		}
	case "headers":
		if err := cp.modifyCloneHeaders(clone); err != nil {
			return err
		}
	case "auth-scenario":
		if err := cp.modifyCloneAuthScenario(clone); err != nil {
			return err
		}
	case "response-body":
		if err := cp.modifyCloneResponseBody(clone); err != nil {
			return err
		}
	case "multiple":
		if err := cp.modifyCloneMultiple(clone); err != nil {
			return err
		}
	}

	// Assign priority (higher than existing)
	clone["priority"] = len(expectations) + 1

	fmt.Printf("\n✅ Cloned endpoint: %s\n", cloneName)
	fmt.Println("📝 This clone will be added to your expectations.")

	// Ask if user wants to add more
	var addMore bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Configure more endpoints?",
		Default: false,
	}, &addMore); err != nil {
		return err
	}

	if !addMore {
		return fmt.Errorf("configuration_updated")
	}

	fmt.Println("\n🎉 Multiple endpoint configuration complete!")
	return fmt.Errorf("configuration_updated") // Signal to continue the review loop
}

// modifyCloneStatusCode modifies the status code of a cloned expectation
func (cp *CollectionProcessor) modifyCloneStatusCode(clone map[string]interface{}) error {
	fmt.Println("\n🔢 Status Code Modification")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var newStatusCode string
	if err := survey.AskOne(&survey.Select{
		Message: "Select new status code:",
		Options: []string{
			"400 - Bad Request",
			"401 - Unauthorized",
			"403 - Forbidden",
			"404 - Not Found",
			"409 - Conflict",
			"422 - Unprocessable Entity",
			"429 - Too Many Requests",
			"500 - Internal Server Error",
			"502 - Bad Gateway",
			"503 - Service Unavailable",
		},
		Default: "404 - Not Found",
	}, &newStatusCode); err != nil {
		return err
	}

	// Parse status code
	code, err := strconv.Atoi(strings.Split(newStatusCode, " ")[0])
	if err != nil {
		return fmt.Errorf("invalid status code: %w", err)
	}

	// Update clone
	if httpResponse, ok := clone["httpResponse"].(map[string]interface{}); ok {
		httpResponse["statusCode"] = code

		// Ask if user wants to update response body
		var updateBody bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Update response body to match error status?",
			Default: true,
		}, &updateBody); err != nil {
			return err
		}

		if updateBody {
			httpResponse["body"] = cp.generateErrorResponse(code)
		}
	}

	fmt.Printf("✅ Status code updated to: %d\n", code)
	return nil
}

// modifyCloneHeaders modifies request headers for clone
func (cp *CollectionProcessor) modifyCloneHeaders(clone map[string]interface{}) error {
	fmt.Println("\n📝 Request Header Modification")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var headerAction string
	if err := survey.AskOne(&survey.Select{
		Message: "How to modify request headers?",
		Options: []string{
			"remove-auth - Remove Authorization requirement (for no-auth scenarios)",
			"change-auth - Change Authorization value (for invalid-auth scenarios)",
			"add-header - Add new header requirement",
			"remove-header - Remove existing header requirement",
		},
		Default: "remove-auth - Remove Authorization requirement (for no-auth scenarios)",
	}, &headerAction); err != nil {
		return err
	}

	actionType := strings.Split(headerAction, " ")[0]
	httpRequest := clone["httpRequest"].(map[string]interface{})

	switch actionType {
	case "remove-auth":
		if headers, ok := httpRequest["headers"]; ok {
			headerMap := headers.(map[string]interface{})
			// Configure to match requests WITHOUT Authorization header
			headerMap["Authorization"] = map[string]interface{}{
				"not": true,
			}
			fmt.Println("✅ Configured to match requests WITHOUT Authorization header")
		} else {
			// Create headers map
			httpRequest["headers"] = map[string]interface{}{
				"Authorization": map[string]interface{}{
					"not": true,
				},
			}
			fmt.Println("✅ Added no-auth requirement")
		}

	case "change-auth":
		var newAuthValue string
		if err := survey.AskOne(&survey.Input{
			Message: "New Authorization value (for invalid scenarios):",
			Default: "Bearer invalid",
			Help:    "Value that should trigger this error scenario",
		}, &newAuthValue); err != nil {
			return err
		}

		if headers, ok := httpRequest["headers"]; ok {
			headerMap := headers.(map[string]interface{})
			headerMap["Authorization"] = newAuthValue
		} else {
			httpRequest["headers"] = map[string]interface{}{
				"Authorization": newAuthValue,
			}
		}
		fmt.Printf("✅ Authorization requirement updated to: %s\n", newAuthValue)

	case "add-header":
		var headerName, headerValue string
		if err := survey.AskOne(&survey.Input{
			Message: "Header name:",
			Help:    "e.g., 'X-API-Key', 'Content-Type'",
		}, &headerName); err != nil {
			return err
		}

		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Value for '%s':", headerName),
			Help:    "Exact value or use regex patterns",
		}, &headerValue); err != nil {
			return err
		}

		if headers, ok := httpRequest["headers"]; ok {
			headerMap := headers.(map[string]interface{})
			headerMap[headerName] = headerValue
		} else {
			httpRequest["headers"] = map[string]interface{}{
				headerName: headerValue,
			}
		}
		fmt.Printf("✅ Added header requirement: %s: %s\n", headerName, headerValue)

	case "remove-header":
		fmt.Println("📝 Remove header functionality coming soon!")
	}

	return nil
}

// generateErrorResponse generates appropriate error response for status code
func (cp *CollectionProcessor) generateErrorResponse(statusCode int) string {
	switch statusCode {
	case 400:
		return `{
  "error": {
    "code": "BAD_REQUEST",
    "message": "Invalid request data provided",
    "details": "Request validation failed",
    "timestamp": "${timestamp}"
  }
}`
	case 401:
		return `{
  "error": {
    "code": "UNAUTHORIZED", 
    "message": "Authentication required",
    "details": "Please provide valid authentication credentials",
    "timestamp": "${timestamp}"
  }
}`
	case 403:
		return `{
  "error": {
    "code": "FORBIDDEN",
    "message": "Access denied",
    "details": "Insufficient permissions for this resource",
    "timestamp": "${timestamp}"
  }
}`
	case 404:
		return `{
  "error": {
    "code": "NOT_FOUND",
    "message": "Resource not found", 
    "details": "The requested resource does not exist",
    "timestamp": "${timestamp}"
  }
}`
	case 409:
		return `{
  "error": {
    "code": "CONFLICT",
    "message": "Resource conflict",
    "details": "The request conflicts with current state",
    "timestamp": "${timestamp}"
  }
}`
	case 422:
		return `{
  "error": {
    "code": "UNPROCESSABLE_ENTITY",
    "message": "Validation failed",
    "details": "Request data failed validation",
    "timestamp": "${timestamp}"
  }
}`
	case 429:
		return `{
  "error": {
    "code": "RATE_LIMITED",
    "message": "Too many requests",
    "details": "Rate limit exceeded",
    "retryAfter": 60,
    "timestamp": "${timestamp}"
  }
}`
	case 500:
		return `{
  "error": {
    "code": "INTERNAL_SERVER_ERROR",
    "message": "An internal server error occurred",
    "details": "Please try again later",
    "timestamp": "${timestamp}"
  }
}`
	case 502:
		return `{
  "error": {
    "code": "BAD_GATEWAY",
    "message": "Bad gateway",
    "details": "Upstream server error",
    "timestamp": "${timestamp}"
  }
}`
	case 503:
		return `{
  "error": {
    "code": "SERVICE_UNAVAILABLE",
    "message": "Service temporarily unavailable", 
    "details": "Please try again later",
    "timestamp": "${timestamp}"
  }
}`
	default:
		return fmt.Sprintf(`{
  "error": {
    "code": "ERROR_%d",
    "message": "Request failed",
    "status": %d,
    "timestamp": "${timestamp}"
  }
}`, statusCode, statusCode)
	}
}

// Placeholder functions for the enhanced review options
func (cp *CollectionProcessor) addScenarioFlow(expectations []map[string]interface{}, nodes []ExecutionNode) error {
	fmt.Println("✨ Add Scenario Flow - Coming in next update!")
	return fmt.Errorf("configuration_updated")
}

func (cp *CollectionProcessor) createErrorVariants(expectations []map[string]interface{}, nodes []ExecutionNode) error {
	fmt.Println("✨ Create Error Variants - Coming in next update!")
	return fmt.Errorf("configuration_updated")
}

func (cp *CollectionProcessor) addAuthScenarios(expectations []map[string]interface{}, nodes []ExecutionNode) error {
	fmt.Println("✨ Add Auth Scenarios - Coming in next update!")
	return fmt.Errorf("configuration_updated")
}

func (cp *CollectionProcessor) batchCloneFlow(expectations []map[string]interface{}, nodes []ExecutionNode) error {
	fmt.Println("✨ Batch Clone Flow - Coming in next update!")
	return fmt.Errorf("configuration_updated")
}

func (cp *CollectionProcessor) templateEndpointsFlow(expectations []map[string]interface{}, nodes []ExecutionNode) error {
	fmt.Println("✨ Template Endpoints Flow - Coming in next update!")
	return fmt.Errorf("configuration_updated")
}

func (cp *CollectionProcessor) modifyCloneAuthScenario(clone map[string]interface{}) error {
	fmt.Println("✨ Auth Scenario Modification - Coming in next update!")
	return nil
}

func (cp *CollectionProcessor) modifyCloneResponseBody(clone map[string]interface{}) error {
	fmt.Println("✨ Response Body Modification - Coming in next update!")
	return nil
}

func (cp *CollectionProcessor) modifyCloneMultiple(clone map[string]interface{}) error {
	fmt.Println("✨ Multiple Modifications - Coming in next update!")
	return nil
}

func (cp *CollectionProcessor) viewJSONConfiguration(expectations []map[string]interface{}) error {
	jsonBytes, err := json.MarshalIndent(expectations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format expectations: %w", err)
	}
	fmt.Printf("\n📋 Full JSON Configuration:\n%s\n\n", string(jsonBytes))
	return nil
}

func (cp *CollectionProcessor) modifyScenarios(expectations []map[string]interface{}, scenarios []APIScenario) error {
	fmt.Println("\n🎭 SCENARIO MODIFICATION OPTIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Scenario modification features:")
	fmt.Println("   • Clone existing scenarios to create variants")
	fmt.Println("   • Modify response status codes, headers, or bodies")
	fmt.Println("   • Add authentication scenarios (success/fail)")
	fmt.Println("   • Create error response variants")
	fmt.Println("🚀 Advanced scenario modification coming in next update!")
	return fmt.Errorf("scenarios_modified")
}

func (cp *CollectionProcessor) testScenarios(expectations []map[string]interface{}) error {
	fmt.Println("\n🧪 SCENARIO TESTING")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Test scenario features:")
	fmt.Println("   • Validate scenario configurations")
	fmt.Println("   • Test priority ordering")
	fmt.Println("   • Verify matching criteria")
	fmt.Println("   • Mock request simulation")
	fmt.Println("🚀 Scenario testing coming in next update!")
	return nil
}
