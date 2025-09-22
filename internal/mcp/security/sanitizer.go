package security

import (
    "encoding/json"
    "fmt"
    "regexp"
    "strings"
)

// CollectionSanitizer removes all credentials and sensitive data from API collections
// This ensures that NO credentials are ever sent to LLMs or stored insecurely
type CollectionSanitizer struct {
    // Patterns to identify and remove sensitive data
    credentialPatterns   []*regexp.Regexp
    sensitiveKeys       map[string]bool
    authTokenPatterns   []*regexp.Regexp
}

// SanitizedCollection represents a collection with all credentials removed
type SanitizedCollection struct {
    Name                string                 `json:"name"`
    Description         string                 `json:"description"`
    Endpoints           []SanitizedEndpoint    `json:"endpoints"`
    Variables           map[string]string      `json:"variables"`           // Only non-sensitive variables
    AuthSchemes         []AuthSchemeInfo       `json:"auth_schemes"`        // Schema info only, no credentials
    OriginalFormat      string                 `json:"original_format"`     // postman/bruno/insomnia
    EndpointCount       int                    `json:"endpoint_count"`
    HasAuthentication   bool                   `json:"has_authentication"`
}

// SanitizedEndpoint represents an API endpoint without any credentials
type SanitizedEndpoint struct {
    Method              string                 `json:"method"`
    Path                string                 `json:"path"`
    Name                string                 `json:"name,omitempty"`
    Description         string                 `json:"description,omitempty"`
    Parameters          []ParameterInfo        `json:"parameters,omitempty"`
    RequestBodySchema   interface{}            `json:"request_body_schema,omitempty"`
    ResponseExamples    []ResponseExample      `json:"response_examples,omitempty"`
    Tags                []string               `json:"tags,omitempty"`
    RequiresAuth        bool                   `json:"requires_auth"`       // Flag only, no credentials
    ContentType         string                 `json:"content_type,omitempty"`
}

// AuthSchemeInfo contains information about authentication schemes WITHOUT credentials
type AuthSchemeInfo struct {
    Type                string                 `json:"type"`                // bearer, basic, api_key, oauth2, etc.
    Location            string                 `json:"location,omitempty"`  // header, query, cookie
    Name                string                 `json:"name,omitempty"`      // header name, query param name
    Description         string                 `json:"description,omitempty"`
    Scheme              string                 `json:"scheme,omitempty"`    // Bearer, Basic, etc.
    // CRITICAL: NO credential fields - never store actual tokens/keys/passwords
}

// Supporting types
type ParameterInfo struct {
    Name                string                 `json:"name"`
    Type                string                 `json:"type"`
    Location            string                 `json:"location"`      // query, path, header, body
    Required            bool                   `json:"required"`
    Description         string                 `json:"description,omitempty"`
    Example             interface{}            `json:"example,omitempty"`
}

type ResponseExample struct {
    StatusCode          int                    `json:"status_code"`
    Description         string                 `json:"description,omitempty"`
    ContentType         string                 `json:"content_type"`
    Body                interface{}            `json:"body,omitempty"`
    Headers             map[string]string      `json:"headers,omitempty"`
}

// SanitizationResult contains both sanitized data and extracted credentials
type SanitizationResult struct {
    SanitizedCollection *SanitizedCollection   `json:"sanitized_collection"`
    ExtractedCredentials map[string]interface{} `json:"-"` // Never serialized - for API invocation only
    CredentialLocations  []CredentialLocation   `json:"credential_locations"`
    SecurityWarnings     []string               `json:"security_warnings"`
}

// CredentialLocation tracks where credentials were found (for user information)
type CredentialLocation struct {
    Location            string                 `json:"location"`      // "auth.bearer", "variable.api_key", etc.
    Type                string                 `json:"type"`          // "token", "password", "api_key"
    Description         string                 `json:"description"`
    EndpointPath        string                 `json:"endpoint_path,omitempty"`
}

// NewCollectionSanitizer creates a new sanitizer with security patterns
func NewCollectionSanitizer() *CollectionSanitizer {
    sanitizer := &CollectionSanitizer{
        credentialPatterns: make([]*regexp.Regexp, 0),
        sensitiveKeys:      make(map[string]bool),
        authTokenPatterns:  make([]*regexp.Regexp, 0),
    }
    
    // Initialize security patterns
    sanitizer.initializeSecurityPatterns()
    
    return sanitizer
}

// SanitizeCollection removes all credentials from collection data
func (cs *CollectionSanitizer) SanitizeCollection(rawCollection []byte, format string) (*SanitizationResult, error) {
    switch strings.ToLower(format) {
    case "postman":
        return cs.sanitizePostmanCollection(rawCollection)
    case "bruno":
        return cs.sanitizeBrunoCollection(rawCollection)
    case "insomnia":
        return cs.sanitizeInsomniaCollection(rawCollection)
    default:
        return nil, fmt.Errorf("unsupported collection format: %s", format)
    }
}

// sanitizePostmanCollection handles Postman collection format
func (cs *CollectionSanitizer) sanitizePostmanCollection(rawData []byte) (*SanitizationResult, error) {
    var postmanCollection map[string]interface{}
    if err := json.Unmarshal(rawData, &postmanCollection); err != nil {
        return nil, fmt.Errorf("invalid Postman collection JSON: %w", err)
    }
    
    result := &SanitizationResult{
        ExtractedCredentials: make(map[string]interface{}),
        CredentialLocations:  make([]CredentialLocation, 0),
        SecurityWarnings:     make([]string, 0),
    }
    
    // Extract basic info
    info := cs.extractPostmanInfo(postmanCollection)
    
    // Process items (endpoints)
    endpoints, authSchemes := cs.processPostmanItems(postmanCollection, result)
    
    // Process variables (sanitize sensitive ones)
    variables := cs.sanitizePostmanVariables(postmanCollection, result)
    
    // Create sanitized collection
    result.SanitizedCollection = &SanitizedCollection{
        Name:              info["name"],
        Description:       info["description"],
        Endpoints:         endpoints,
        Variables:         variables,
        AuthSchemes:       authSchemes,
        OriginalFormat:    "postman",
        EndpointCount:     len(endpoints),
        HasAuthentication: len(authSchemes) > 0,
    }
    
    return result, nil
}

// sanitizeBrunoCollection handles Bruno collection format
func (cs *CollectionSanitizer) sanitizeBrunoCollection(rawData []byte) (*SanitizationResult, error) {
    // Bruno collections are typically .bru files, but can be JSON exports
    result := &SanitizationResult{
        ExtractedCredentials: make(map[string]interface{}),
        CredentialLocations:  make([]CredentialLocation, 0),
        SecurityWarnings:     make([]string, 0),
    }
    
    // Try to parse as JSON first
    var brunoData map[string]interface{}
    if err := json.Unmarshal(rawData, &brunoData); err != nil {
        // If not JSON, treat as .bru format
        return cs.parseBrunoTextFormat(string(rawData), result)
    }
    
    // Process JSON format Bruno collection
    endpoints := cs.processBrunoItems(brunoData, result)
    authSchemes := cs.extractBrunoAuthSchemes(brunoData, result)
    variables := cs.sanitizeBrunoVariables(brunoData, result)
    
    result.SanitizedCollection = &SanitizedCollection{
        Name:              cs.getStringValue(brunoData, "name", "Bruno Collection"),
        Description:       cs.getStringValue(brunoData, "description", ""),
        Endpoints:         endpoints,
        Variables:         variables,
        AuthSchemes:       authSchemes,
        OriginalFormat:    "bruno",
        EndpointCount:     len(endpoints),
        HasAuthentication: len(authSchemes) > 0,
    }
    
    return result, nil
}

// sanitizeInsomniaCollection handles Insomnia collection format
func (cs *CollectionSanitizer) sanitizeInsomniaCollection(rawData []byte) (*SanitizationResult, error) {
    var insomniaData map[string]interface{}
    if err := json.Unmarshal(rawData, &insomniaData); err != nil {
        return nil, fmt.Errorf("invalid Insomnia collection JSON: %w", err)
    }
    
    result := &SanitizationResult{
        ExtractedCredentials: make(map[string]interface{}),
        CredentialLocations:  make([]CredentialLocation, 0),
        SecurityWarnings:     make([]string, 0),
    }
    
    // Process Insomnia resources
    endpoints, authSchemes := cs.processInsomniaResources(insomniaData, result)
    variables := cs.sanitizeInsomniaEnvironments(insomniaData, result)
    
    result.SanitizedCollection = &SanitizedCollection{
        Name:              cs.getStringValue(insomniaData, "name", "Insomnia Collection"),
        Description:       cs.getStringValue(insomniaData, "description", ""),
        Endpoints:         endpoints,
        Variables:         variables,
        AuthSchemes:       authSchemes,
        OriginalFormat:    "insomnia",
        EndpointCount:     len(endpoints),
        HasAuthentication: len(authSchemes) > 0,
    }
    
    return result, nil
}

// initializeSecurityPatterns sets up patterns to identify credentials
func (cs *CollectionSanitizer) initializeSecurityPatterns() {
    // Patterns for common credential formats
    credentialPatterns := []string{
        `[Bb]earer\s+[A-Za-z0-9\-._~+/]+=*`,           // Bearer tokens
        `[Aa]pi[Kk]ey\s+[A-Za-z0-9\-._~+/]+=*`,       // API keys
        `[Aa]ccess[Tt]oken\s+[A-Za-z0-9\-._~+/]+=*`,  // Access tokens
        `pk_[a-zA-Z0-9_]{24,}`,                         // Stripe public keys
        `sk_[a-zA-Z0-9_]{24,}`,                         // Stripe secret keys
        `ghp_[A-Za-z0-9_]{36}`,                         // GitHub personal tokens
        `[A-Za-z0-9_]{40}`,                             // Generic 40-char tokens
    }
    
    for _, pattern := range credentialPatterns {
        if regex, err := regexp.Compile(pattern); err == nil {
            cs.credentialPatterns = append(cs.credentialPatterns, regex)
        }
    }
    
    // Sensitive keys that should be removed
    cs.sensitiveKeys = map[string]bool{
        "password":      true,
        "secret":        true,
        "token":         true,
        "key":           true,
        "api_key":       true,
        "apikey":        true,
        "access_token":  true,
        "refresh_token": true,
        "bearer":        true,
        "authorization": true,
        "x-api-key":     true,
        "x-auth-token":  true,
    }
}

// Helper methods for processing different collection formats

func (cs *CollectionSanitizer) extractPostmanInfo(collection map[string]interface{}) map[string]string {
    info := make(map[string]string)
    
    if infoObj, ok := collection["info"].(map[string]interface{}); ok {
        info["name"] = cs.getStringValue(infoObj, "name", "Unnamed Collection")
        info["description"] = cs.getStringValue(infoObj, "description", "")
    }
    
    return info
}

func (cs *CollectionSanitizer) processPostmanItems(collection map[string]interface{}, result *SanitizationResult) ([]SanitizedEndpoint, []AuthSchemeInfo) {
    var endpoints []SanitizedEndpoint
    var authSchemes []AuthSchemeInfo
    
    if items, ok := collection["item"].([]interface{}); ok {
        cs.processPostmanItemsRecursive(items, &endpoints, &authSchemes, result, "")
    }
    
    return endpoints, authSchemes
}

func (cs *CollectionSanitizer) processPostmanItemsRecursive(items []interface{}, endpoints *[]SanitizedEndpoint, authSchemes *[]AuthSchemeInfo, result *SanitizationResult, pathPrefix string) {
    for _, item := range items {
        if itemMap, ok := item.(map[string]interface{}); ok {
            // Check if it's a folder (has sub-items)
            if subItems, hasSubItems := itemMap["item"].([]interface{}); hasSubItems {
                folderName := cs.getStringValue(itemMap, "name", "")
                newPrefix := pathPrefix
                if folderName != "" {
                    if newPrefix != "" {
                        newPrefix += "/"
                    }
                    newPrefix += folderName
                }
                cs.processPostmanItemsRecursive(subItems, endpoints, authSchemes, result, newPrefix)
            } else {
                // Process individual request
                endpoint := cs.processPostmanRequest(itemMap, result, pathPrefix)
                if endpoint != nil {
                    *endpoints = append(*endpoints, *endpoint)
                }
                
                // Extract auth scheme if present
                if authScheme := cs.extractPostmanAuthScheme(itemMap, result); authScheme != nil {
                    *authSchemes = append(*authSchemes, *authScheme)
                }
            }
        }
    }
}

func (cs *CollectionSanitizer) processPostmanRequest(item map[string]interface{}, result *SanitizationResult, pathPrefix string) *SanitizedEndpoint {
    request, ok := item["request"].(map[string]interface{})
    if !ok {
        return nil
    }
    
    endpoint := &SanitizedEndpoint{
        Name:        cs.getStringValue(item, "name", ""),
        Description: cs.getStringValue(item, "description", ""),
        Method:      cs.getStringValue(request, "method", "GET"),
        RequiresAuth: false,
    }
    
    // Extract URL and path
    if url := cs.extractPostmanURL(request); url != "" {
        endpoint.Path = cs.sanitizeURLPath(url, result)
    }
    
    // Check for authentication
    if _, hasAuth := request["auth"]; hasAuth {
        endpoint.RequiresAuth = true
        cs.extractAndStoreCredentials(request["auth"], "request.auth", result)
    }
    
    // Process headers (remove sensitive ones)
    if headers := cs.processPostmanHeaders(request, result); len(headers) > 0 {
        // Store sanitized headers as parameters
        for name, value := range headers {
            endpoint.Parameters = append(endpoint.Parameters, ParameterInfo{
                Name:     name,
                Type:     "string",
                Location: "header",
                Example:  value,
            })
        }
    }
    
    // Process query parameters
    endpoint.Parameters = append(endpoint.Parameters, cs.processPostmanQueryParams(request, result)...)
    
    // Process request body
    if body := cs.processPostmanBody(request, result); body != nil {
        endpoint.RequestBodySchema = body
    }
    
    return endpoint
}

func (cs *CollectionSanitizer) extractPostmanURL(request map[string]interface{}) string {
    if urlObj, ok := request["url"].(map[string]interface{}); ok {
        if raw, ok := urlObj["raw"].(string); ok {
            return raw
        }
        
        // Reconstruct from parts
        if host, ok := urlObj["host"].([]interface{}); ok {
            if path, ok := urlObj["path"].([]interface{}); ok {
                var hostStr strings.Builder
                for i, h := range host {
                    if i > 0 {
                        hostStr.WriteString(".")
                    }
                    hostStr.WriteString(fmt.Sprintf("%v", h))
                }
                
                var pathStr strings.Builder
                for _, p := range path {
                    pathStr.WriteString("/")
                    pathStr.WriteString(fmt.Sprintf("%v", p))
                }
                
                return "https://" + hostStr.String() + pathStr.String()
            }
        }
    }
    
    if urlStr, ok := request["url"].(string); ok {
        return urlStr
    }
    
    return ""
}

// Security helper methods

func (cs *CollectionSanitizer) sanitizeURLPath(url string, result *SanitizationResult) string {
    // Remove credentials from URL
    for _, pattern := range cs.credentialPatterns {
        if pattern.MatchString(url) {
            result.SecurityWarnings = append(result.SecurityWarnings, "Credentials found in URL - removed for security")
            url = pattern.ReplaceAllString(url, "[REDACTED]")
        }
    }
    
    // Extract just the path part
    if strings.Contains(url, "://") {
        parts := strings.Split(url, "/")
        if len(parts) > 3 {
            return "/" + strings.Join(parts[3:], "/")
        }
    }
    
    return url
}

func (cs *CollectionSanitizer) extractAndStoreCredentials(authData interface{}, location string, result *SanitizationResult) {
    if authMap, ok := authData.(map[string]interface{}); ok {
        for key, value := range authMap {
            if cs.isSensitiveKey(key) {
                // Store credential for API invocation (not sent to LLM)
                result.ExtractedCredentials[location+"."+key] = value
                
                // Track location for user information
                result.CredentialLocations = append(result.CredentialLocations, CredentialLocation{
                    Location:    location + "." + key,
                    Type:        cs.getCredentialType(key),
                    Description: fmt.Sprintf("Found %s in %s", cs.getCredentialType(key), location),
                })
            }
        }
    }
}

func (cs *CollectionSanitizer) isSensitiveKey(key string) bool {
    keyLower := strings.ToLower(key)
    return cs.sensitiveKeys[keyLower] || strings.Contains(keyLower, "password") || strings.Contains(keyLower, "secret")
}

func (cs *CollectionSanitizer) getCredentialType(key string) string {
    keyLower := strings.ToLower(key)
    if strings.Contains(keyLower, "password") {
        return "password"
    }
    if strings.Contains(keyLower, "token") {
        return "token"
    }
    if strings.Contains(keyLower, "key") {
        return "api_key"
    }
    return "credential"
}

func (cs *CollectionSanitizer) getStringValue(obj map[string]interface{}, key, defaultValue string) string {
    if value, ok := obj[key].(string); ok {
        return value
    }
    return defaultValue
}

// Additional helper methods would be implemented for:
// - sanitizePostmanVariables
// - processPostmanHeaders
// - processPostmanQueryParams
// - processPostmanBody
// - extractPostmanAuthScheme
// - Bruno and Insomnia specific processing methods

// Placeholder implementations for brevity (would be fully implemented in production)

func (cs *CollectionSanitizer) sanitizePostmanVariables(collection map[string]interface{}, result *SanitizationResult) map[string]string {
    variables := make(map[string]string)
    // Implementation would extract variables and sanitize sensitive ones
    return variables
}

func (cs *CollectionSanitizer) processPostmanHeaders(request map[string]interface{}, result *SanitizationResult) map[string]string {
    headers := make(map[string]string)
    // Implementation would process headers and remove sensitive ones
    return headers
}

func (cs *CollectionSanitizer) processPostmanQueryParams(request map[string]interface{}, result *SanitizationResult) []ParameterInfo {
    var params []ParameterInfo
    // Implementation would extract query parameters
    return params
}

func (cs *CollectionSanitizer) processPostmanBody(request map[string]interface{}, result *SanitizationResult) interface{} {
    // Implementation would process request body
    return nil
}

func (cs *CollectionSanitizer) extractPostmanAuthScheme(item map[string]interface{}, result *SanitizationResult) *AuthSchemeInfo {
    // Implementation would extract auth scheme info without credentials
    return nil
}

// Bruno and Insomnia methods (simplified for brevity)
func (cs *CollectionSanitizer) parseBrunoTextFormat(content string, result *SanitizationResult) (*SanitizationResult, error) {
    // Implementation for Bruno .bru file format
    return result, fmt.Errorf("Bruno text format parsing not yet implemented")
}

func (cs *CollectionSanitizer) processBrunoItems(data map[string]interface{}, result *SanitizationResult) []SanitizedEndpoint {
    return []SanitizedEndpoint{}
}

func (cs *CollectionSanitizer) extractBrunoAuthSchemes(data map[string]interface{}, result *SanitizationResult) []AuthSchemeInfo {
    return []AuthSchemeInfo{}
}

func (cs *CollectionSanitizer) sanitizeBrunoVariables(data map[string]interface{}, result *SanitizationResult) map[string]string {
    return make(map[string]string)
}

func (cs *CollectionSanitizer) processInsomniaResources(data map[string]interface{}, result *SanitizationResult) ([]SanitizedEndpoint, []AuthSchemeInfo) {
    return []SanitizedEndpoint{}, []AuthSchemeInfo{}
}

func (cs *CollectionSanitizer) sanitizeInsomniaEnvironments(data map[string]interface{}, result *SanitizationResult) map[string]string {
    return make(map[string]string)
}

// ValidateNoCredentials ensures no credentials are present in data going to LLM
func (cs *CollectionSanitizer) ValidateNoCredentials(data interface{}) error {
    jsonData, err := json.Marshal(data)
    if err != nil {
        return err
    }
    
    content := string(jsonData)
    
    // Check against credential patterns
    for _, pattern := range cs.credentialPatterns {
        if pattern.MatchString(content) {
            return fmt.Errorf("potential credentials detected in data - sanitization failed")
        }
    }
    
    // Check for sensitive keys
    for key := range cs.sensitiveKeys {
        if strings.Contains(strings.ToLower(content), key) {
            return fmt.Errorf("sensitive key '%s' found in data - sanitization failed", key)
        }
    }
    
    return nil
}