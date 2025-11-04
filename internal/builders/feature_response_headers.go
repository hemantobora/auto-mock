package builders

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

// applyResponseHeaders prompts for response headers and stores them in
// exp.HttpResponse.Headers ([]NameValues). If duplicate (case-insensitive)
// names are detected, it reverts to the original expectation.
func applyResponseHeaders() FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("\n📋 Custom Response Headers Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		fmt.Println("\n💡 Common Response Headers:")
		fmt.Println("   • X-Request-ID: Track requests")
		fmt.Println("   • X-RateLimit-*: Rate limiting info")
		fmt.Println("   • Cache-Control: Caching behavior")
		fmt.Println("   • X-Custom-*: Application-specific headers")

		// Ensure slice is initialized (keep your existing helper if you like)
		if exp.HttpResponse.Headers == nil {
			exp.HttpResponse.Headers = []models.NameValues{}
		}

		oldLength := len(exp.HttpResponse.Headers)

		for {
			var add bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Add / update a response header?",
				Default: len(exp.HttpResponse.Headers) == 0,
			}, &add); err != nil {
				return err
			}
			if !add {
				break
			}

			var k, v string
			if err := survey.AskOne(&survey.Input{Message: "Header name:"}, &k, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
			if err := survey.AskOne(&survey.Input{Message: "Header value:"}, &v, survey.WithValidator(survey.Required)); err != nil {
				return err
			}

			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			upsertHeader(&exp.HttpResponse.Headers, k, v)
		}

		if oldLength < len(exp.HttpResponse.Headers) {
			fmt.Printf("✅ Custom response headers: %d configured\n", len(exp.HttpResponse.Headers))
		} else {
			fmt.Println("ℹ️  No custom response headers added")
		}
		return nil
	}
}

// upsertHeader updates an existing header (case-insensitive) or appends a new one.
// If exists, it REPLACES the values with a single-value slice [v] (mirrors old behavior).
func upsertHeader(headers *[]models.NameValues, name, value string) {
	i := headerIndex(*headers, name)
	if i >= 0 {
		(*headers)[i].Values = []string{value}
		return
	}
	*headers = append(*headers, models.NameValues{
		Name:   name,
		Values: []string{value},
	})
}
