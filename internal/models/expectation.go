// Package models provides shared data structures
package models

// APIExpectation represents a single API expectation with UI-friendly fields
type APIExpectation struct {
	ID          string                 `json:"id"`
	Method      string                 `json:"method"`       // GET, POST, etc.
	Path        string                 `json:"path"`         // /api/users
	Description string                 `json:"description"`  // User-friendly description
	Raw         map[string]interface{} `json:"raw"`          // Full MockServer expectation
}

// ExpectationStats provides statistics about expectations
type ExpectationStats struct {
	Total        int            `json:"total"`
	ByMethod     map[string]int `json:"by_method"`
	ByStatusCode map[int]int    `json:"by_status_code"`
}
