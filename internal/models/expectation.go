package models

import (
	"encoding/json"
	"fmt"
)

// MockExpectation represents a complete mock server expectation
// This is the primary model used throughout the application for building and managing expectations
type MockExpectation struct {
	// Identification
	ID          string `json:"id,omitempty"`          // Unique identifier for the expectation
	Description string `json:"description,omitempty"` // Optional detailed description
	Priority    int    `json:"priority,omitempty"`

	HttpRequest  *HttpRequest  `json:"httpRequest,omitempty"`
	HttpResponse *HttpResponse `json:"httpResponse,omitempty"`
	Forward      *HttpForward  `json:"httpForward,omitempty"`

	Times             *Times             `json:"times,omitempty"`
	ConnectionOptions *ConnectionOptions `json:"connectionOptions,omitempty"`
	Progressive       *Progressive       `json:"-"`
}

type Progressive struct {
	Base int
	Step int
	Cap  int
}

type HttpRequest struct {
	Method                string              `json:"method,omitempty"`
	Path                  string              `json:"path,omitempty"`
	PathParameters        map[string][]string `json:"pathParameters,omitempty"`
	QueryStringParameters map[string][]string `json:"queryStringParameters,omitempty"`
	Headers               []NameValues        `json:"headers,omitempty"`
	Body                  any                 `json:"body,omitempty"`
}

type HttpResponse struct {
	StatusCode int          `json:"statusCode,omitempty"`
	Body       any          `json:"body,omitempty"`
	Headers    []NameValues `json:"headers,omitempty"`
	Cookies    []NameValues `json:"cookies,omitempty"`
	Delay      *Delay       `json:"delay,omitempty"`
}

type NameValues struct {
	Name   string   `json:"name,omitempty"`
	Values []string `json:"values,omitempty"`
}

type HttpForward struct {
	Scheme string `json:"scheme,omitempty"` // "HTTP", "HTTPS"
	Host   string `json:"host,omitempty"`
	Port   int    `json:"port,omitempty"`
}

// Times represents MockServer times configuration
type Times struct {
	RemainingTimes int  `json:"remainingTimes,omitempty"`
	Unlimited      bool `json:"unlimited"`
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
	jsonBytes, err := json.MarshalIndent(expectations, "", "  ")
	if err != nil {
		fmt.Printf("❌ Failed to marshal expectations: %v\n", err)
		return "[]"
	}
	return string(jsonBytes)
}
