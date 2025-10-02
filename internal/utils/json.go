package utils

import (
	"encoding/json"
	"fmt"
)

// PrettyPrintJSON takes a JSON string and returns a pretty-printed version of it
func PrettyPrintJSON(jsonStr string) (string, error) {
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	prettyJSON, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format JSON: %w", err)
	}

	return string(prettyJSON), nil
}
