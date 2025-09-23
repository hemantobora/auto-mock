package builders

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

// BuildGraphQLExpectation builds a single GraphQL mock expectation using enhanced 8-step process
func BuildGraphQLExpectation() (MockExpectation, error) {
	return BuildGraphQLExpectationWithContext([]MockExpectation{})
}

// BuildGraphQLExpectationWithContext builds a GraphQL expectation with context of existing expectations
func BuildGraphQLExpectationWithContext(existingExpectations []MockExpectation) (MockExpectation, error) {
	var expectation MockExpectation

	fmt.Println("🚀 Starting Enhanced 8-Step GraphQL Expectation Builder")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	steps := []struct {
		name string
		fn   func(*MockExpectation) error
	}{
		{"GraphQL Operation Details", collectGraphQLOperationDetails},
		{"Expectation Identification", func(exp *MockExpectation) error {
			return CollectExpectationName(exp, existingExpectations)
		}},
		{"Query/Mutation Content", collectGraphQLQueryContent},
		{"Variable Matching", collectGraphQLVariableMatching},
		{"Request Header Matching", collectRequestHeaderMatching}, // Reuse from REST
		{"GraphQL Response Definition", collectGraphQLResponseDefinition},
		{"Advanced Features", collectAdvancedFeatures}, // Enhanced advanced features
		{"Review and Confirm", reviewGraphQLConfirm},
	}

	for i, step := range steps {
		if err := step.fn(&expectation); err != nil {
			return expectation, fmt.Errorf("step %d (%s) failed: %w", i+1, step.name, err)
		}
	}

	return expectation, nil
}

// Step 1: Collect GraphQL Operation Details
func collectGraphQLOperationDetails(expectation *MockExpectation) error {
	fmt.Println("\n🔗 Step 1: GraphQL Operation Details")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// GraphQL always uses POST to /graphql
	expectation.Method = "POST"
	expectation.Path = "/graphql"

	// Operation type selection
	var operationType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select GraphQL operation type:",
		Options: []string{
			"query - Read data from the server",
			"mutation - Modify data on the server",
			"subscription - Real-time data updates",
		},
		Default: "query - Read data from the server",
	}, &operationType); err != nil {
		return err
	}

	operationType = strings.Split(operationType, " ")[0]

	// Store operation type for later use
	if expectation.Headers == nil {
		expectation.Headers = make(map[string]string)
	}
	expectation.Headers["X-GraphQL-Operation-Type"] = operationType

	fmt.Printf("✅ GraphQL Operation: %s POST /graphql\n", operationType)
	return nil
}

// Step 2: Collect GraphQL Query/Mutation Content
func collectGraphQLQueryContent(expectation *MockExpectation) error {
	fmt.Println("\n📝 Step 2: Query/Mutation Content")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	operationType := expectation.Headers["X-GraphQL-Operation-Type"]

	var useTemplate bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("Use a %s template to get started?", operationType),
		Default: true,
		Help:    "Templates provide common GraphQL patterns you can customize",
	}, &useTemplate); err != nil {
		return err
	}

	var queryContent string
	var operationName string

	if useTemplate {
		template, name, err := generateGraphQLTemplate(operationType)
		if err != nil {
			return err
		}
		queryContent = template
		operationName = name

		fmt.Printf("💡 Generated %s template:\n%s\n\n", operationType, template)

		var useGenerated bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Use this template?",
			Default: true,
		}, &useGenerated); err != nil {
			return err
		}

		if !useGenerated {
			queryContent = ""
		}
	}

	if queryContent == "" {
		if err := survey.AskOne(&survey.Multiline{
			Message: fmt.Sprintf("Enter your GraphQL %s:", operationType),
			Help:    "Paste your complete GraphQL query, mutation, or subscription",
		}, &queryContent); err != nil {
			return err
		}

		// Extract operation name if not provided
		if operationName == "" {
			operationName = extractOperationName(queryContent)
		}
	}

	// Create GraphQL request body
	requestBodyMap := map[string]interface{}{
		"query": queryContent,
	}

	if operationName != "" {
		requestBodyMap["operationName"] = operationName
	} else {
		requestBodyMap["operationName"] = nil
	}
	
	// Add empty variables placeholder
	requestBodyMap["variables"] = map[string]interface{}{}

	// Convert to JSON for storage
	requestBodyJSON, err := json.MarshalIndent(requestBodyMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	expectation.Body = string(requestBodyJSON)

	fmt.Printf("✅ GraphQL %s configured with operation: %s\n", operationType, operationName)
	return nil
}

// Step 3: Collect GraphQL Variable Matching
func collectGraphQLVariableMatching(expectation *MockExpectation) error {
	fmt.Println("\n🔢 Step 3: Variable Matching")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	var needsVariables bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Does this operation require specific variables to match?",
		Default: false,
		Help:    "Only specify if you need to match exact variable values",
	}, &needsVariables); err != nil {
		return err
	}

	if !needsVariables {
		fmt.Println("ℹ️  No variable matching configured - will accept any variables")
		return nil
	}

	// Variables will be part of the request body matching
	var variablesJSON string

	if err := survey.AskOne(&survey.Multiline{
		Message: "Enter variables JSON to match:",
		Help:    "Example: {\"id\": \"123\", \"limit\": 10}",
	}, &variablesJSON); err != nil {
		return err
	}

	// Validate variables JSON
	if err := ValidateJSON(variablesJSON); err != nil {
		fmt.Printf("⚠️  Variables JSON validation failed: %v\n", err)
		var proceed bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "JSON is invalid. Use it anyway?",
			Default: false,
		}, &proceed); err != nil {
			return err
		}
		if !proceed {
			return fmt.Errorf("invalid variables JSON provided")
		}
	}

	// Update the request body to include specific variables
	var operationName string
	if name, exists := expectation.Headers["X-GraphQL-Operation-Name"]; exists {
		operationName = name
	}

	// Parse existing request body to get query
	var existingBody map[string]interface{}
	if err := json.Unmarshal([]byte(expectation.Body.(string)), &existingBody); err != nil {
		return fmt.Errorf("failed to parse existing request body: %w", err)
	}
	
	queryContent := ""
	if q, ok := existingBody["query"].(string); ok {
		queryContent = q
	}
	
	if on, ok := existingBody["operationName"].(string); ok {
		operationName = on
	}

	// Parse variables JSON
	var variables interface{}
	if err := json.Unmarshal([]byte(variablesJSON), &variables); err != nil {
		return fmt.Errorf("failed to parse variables JSON: %w", err)
	}

	// Create new request body with variables
	requestBodyMap := map[string]interface{}{
		"query": queryContent,
		"operationName": operationName,
		"variables": variables,
	}

	requestBodyJSON, err := json.MarshalIndent(requestBodyMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	expectation.Body = string(requestBodyJSON)

	fmt.Printf("✅ Variable matching configured\n")
	return nil
}

// Step 5: Collect GraphQL Response Definition
func collectGraphQLResponseDefinition(expectation *MockExpectation) error {
	fmt.Println("\n📤 Step 5: GraphQL Response Definition")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// GraphQL responses are typically 200 OK with errors in the response body
	var responseType string
	if err := survey.AskOne(&survey.Select{
		Message: "Select response type:",
		Options: []string{
			"success - Successful response with data",
			"error - GraphQL error response",
			"partial - Partial data with some errors",
		},
		Default: "success - Successful response with data",
	}, &responseType); err != nil {
		return err
	}

	responseType = strings.Split(responseType, " ")[0]
	expectation.StatusCode = 200 // GraphQL always returns 200

	var useTemplate bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Use a response template?",
		Default: true,
		Help:    "Templates provide proper GraphQL response structure",
	}, &useTemplate); err != nil {
		return err
	}

	var responseBody string

	if useTemplate {
		template := generateGraphQLResponseTemplate(responseType)
		fmt.Printf("💡 Generated %s response template:\n%s\n\n", responseType, template)

		var useGenerated bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Use this template?",
			Default: true,
		}, &useGenerated); err != nil {
			return err
		}

		if useGenerated {
			responseBody = template
		}
	}

	if responseBody == "" {
		if err := survey.AskOne(&survey.Multiline{
			Message: "Enter GraphQL response JSON:",
			Help:    "Must include 'data' and/or 'errors' fields per GraphQL spec",
		}, &responseBody); err != nil {
			return err
		}

		// Validate JSON
		if err := ValidateJSON(responseBody); err != nil {
			fmt.Printf("⚠️  JSON validation failed: %v\n", err)
			var proceed bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "JSON is invalid. Use it anyway?",
				Default: false,
			}, &proceed); err != nil {
				return err
			}
			if !proceed {
				return fmt.Errorf("invalid JSON provided")
			}
		}
	}

	// Format and store JSON
	formattedJSON, _ := FormatJSON(responseBody)
	expectation.ResponseBody = formattedJSON

	fmt.Printf("✅ GraphQL %s response configured\n", responseType)
	return nil
}

// Step 8: Review and Confirm (GraphQL specific)
func reviewGraphQLConfirm(expectation *MockExpectation) error {
	fmt.Println("\n🔄 Step 8: Review and Confirm")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	operationType := expectation.Headers["X-GraphQL-Operation-Type"]

	// Display summary
	fmt.Printf("\n📋 GraphQL Expectation Summary:\n")
	fmt.Printf("   Name: %s\n", expectation.Name)
	if expectation.Description != "" {
		fmt.Printf("   Description: %s\n", expectation.Description)
	}
	fmt.Printf("   Operation Type: %s\n", operationType)
	fmt.Printf("   Endpoint: %s %s\n", expectation.Method, expectation.Path)
	fmt.Printf("   Status Code: %d\n", expectation.StatusCode)

	if len(expectation.Headers) > 1 { // More than just the operation type
		fmt.Printf("   Request Headers: %d\n", len(expectation.Headers)-1)
	}
	if expectation.Body != nil {
		fmt.Printf("   Query/Variables: Configured\n")
	}

	var confirm bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Create this GraphQL expectation?",
		Default: true,
	}, &confirm); err != nil {
		return err
	}

	if !confirm {
		return fmt.Errorf("expectation creation cancelled by user")
	}

	fmt.Printf("\n✅ GraphQL Expectation Created: %s\n", expectation.Name)
	return nil
}

// Helper functions for GraphQL

// generateGraphQLTemplate generates GraphQL operation templates
func generateGraphQLTemplate(operationType string) (string, string, error) {
	var template, operationName string

	switch operationType {
	case "query":
		var queryType string
		if err := survey.AskOne(&survey.Select{
			Message: "Select query template:",
			Options: []string{
				"user - Get user by ID",
				"users - List users with pagination",
				"search - Search with filters",
				"custom - Custom query",
			},
		}, &queryType); err != nil {
			return "", "", err
		}

		queryType = strings.Split(queryType, " ")[0]

		switch queryType {
		case "user":
			template = `query GetUser($id: ID!) {
  user(id: $id) {
    id
    name
    email
    createdAt
  }
}`
			operationName = "GetUser"
		case "users":
			template = `query GetUsers($limit: Int, $offset: Int) {
  users(limit: $limit, offset: $offset) {
    edges {
      node {
        id
        name
        email
      }
    }
    pageInfo {
      hasNextPage
      hasPreviousPage
    }
  }
}`
			operationName = "GetUsers"
		case "search":
			template = `query SearchUsers($query: String!, $filters: UserFilters) {
  searchUsers(query: $query, filters: $filters) {
    totalCount
    results {
      id
      name
      email
      score
    }
  }
}`
			operationName = "SearchUsers"
		default:
			// Custom query - let user input manually
			var customQuery string
			if err := survey.AskOne(&survey.Multiline{
				Message: "Enter your custom GraphQL query:",
				Help:    "Paste your complete GraphQL query here",
			}, &customQuery); err != nil {
				return "", "", err
			}
			
			template = strings.TrimSpace(customQuery)
			if template == "" {
				return "", "", fmt.Errorf("custom query cannot be empty")
			}
			
			// Try to extract operation name
			operationName = extractOperationName(template)
			if operationName == "" {
				// Ask user for operation name if not detected
				if err := survey.AskOne(&survey.Input{
					Message: "Enter operation name (optional):",
					Help:    "e.g., 'GetUserData', 'SearchProducts'",
				}, &operationName); err != nil {
					return "", "", err
				}
			}
			// Skip template display for custom mutations - user already sees what they typed
			return template, operationName, nil
		}

	case "mutation":
		var mutationType string
		if err := survey.AskOne(&survey.Select{
			Message: "Select mutation template:",
			Options: []string{
				"create - Create user",
				"update - Update user",
				"delete - Delete user",
				"custom - Custom mutation",
			},
		}, &mutationType); err != nil {
			return "", "", err
		}

		mutationType = strings.Split(mutationType, " ")[0]

		switch mutationType {
		case "create":
			template = `mutation CreateUser($input: CreateUserInput!) {
  createUser(input: $input) {
    user {
      id
      name
      email
      createdAt
    }
    errors {
      field
      message
    }
  }
}`
			operationName = "CreateUser"
		case "update":
			template = `mutation UpdateUser($id: ID!, $input: UpdateUserInput!) {
  updateUser(id: $id, input: $input) {
    user {
      id
      name
      email
      updatedAt
    }
    errors {
      field
      message
    }
  }
}`
			operationName = "UpdateUser"
		case "delete":
			template = `mutation DeleteUser($id: ID!) {
  deleteUser(id: $id) {
    success
    message
  }
}`
			operationName = "DeleteUser"
		default:
			// Custom mutation - let user input manually
			var customMutation string
			if err := survey.AskOne(&survey.Multiline{
				Message: "Enter your custom GraphQL mutation:",
				Help:    "Paste your complete GraphQL mutation here",
			}, &customMutation); err != nil {
				return "", "", err
			}
			
			template = strings.TrimSpace(customMutation)
			if template == "" {
				return "", "", fmt.Errorf("custom mutation cannot be empty")
			}
			
			// Try to extract operation name
			operationName = extractOperationName(template)
			if operationName == "" {
				// Ask user for operation name if not detected
				if err := survey.AskOne(&survey.Input{
					Message: "Enter operation name (optional):",
					Help:    "e.g., 'ProcessPayment', 'UpdateInventory'",
				}, &operationName); err != nil {
					return "", "", err
				}
			}
		}

	case "subscription":
		template = `subscription UserUpdates($userId: ID!) {
  userUpdates(userId: $userId) {
    id
    name
    email
    status
    updatedAt
  }
}`
		operationName = "UserUpdates"
	}

	return template, operationName, nil
}

// generateGraphQLResponseTemplate generates GraphQL response templates
func generateGraphQLResponseTemplate(responseType string) string {
	switch responseType {
	case "success":
		return `{
  "data": {
    "user": {
      "id": "user-123",
      "name": "John Doe",
      "email": "john.doe@example.com",
      "createdAt": "2025-09-21T15:30:00Z"
    }
  }
}`
	case "error":
		return `{
  "errors": [
    {
      "message": "User not found",
      "locations": [
        {
          "line": 2,
          "column": 3
        }
      ],
      "path": ["user"],
      "extensions": {
        "code": "USER_NOT_FOUND",
        "timestamp": "2025-09-21T15:30:00Z"
      }
    }
  ]
}`
	case "partial":
		return `{
  "data": {
    "user": {
      "id": "user-123",
      "name": "John Doe",
      "email": null
    }
  },
  "errors": [
    {
      "message": "Email field is restricted",
      "path": ["user", "email"],
      "extensions": {
        "code": "FIELD_RESTRICTED"
      }
    }
  ]
}`
	default:
		return `{
  "data": {
    "message": "GraphQL response",
    "timestamp": "2025-09-21T15:30:00Z"
  }
}`
	}
}

// extractOperationName extracts operation name from GraphQL query
func extractOperationName(query string) string {
	lines := strings.Split(query, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "query ") || strings.HasPrefix(line, "mutation ") || strings.HasPrefix(line, "subscription ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Remove parentheses and parameters
				name := parts[1]
				if idx := strings.Index(name, "("); idx != -1 {
					name = name[:idx]
				}
				return name
			}
		}
	}
	return ""
}

// extractQueryFromBody extracts the query field from GraphQL request body
func extractQueryFromBody(body string) string {
	// Simple extraction - in production you'd parse the JSON properly
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"query":`) {
			// Extract the query content between quotes
			start := strings.Index(line, `"`) + 1
			if start > 0 {
				end := strings.LastIndex(line, `"`)
				if end > start {
					return strings.ReplaceAll(line[start:end], `\"`, `"`)
				}
			}
		}
	}
	return ""
}
