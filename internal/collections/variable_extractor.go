package collections

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

// VariableExtractor provides advanced variable extraction from API responses
type VariableExtractor struct {
	response *APIResponse
}

// NewVariableExtractor creates a new variable extractor
func NewVariableExtractor(response *APIResponse) *VariableExtractor {
	return &VariableExtractor{response: response}
}

// ExtractVariables intelligently extracts variables with disambiguation
func (ve *VariableExtractor) ExtractVariables(requestedVars []string) (map[string]string, error) {
	result := make(map[string]string)
	
	// Parse response body as JSON
	var jsonData interface{}
	if err := json.Unmarshal([]byte(ve.response.Body), &jsonData); err != nil {
		return nil, fmt.Errorf("response is not valid JSON: %w", err)
	}
	
	for _, varName := range requestedVars {
		// Check if varName is already a path (contains . or [])
		if strings.Contains(varName, ".") || strings.Contains(varName, "[") {
			// User specified exact path - extract directly
			value, err := ve.extractByPath(jsonData, varName)
			if err != nil {
				fmt.Printf("⚠️  Could not extract '%s': %v\n", varName, err)
				continue
			}
			result[varName] = value
		} else {
			// Simple name - search and disambiguate
			value, err := ve.extractWithDisambiguation(jsonData, varName)
			if err != nil {
				fmt.Printf("⚠️  Could not extract '%s': %v\n", varName, err)
				continue
			}
			result[varName] = value
		}
	}
	
	return result, nil
}

// extractByPath extracts value using dot notation or array syntax
func (ve *VariableExtractor) extractByPath(data interface{}, path string) (string, error) {
	// Support both dot notation and bracket notation
	// Examples: "data.user.id", "items[0].id", "data.users[0].profile.name"
	
	parts := ve.parsePath(path)
	current := data
	
	for _, part := range parts {
		if part.isArray {
			// Handle array access
			arr, ok := current.([]interface{})
			if !ok {
				return "", fmt.Errorf("expected array at '%s'", part.key)
			}
			if part.index >= len(arr) {
				return "", fmt.Errorf("array index %d out of bounds at '%s'", part.index, part.key)
			}
			current = arr[part.index]
		} else {
			// Handle object access
			obj, ok := current.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("expected object at '%s'", part.key)
			}
			val, exists := obj[part.key]
			if !exists {
				return "", fmt.Errorf("key '%s' not found", part.key)
			}
			current = val
		}
	}
	
	return fmt.Sprintf("%v", current), nil
}

// extractWithDisambiguation finds all matches and lets user choose
func (ve *VariableExtractor) extractWithDisambiguation(data interface{}, varName string) (string, error) {
	// Find all paths where this variable name appears
	paths := ve.findAllPaths(data, varName, "")
	
	if len(paths) == 0 {
		return "", fmt.Errorf("variable '%s' not found in response", varName)
	}
	
	if len(paths) == 1 {
		// Only one match - use it
		return paths[0].value, nil
	}
	
	// Multiple matches - ask user to disambiguate
	fmt.Printf("\n🔍 Found %d occurrences of '%s' in response:\n", len(paths), varName)
	
	options := make([]string, len(paths))
	for i, p := range paths {
		options[i] = fmt.Sprintf("%s = %s", p.path, p.value)
	}
	
	var selected string
	if err := survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Multiple '%s' found. Which one do you want?", varName),
		Options: options,
	}, &selected); err != nil {
		return "", err
	}
	
	// Extract value from selection
	for _, p := range paths {
		if strings.HasPrefix(selected, p.path) {
			return p.value, nil
		}
	}
	
	return "", fmt.Errorf("selection error")
}

// PathMatch represents a found variable path
type PathMatch struct {
	path  string
	value string
}

// findAllPaths recursively finds all paths matching the variable name
func (ve *VariableExtractor) findAllPaths(data interface{}, varName string, currentPath string) []PathMatch {
	var matches []PathMatch
	
	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			newPath := key
			if currentPath != "" {
				newPath = currentPath + "." + key
			}
			
			// Check if this key matches
			if key == varName {
				matches = append(matches, PathMatch{
					path:  newPath,
					value: fmt.Sprintf("%v", val),
				})
			}
			
			// Recurse into nested structures
			matches = append(matches, ve.findAllPaths(val, varName, newPath)...)
		}
		
	case []interface{}:
		for i, item := range v {
			newPath := fmt.Sprintf("%s[%d]", currentPath, i)
			matches = append(matches, ve.findAllPaths(item, varName, newPath)...)
		}
	}
	
	return matches
}

// PathPart represents a parsed path component
type PathPart struct {
	key     string
	isArray bool
	index   int
}

// parsePath parses a path string into components
func (ve *VariableExtractor) parsePath(path string) []PathPart {
	var parts []PathPart
	
	// Regular expression to match: key, key[0], key.subkey, key[0].subkey
	re := regexp.MustCompile(`([^.\[]+)(?:\[(\d+)\])?`)
	matches := re.FindAllStringSubmatch(path, -1)
	
	for _, match := range matches {
		if match[1] == "" {
			continue
		}
		
		part := PathPart{key: match[1]}
		
		if match[2] != "" {
			// Array access
			part.isArray = true
			part.index, _ = strconv.Atoi(match[2])
		}
		
		parts = append(parts, part)
	}
	
	return parts
}

// ExtractFromHeaders extracts variables from response headers
func (ve *VariableExtractor) ExtractFromHeaders(headerMappings map[string]string) map[string]string {
	result := make(map[string]string)
	
	for varName, headerName := range headerMappings {
		if value, exists := ve.response.Headers[headerName]; exists {
			result[varName] = value
		}
	}
	
	return result
}

// ExtractFromCookies extracts variables from response cookies
func (ve *VariableExtractor) ExtractFromCookies(cookieMappings map[string]string) map[string]string {
	result := make(map[string]string)
	
	for varName, cookieName := range cookieMappings {
		if value, exists := ve.response.Cookies[cookieName]; exists {
			result[varName] = value
		}
	}
	
	return result
}

// SmartExtract performs intelligent extraction with user guidance
func (ve *VariableExtractor) SmartExtract(suggestedVars []string) (map[string]string, error) {
	fmt.Println("\n🔧 VARIABLE EXTRACTION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Show response preview
	fmt.Println("\n📋 Response Preview:")
	ve.showResponsePreview()
	
	// Guide user through extraction
	var extractionMethod string
	if err := survey.AskOne(&survey.Select{
		Message: "How would you like to extract variables?",
		Options: []string{
			"auto - Auto-detect from suggested names (may need disambiguation)",
			"path - I'll provide exact JSONPath expressions",
			"headers - Extract from response headers",
			"cookies - Extract from response cookies",
			"skip - No variables needed from this response",
		},
	}, &extractionMethod); err != nil {
		return nil, err
	}
	
	method := strings.Split(extractionMethod, " ")[0]
	
	switch method {
	case "auto":
		return ve.ExtractVariables(suggestedVars)
	case "path":
		return ve.extractWithPaths()
	case "headers":
		return ve.extractWithHeaderGuidance()
	case "cookies":
		return ve.extractWithCookieGuidance()
	case "skip":
		return map[string]string{}, nil
	}
	
	return map[string]string{}, nil
}

// showResponsePreview shows a formatted preview of the response
func (ve *VariableExtractor) showResponsePreview() {
	var jsonData interface{}
	if err := json.Unmarshal([]byte(ve.response.Body), &jsonData); err == nil {
		preview, _ := json.MarshalIndent(jsonData, "", "  ")
		// Show first 500 chars
		if len(preview) > 500 {
			fmt.Printf("%s\n... (truncated)\n", string(preview[:500]))
		} else {
			fmt.Printf("%s\n", string(preview))
		}
	}
}

// extractWithPaths guides user through JSONPath extraction
func (ve *VariableExtractor) extractWithPaths() (map[string]string, error) {
	result := make(map[string]string)
	
	for {
		var pathInput string
		if err := survey.AskOne(&survey.Input{
			Message: "Enter variable extraction (format: varName=path or just path):",
			Help:    "Examples: token=data.token, userId=data.user.id, items[0].id",
		}, &pathInput); err != nil {
			return nil, err
		}
		
		if pathInput == "" {
			break
		}
		
		// Parse input
		var varName, path string
		if strings.Contains(pathInput, "=") {
			parts := strings.SplitN(pathInput, "=", 2)
			varName = strings.TrimSpace(parts[0])
			path = strings.TrimSpace(parts[1])
		} else {
			path = strings.TrimSpace(pathInput)
			// Use last part of path as variable name
			pathParts := strings.Split(path, ".")
			varName = pathParts[len(pathParts)-1]
			varName = strings.ReplaceAll(varName, "[", "")
			varName = strings.ReplaceAll(varName, "]", "")
		}
		
		// Extract value
		var jsonData interface{}
		json.Unmarshal([]byte(ve.response.Body), &jsonData)
		value, err := ve.extractByPath(jsonData, path)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
			continue
		}
		
		result[varName] = value
		fmt.Printf("✅ Extracted: %s = %s\n", varName, value)
		
		var addMore bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Add another variable?",
			Default: false,
		}, &addMore); err != nil {
			return nil, err
		}
		
		if !addMore {
			break
		}
	}
	
	return result, nil
}

// extractWithHeaderGuidance guides user through header extraction
func (ve *VariableExtractor) extractWithHeaderGuidance() (map[string]string, error) {
	fmt.Println("\n📨 Available Headers:")
	for k, v := range ve.response.Headers {
		fmt.Printf("  %s: %s\n", k, v)
	}
	
	result := make(map[string]string)
	
	for {
		var mapping string
		if err := survey.AskOne(&survey.Input{
			Message: "Enter header mapping (format: varName=HeaderName):",
			Help:    "Example: token=Authorization, sessionId=X-Session-ID",
		}, &mapping); err != nil {
			return nil, err
		}
		
		if mapping == "" {
			break
		}
		
		parts := strings.SplitN(mapping, "=", 2)
		if len(parts) != 2 {
			fmt.Println("❌ Invalid format. Use: varName=HeaderName")
			continue
		}
		
		varName := strings.TrimSpace(parts[0])
		headerName := strings.TrimSpace(parts[1])
		
		if value, exists := ve.response.Headers[headerName]; exists {
			result[varName] = value
			fmt.Printf("✅ Extracted: %s = %s\n", varName, value)
		} else {
			fmt.Printf("❌ Header '%s' not found\n", headerName)
		}
		
		var addMore bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Add another header?",
			Default: false,
		}, &addMore); err != nil {
			return nil, err
		}
		
		if !addMore {
			break
		}
	}
	
	return result, nil
}

// extractWithCookieGuidance guides user through cookie extraction
func (ve *VariableExtractor) extractWithCookieGuidance() (map[string]string, error) {
	fmt.Println("\n🍪 Available Cookies:")
	for k, v := range ve.response.Cookies {
		fmt.Printf("  %s: %s\n", k, v)
	}
	
	result := make(map[string]string)
	
	for {
		var mapping string
		if err := survey.AskOne(&survey.Input{
			Message: "Enter cookie mapping (format: varName=CookieName):",
			Help:    "Example: sessionToken=SESSION_ID",
		}, &mapping); err != nil {
			return nil, err
		}
		
		if mapping == "" {
			break
		}
		
		parts := strings.SplitN(mapping, "=", 2)
		if len(parts) != 2 {
			fmt.Println("❌ Invalid format. Use: varName=CookieName")
			continue
		}
		
		varName := strings.TrimSpace(parts[0])
		cookieName := strings.TrimSpace(parts[1])
		
		if value, exists := ve.response.Cookies[cookieName]; exists {
			result[varName] = value
			fmt.Printf("✅ Extracted: %s = %s\n", varName, value)
		} else {
			fmt.Printf("❌ Cookie '%s' not found\n", cookieName)
		}
		
		var addMore bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Add another cookie?",
			Default: false,
		}, &addMore); err != nil {
			return nil, err
		}
		
		if !addMore {
			break
		}
	}
	
	return result, nil
}
