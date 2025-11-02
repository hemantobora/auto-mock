package collections

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/hemantobora/auto-mock/internal/models"
)

// ScriptEngine provides a JavaScript execution environment for pre/post scripts
type ScriptEngine struct {
	vm            *goja.Runtime
	extractedVars map[string]string
	existingVars  map[string]string
	// response context
	responseData    interface{}
	responseText    string
	responseStatus  int
	responseHeaders map[string]string
	// request context
	requestMethod  string
	requestURL     string
	requestBody    string
	requestHeaders map[string]string
	requestObject  map[string]interface{}
}

// NewScriptEngine creates a new JavaScript execution environment
func NewScriptEngine(existingVars map[string]string) *ScriptEngine {
	se := &ScriptEngine{
		vm:            goja.New(),
		extractedVars: make(map[string]string),
		existingVars:  existingVars,
	}

	se.setupEnvironment()
	return se
}

// setupEnvironment configures the Postman-compatible JavaScript environment
func (se *ScriptEngine) setupEnvironment() {
	// Create pm object (Postman API)
	// Prepare request object placeholder; values are set via SetRequestData per execution
	se.requestObject = map[string]interface{}{
		"method": "",
		"url":    "",
		"body":   "",
		"json":   se.requestJson,
		"headers": map[string]interface{}{
			"get": se.requestHeadersGet,
		},
	}

	pm := map[string]interface{}{
		"environment": map[string]interface{}{
			"set": se.environmentSet,
			"get": se.environmentGet,
		},
		"globals": map[string]interface{}{
			"set": se.globalsSet,
			"get": se.globalsGet,
		},
		"collectionVariables": map[string]interface{}{
			"set": se.collectionVariablesSet,
			"get": se.collectionVariablesGet,
		},
		"variables": map[string]interface{}{
			"get": se.variablesGet,
		},
		"request": se.requestObject,
		"response": map[string]interface{}{
			"json": se.responseJson,
			"text": se.responseTextFn,
			"code": se.responseCode,
			"headers": map[string]interface{}{
				"get": se.responseHeadersGet,
			},
		},
	}

	// Set pm object in VM
	se.vm.Set("pm", pm)

	// Set up console for debugging
	console := map[string]interface{}{
		"log": func(args ...interface{}) {
			fmt.Printf("   [Script Log] %v\n", args)
		},
		"error": func(args ...interface{}) {
			fmt.Printf("   [Script Error] %v\n", args)
		},
		"warn": func(args ...interface{}) {
			fmt.Printf("   [Script Warn] %v\n", args)
		},
	}
	se.vm.Set("console", console)
}

// SetResponseData sets the response context for post-scripts
func (se *ScriptEngine) SetResponseData(jsonData interface{}, text string, status int, headers map[string]string) {
	se.responseData = jsonData
	se.responseText = text
	se.responseStatus = status
	se.responseHeaders = headers
}

// SetRequestData sets the request context for scripts
func (se *ScriptEngine) SetRequestData(method, url, body string, headers map[string]string) {
	se.requestMethod = method
	se.requestURL = url
	se.requestBody = body
	se.requestHeaders = headers
	if se.requestObject != nil {
		se.requestObject["method"] = method
		se.requestObject["url"] = url
		se.requestObject["body"] = body
	}
}

// Execute runs the JavaScript code and returns any errors
func (se *ScriptEngine) Execute(script string) error {
	// Add panic recovery for script execution
	var execErr error
	defer func() {
		if r := recover(); r != nil {
			execErr = &models.ScriptExecutionError{
				ScriptType: "unknown",
				APIName:    "",
				Cause:      fmt.Errorf("panic during script execution: %v", r),
			}
		}
	}()

	_, err := se.vm.RunString(script)
	if err != nil {
		// Wrap the error with more context
		return &models.ScriptExecutionError{
			ScriptType: "unknown",
			APIName:    "",
			Cause:      fmt.Errorf("script execution failed: %w", err),
		}
	}

	if execErr != nil {
		return execErr
	}

	return nil
}

// GetExtractedVariables returns all variables that were set during script execution
func (se *ScriptEngine) GetExtractedVariables() map[string]string {
	return se.extractedVars
}

// Environment variable methods
func (se *ScriptEngine) environmentSet(key string, value interface{}) {
	strValue := fmt.Sprintf("%v", value)
	se.extractedVars[key] = strValue
	fmt.Printf("   📝 pm.environment.set('%s', '%s')\n", key, strValue)
}

func (se *ScriptEngine) environmentGet(key string) interface{} {
	// First check extracted vars (from current execution)
	if val, exists := se.extractedVars[key]; exists {
		return val
	}
	// Then check existing vars (from previous APIs)
	if val, exists := se.existingVars[key]; exists {
		return val
	}
	return nil
}

// Globals variable methods (same as environment for our purposes)
func (se *ScriptEngine) globalsSet(key string, value interface{}) {
	strValue := fmt.Sprintf("%v", value)
	se.extractedVars[key] = strValue
	fmt.Printf("   📝 pm.globals.set('%s', '%s')\n", key, strValue)
}

func (se *ScriptEngine) globalsGet(key string) interface{} {
	if val, exists := se.extractedVars[key]; exists {
		return val
	}
	if val, exists := se.existingVars[key]; exists {
		return val
	}
	return nil
}

// Collection variable methods (same as environment for our purposes)
func (se *ScriptEngine) collectionVariablesSet(key string, value interface{}) {
	strValue := fmt.Sprintf("%v", value)
	se.extractedVars[key] = strValue
	fmt.Printf("   📝 pm.collectionVariables.set('%s', '%s')\n", key, strValue)
}

func (se *ScriptEngine) collectionVariablesGet(key string) interface{} {
	if val, exists := se.extractedVars[key]; exists {
		return val
	}
	if val, exists := se.existingVars[key]; exists {
		return val
	}
	return nil
}

// Generic variables get (checks all variable scopes)
func (se *ScriptEngine) variablesGet(key string) interface{} {
	return se.environmentGet(key)
}

// Response methods for post-scripts
func (se *ScriptEngine) responseJson() interface{} {
	return se.responseData
}

func (se *ScriptEngine) responseTextFn() string {
	return se.responseText
}

func (se *ScriptEngine) responseCode() int {
	return se.responseStatus
}

func (se *ScriptEngine) responseHeadersGet(name string) string {
	if se.responseHeaders == nil {
		return ""
	}

	// Try exact match first
	if val, exists := se.responseHeaders[name]; exists {
		return val
	}

	// Try case-insensitive match
	for k, v := range se.responseHeaders {
		if strings.EqualFold(k, name) {
			return v
		}
	}

	return ""
}

// Request helpers
func (se *ScriptEngine) requestHeadersGet(name string) string {
	if se.requestHeaders == nil {
		return ""
	}
	if val, exists := se.requestHeaders[name]; exists {
		return val
	}
	for k, v := range se.requestHeaders {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return ""
}

func (se *ScriptEngine) requestJson() interface{} {
	if se.requestBody == "" {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(se.requestBody), &v); err == nil {
		return v
	}
	return nil
}
