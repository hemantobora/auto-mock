package models

import (
	"encoding/json"
	"regexp"
)

// ValidateJSON validates if a string is valid JSON
func ValidateJSON(jsonStr string) error {
	var temp interface{}
	if err := json.Unmarshal([]byte(jsonStr), &temp); err != nil {
		return &JSONValidationError{
			Context: "JSON validation",
			Content: jsonStr,
			Cause:   err,
		}
	}
	return nil
}

// FormatJSON formats JSON string with proper indentation
func FormatJSON(jsonStr string) (string, error) {
	var temp interface{}
	if err := json.Unmarshal([]byte(jsonStr), &temp); err != nil {
		return jsonStr, &JSONValidationError{
			Context: "JSON formatting",
			Content: jsonStr,
			Cause:   err,
		}
	}

	formatted, err := json.MarshalIndent(temp, "", "  ")
	if err != nil {
		return jsonStr, &JSONValidationError{
			Context: "JSON marshaling",
			Content: jsonStr,
			Cause:   err,
		}
	}

	return string(formatted), nil
}

// IsValidRegex tests if a regex pattern is valid
func IsValidRegex(pattern string) error {
	if _, err := regexp.Compile(pattern); err != nil {
		return &RegexValidationError{
			Pattern: pattern,
			Context: "regex compilation",
			Cause:   err,
		}
	}
	return nil
}
