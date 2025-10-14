package models

import (
	"encoding/json"
)

// MockExpectation represents a complete mock server expectation
// This is the primary model used throughout the application for building and managing expectations
type MockExpectation struct {
	// Identification
	Name        string `json:"name,omitempty"`        // User-friendly name for identification
	Description string `json:"description,omitempty"` // Optional detailed description

	// Request matching
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	QueryParams map[string]string `json:"queryParams,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	HeaderTypes map[string]string `json:"headerTypes,omitempty"` // "exact" or "regex"
	Body        interface{}       `json:"body,omitempty"`

	// Response
	StatusCode      int               `json:"statusCode"`
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`
	ResponseBody    interface{}       `json:"responseBody"`

	// Advanced features
	ResponseDelay     string             `json:"responseDelay,omitempty"`
	Times             *Times             `json:"times,omitempty"`
	Callbacks         *CallbackConfig    `json:"callbacks,omitempty"`
	ConnectionOptions *ConnectionOptions `json:"connectionOptions,omitempty"`
	Priority          int                `json:"priority,omitempty"`
}

// Times represents MockServer times configuration
type Times struct {
	RemainingTimes int  `json:"remainingTimes,omitempty"`
	Unlimited      bool `json:"unlimited"`
}

// CallbackConfig represents MockServer callback configuration
type CallbackConfig struct {
	CallbackClass string        `json:"callbackClass,omitempty"`
	HttpCallback  *HttpCallback `json:"httpCallback,omitempty"`
}

// HttpCallback represents HTTP callback configuration
type HttpCallback struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    interface{}       `json:"body,omitempty"`
}

// ConnectionOptions represents MockServer connection options
type ConnectionOptions struct {
	SuppressConnectionErrors bool `json:"suppressConnectionErrors,omitempty"`
	SuppressContentLength    bool `json:"suppressContentLength,omitempty"`
	ChunkedEncoding          bool `json:"chunkedEncoding,omitempty"`
	KeepAlive                bool `json:"keepAlive,omitempty"`
	CloseSocket              bool `json:"closeSocket,omitempty"`
	DropConnection           bool `json:"dropConnection,omitempty"`
}

// PathMatchingStrategy represents how paths should be matched
type PathMatchingStrategy string

const (
	PathExact PathMatchingStrategy = "exact"
	PathRegex PathMatchingStrategy = "regex"
)

// QueryParamMatchingStrategy represents how query parameters should be matched
type QueryParamMatchingStrategy string

const (
	QueryExact  QueryParamMatchingStrategy = "exact"
	QueryRegex  QueryParamMatchingStrategy = "regex"
	QuerySubset QueryParamMatchingStrategy = "subset"
)

// RequestBodyMatchingStrategy represents how request body should be matched
type RequestBodyMatchingStrategy string

const (
	BodyExact   RequestBodyMatchingStrategy = "exact"
	BodyPartial RequestBodyMatchingStrategy = "partial"
	BodyRegex   RequestBodyMatchingStrategy = "regex"
)

// ExpectationsToMockServerJSON converts expectations to MockServer JSON format
func ExpectationsToMockServerJSON(expectations []MockExpectation) string {
	var mockServerExpectations []map[string]interface{}

	for _, expectation := range expectations {
		mockServerExp := map[string]interface{}{
			"httpRequest":  buildHttpRequest(expectation),
			"httpResponse": buildHttpResponse(expectation),
		}

		// Add times if specified
		if expectation.Times != nil {
			mockServerExp["times"] = expectation.Times
		}

		// Add priority if specified
		if expectation.Priority != 0 {
			mockServerExp["priority"] = expectation.Priority
		}

		// Add callbacks if specified
		if expectation.Callbacks != nil {
			if expectation.Callbacks.HttpCallback != nil {
				mockServerExp["httpCallback"] = expectation.Callbacks.HttpCallback
			}
			if expectation.Callbacks.CallbackClass != "" {
				mockServerExp["callback"] = map[string]interface{}{
					"callbackClass": expectation.Callbacks.CallbackClass,
				}
			}
		}

		// Add connection options if specified
		if expectation.ConnectionOptions != nil {
			mockServerExp["connectionOptions"] = expectation.ConnectionOptions
		}

		mockServerExpectations = append(mockServerExpectations, mockServerExp)
	}

	jsonBytes, err := json.MarshalIndent(mockServerExpectations, "", "  ")
	if err != nil {
		return "[]" // Fallback to empty array
	}

	return string(jsonBytes)
}

// buildHttpRequest builds the httpRequest part of MockServer expectation
func buildHttpRequest(expectation MockExpectation) map[string]interface{} {
	request := map[string]interface{}{
		"method": expectation.Method,
		"path":   expectation.Path,
	}

	// Add query parameters if present
	if len(expectation.QueryParams) > 0 {
		queryParams := make(map[string][]string)
		for key, value := range expectation.QueryParams {
			queryParams[key] = []string{value}
		}
		request["queryStringParameters"] = queryParams
	}

	// Add headers if present
	if len(expectation.Headers) > 0 {
		headers := make(map[string]interface{})
		for key, value := range expectation.Headers {
			// Check if this header should use regex matching
			if expectation.HeaderTypes != nil && expectation.HeaderTypes[key] == "regex" {
				headers[key] = map[string]interface{}{
					"matcher": "regex",
					"value":   value,
				}
			} else {
				// Default to exact matching
				headers[key] = value
			}
		}
		request["headers"] = headers
	}

	// Add body if present
	if expectation.Body != nil {
		request["body"] = expectation.Body
	}

	return request
}

// buildHttpResponse builds the httpResponse part of MockServer expectation
func buildHttpResponse(expectation MockExpectation) map[string]interface{} {
	response := map[string]interface{}{
		"statusCode": expectation.StatusCode,
		"body":       expectation.ResponseBody,
	}

	// Add default headers
	headers := map[string]string{
		"Content-Type":                 "application/json",
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type, Authorization",
	}

	// Merge with custom headers
	for key, value := range expectation.ResponseHeaders {
		headers[key] = value
	}

	response["headers"] = headers

	// Add delay if specified
	if expectation.ResponseDelay != "" {
		response["delay"] = map[string]interface{}{
			"timeUnit": "MILLISECONDS",
			"value":    expectation.ResponseDelay,
		}
	}

	return response
}
