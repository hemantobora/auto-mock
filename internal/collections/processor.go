package collections

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	store       *state.S3Store
	projectName string
	cleanName   string
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
	
	// Step 5: Configure matching criteria
	expectations, err := cp.configureMatchingCriteria(executionNodes)
	if err != nil {
		return fmt.Errorf("failed to configure matching: %w", err)
	}
	
	// Step 6: Review and validate
	if err := cp.reviewExpectations(expectations); err != nil {
		return fmt.Errorf("review failed: %w", err)
	}
	
	// Step 7: Save to S3
	return cp.saveExpectations(expectations)
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
	
	var proceed bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "I understand the security implications and have prepared my environment. Continue?",
		Default: false,
	}, &proceed); err != nil {
		return err
	}
	
	if !proceed {
		return fmt.Errorf("user cancelled after disclaimer")
	}
	
	return nil
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

// Step 3: Build execution DAG
func (cp *CollectionProcessor) buildExecutionDAG(apis []APIRequest) ([]ExecutionNode, error) {
	fmt.Println("\n🔗 EXECUTION ORDER CONFIGURATION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Configure the order of API execution (like a dependency graph)")
	fmt.Printf("Found %d APIs to order:\n\n", len(apis))
	
	// Display all APIs
	for i, api := range apis {
		fmt.Printf("%d. %s %s - %s\n", i+1, api.Method, api.URL, api.Name)
	}
	
	var executionNodes []ExecutionNode
	
	// For each API, ask for dependencies
	for _, api := range apis {
		fmt.Printf("\n🔧 Configuring: %s %s\n", api.Method, api.Name)
		
		node := ExecutionNode{
			API:          api,
			Dependencies: []string{},
			Variables:    []string{},
		}
		
		// Ask for dependencies
		if len(executionNodes) > 0 {
			var availableDeps []string
			for _, existing := range executionNodes {
				availableDeps = append(availableDeps, existing.API.Name)
			}
			
			var selectedDeps []string
			if err := survey.AskOne(&survey.MultiSelect{
				Message: "Which APIs should run BEFORE this one? (Select dependencies)",
				Options: availableDeps,
			}, &selectedDeps); err != nil {
				return nil, err
			}
			
			node.Dependencies = selectedDeps
		}
		
		// Ask what variables this API provides (from response)
		var providedVars string
		if err := survey.AskOne(&survey.Input{
			Message: "What variables does this API provide? (comma-separated, e.g., token,userId)",
			Help:    "Variables extracted from response for use in subsequent APIs",
		}, &providedVars); err != nil {
			return nil, err
		}
		
		if providedVars != "" {
			node.Variables = strings.Split(strings.ReplaceAll(providedVars, " ", ""), ",")
		}
		
		executionNodes = append(executionNodes, node)
	}
	
	// Validate DAG (no cycles)
	if err := cp.validateDAG(executionNodes); err != nil {
		return nil, fmt.Errorf("invalid execution order: %w", err)
	}
	
	fmt.Println("✅ Execution order configured successfully!")
	return executionNodes, nil
}

// Step 4: Execute APIs in DAG order
func (cp *CollectionProcessor) executeAPIs(nodes []ExecutionNode) error {
	fmt.Println("\n🚀 EXECUTING APIs")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	executed := make(map[string]*APIResponse)
	variables := make(map[string]string)
	
	// Execute in dependency order
	for i := 0; i < len(nodes); i++ {
		node := &nodes[i]
		
		// Check if dependencies are satisfied
		allDepsSatisfied := true
		for _, dep := range node.Dependencies {
			if _, exists := executed[dep]; !exists {
				allDepsSatisfied = false
				break
			}
		}
		
		if !allDepsSatisfied {
			// Move to end and try again
			nodes = append(nodes[i+1:], nodes[i])
			i--
			continue
		}
		
		fmt.Printf("▶️  Executing: %s %s\n", node.API.Method, node.API.Name)
		
		// Execute the API
		response, err := cp.executeAPI(node.API, variables)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
			
			var continueOnError bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Continue with remaining APIs?",
				Default: true,
			}, &continueOnError); err != nil {
				return err
			}
			
			if !continueOnError {
				return fmt.Errorf("execution stopped on error")
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
		executed[node.API.Name] = response
		
		// Extract variables from response
		if len(node.Variables) > 0 {
			extractedVars := cp.extractVariables(response, node.Variables)
			for k, v := range extractedVars {
				variables[k] = v
			}
		}
		
		fmt.Printf("✅ Completed: %d ms, Status: %d\n", response.Duration.Milliseconds(), response.StatusCode)
	}
	
	fmt.Printf("\n🎉 Executed %d APIs successfully!\n", len(nodes))
	return nil
}

// Step 5: Configure matching criteria
func (cp *CollectionProcessor) configureMatchingCriteria(nodes []ExecutionNode) ([]map[string]interface{}, error) {
	fmt.Println("\n🎯 MATCHING CRITERIA CONFIGURATION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Configure how requests should be matched to these expectations")
	
	var expectations []map[string]interface{}
	
	for _, node := range nodes {
		if node.Response == nil {
			continue
		}
		
		fmt.Printf("\n🔧 Configuring: %s %s\n", node.API.Method, node.API.Name)
		
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
		
		// Ask for matching criteria
		var matchingOptions []string
		if err := survey.AskOne(&survey.MultiSelect{
			Message: "What should be matched for this request?",
			Options: []string{
				"Path (exact match)",
				"Path (pattern match)",
				"Query parameters",
				"Headers",
				"Request body",
				"Path + Query parameters (recommended)",
			},
			Default: []string{"Path + Query parameters (recommended)"},
		}, &matchingOptions); err != nil {
			return nil, err
		}
		
		// Configure based on selections
		httpRequest := expectation["httpRequest"].(map[string]interface{})
		
		for _, option := range matchingOptions {
			switch {
			case strings.Contains(option, "Query parameters"):
				if len(node.API.QueryParams) > 0 {
					httpRequest["queryStringParameters"] = node.API.QueryParams
				}
			case strings.Contains(option, "Headers"):
				if len(node.API.Headers) > 0 {
					httpRequest["headers"] = node.API.Headers
				}
			case strings.Contains(option, "Request body"):
				if node.API.Body != "" {
					httpRequest["body"] = node.API.Body
				}
			case strings.Contains(option, "pattern match"):
				// Convert exact path to pattern
				path := cp.extractPath(node.API.URL)
				httpRequest["path"] = cp.convertToPattern(path)
			}
		}
		
		expectations = append(expectations, expectation)
	}
	
	return expectations, nil
}

// Step 6: Review expectations
func (cp *CollectionProcessor) reviewExpectations(expectations []map[string]interface{}) error {
	fmt.Println("\n📋 REVIEW GENERATED EXPECTATIONS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	jsonBytes, err := json.MarshalIndent(expectations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format expectations: %w", err)
	}
	
	fmt.Printf("Generated %d expectations:\n\n", len(expectations))
	fmt.Println(string(jsonBytes))
	
	for {
		var action string
		if err := survey.AskOne(&survey.Select{
			Message: "Review complete. What would you like to do?",
			Options: []string{
				"save - Save expectations to S3",
				"redo - Reconfigure matching criteria",
				"exit - Exit without saving",
			},
		}, &action); err != nil {
			return err
		}
		
		action = strings.Split(action, " ")[0]
		switch action {
		case "save":
			return nil
		case "redo":
			fmt.Println("🔄 Reconfiguration coming soon! For now, restart the import process.")
			return fmt.Errorf("reconfiguration requested")
		case "exit":
			return fmt.Errorf("user chose to exit without saving")
		}
	}
}

// Step 7: Save expectations
func (cp *CollectionProcessor) saveExpectations(expectations []map[string]interface{}) error {
	// Convert to MockConfiguration format
	mockConfig := &state.MockConfiguration{
		Metadata: state.ConfigMetadata{
			ProjectID:   cp.cleanName,
			Version:     fmt.Sprintf("v%d", time.Now().Unix()),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Description: fmt.Sprintf("Generated from %s collection", cp.collectionType),
			Provider:    fmt.Sprintf("collection-import-%s", cp.collectionType),
		},
		Expectations: make([]state.MockExpectation, 0, len(expectations)),
	}
	
	// Convert expectations to proper format
	for i, exp := range expectations {
		mockExp := state.MockExpectation{
			ID:       fmt.Sprintf("collection_%s_%d_%d", cp.collectionType, time.Now().Unix(), i),
			Priority: len(expectations) - i, // Higher priority for earlier expectations
		}
		
		// Extract httpRequest and httpResponse
		if httpReq, ok := exp["httpRequest"].(map[string]interface{}); ok {
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
					api.Body = cp.getString(body, "raw")
				}
				
				apis = append(apis, api)
			}
		}
	}
	
	return apis
}

func (cp *CollectionProcessor) parseBrunoCollection(data []byte) ([]APIRequest, error) {
	// Bruno uses .bru files, but if JSON is provided, parse accordingly
	var collection map[string]interface{}
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	
	var apis []APIRequest
	// Bruno parsing logic would go here
	fmt.Println("🚧 Bruno collection parsing - implementing based on Bruno format")
	
	return apis, nil
}

func (cp *CollectionProcessor) parseInsomniaCollection(data []byte) ([]APIRequest, error) {
	var collection map[string]interface{}
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	
	var apis []APIRequest
	
	if resources, ok := collection["resources"].([]interface{}); ok {
		for _, resource := range resources {
			if resourceMap, ok := resource.(map[string]interface{}); ok {
				if resourceType := cp.getString(resourceMap, "_type"); resourceType == "request" {
					api := APIRequest{
						ID:     cp.getString(resourceMap, "_id"),
						Name:   cp.getString(resourceMap, "name"),
						Method: cp.getString(resourceMap, "method"),
						URL:    cp.getString(resourceMap, "url"),
					}
					
					// Extract headers
					if headers, ok := resourceMap["headers"].([]interface{}); ok {
						api.Headers = cp.extractInsomniaHeaders(headers)
					}
					
					// Extract body
					if body, ok := resourceMap["body"].(map[string]interface{}); ok {
						api.Body = cp.getString(body, "text")
					}
					
					apis = append(apis, api)
				}
			}
		}
	}
	
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

func (cp *CollectionProcessor) validateDAG(nodes []ExecutionNode) error {
	// Simple cycle detection
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	
	var dfs func(string) bool
	dfs = func(node string) bool {
		visited[node] = true
		inStack[node] = true
		
		for _, n := range nodes {
			if n.API.Name == node {
				for _, dep := range n.Dependencies {
					if inStack[dep] {
						return true // Cycle found
					}
					if !visited[dep] && dfs(dep) {
						return true
					}
				}
			}
		}
		
		inStack[node] = false
		return false
	}
	
	for _, node := range nodes {
		if !visited[node.API.Name] && dfs(node.API.Name) {
			return fmt.Errorf("circular dependency detected")
		}
	}
	
	return nil
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

func (cp *CollectionProcessor) extractVariables(response *APIResponse, variables []string) map[string]string {
	result := make(map[string]string)
	
	// Simple JSON response parsing for variable extraction
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(response.Body), &jsonData); err == nil {
		for _, varName := range variables {
			if value, exists := jsonData[varName]; exists {
				result[varName] = fmt.Sprintf("%v", value)
			}
		}
	}
	
	return result
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
