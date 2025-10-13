package collections

import (
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/hemantobora/auto-mock/internal/models"
)

// ScriptEngine provides a JavaScript execution environment for pre/post scripts
type ScriptEngine struct {
	vm              *goja.Runtime
	extractedVars   map[string]string
	existingVars    map[string]string
	responseData    interface{}
	responseHeaders map[string]string
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
		"request": map[string]interface{}{
			// Can be extended if needed for pre-scripts
		},
		"response": map[string]interface{}{
			"json": se.responseJson,
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

// SetResponseData sets the response data for post-scripts
func (se *ScriptEngine) SetResponseData(jsonData interface{}, headers map[string]string) {
	se.responseData = jsonData
	se.responseHeaders = headers
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
