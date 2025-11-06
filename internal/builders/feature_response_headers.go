package builders

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

// ControlContentLengthHeaders prompts for response headers and stores them in
// exp.HttpResponse.Headers ([]NameValues). If duplicate (case-insensitive)
// names are detected, it reverts to the original expectation.
func ControlContentLengthHeaders() FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("\n📋 Custom Response Headers Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		// Ensure ConnectionOptions is initialized
		if exp.HttpResponse.ConnectionOptions == nil {
			exp.HttpResponse.ConnectionOptions = &models.ConnectionOptions{}
		}

		// add selection between contentLengthHeaderOverride and suppressContentLengthHeader.
		// Only one could be chosen.
		var choice string
		if err := survey.AskOne(&survey.Select{
			Message: "Choose Content-Length header action:",
			Options: []string{
				"Set Content-Length header value",
				"Suppress Content-Length header",
				"Skip",
			},
		}, &choice); err != nil {
			return err
		}

		if choice == "Set Content-Length header value" {
			// Prompt for Content-Length header value
			var contentLengthHeader string
			if err := survey.AskOne(&survey.Input{
				Message: "Set Content-Length header value (leave blank to skip):",
				Help:    "Specify a value for the Content-Length header",
			}, &contentLengthHeader); err != nil {
				return nil
			}
			if contentLengthHeader != "" {
				val, err := strconv.Atoi(strings.TrimSpace(contentLengthHeader))
				if err != nil || val < 0 {
					fmt.Printf("invalid Content-Length value: %q, skipping setting Content-Length header\n", contentLengthHeader)
					return nil
				}
				exp.HttpResponse.ConnectionOptions.ContentLengthOverride = val
				fmt.Printf("✅ Content-Length header set to: %d\n", val)
			}
			return nil
		} else if choice == "Suppress Content-Length header" {
			exp.HttpResponse.ConnectionOptions.SuppressContentLengthHeader = true
			fmt.Println("✅ Content-Length header will be suppressed")
		}
		return nil
	}
}
