package builders

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

// applyResponseHeaders returns a FeatureFunc that collects custom response headers
func applyResponseHeaders(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("\n📋 Custom Response Headers Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		fmt.Println("\n💡 Common Response Headers:")
		fmt.Println("   • X-Request-ID: Track requests")
		fmt.Println("   • X-RateLimit-*: Rate limiting info")
		fmt.Println("   • Cache-Control: Caching behavior")
		fmt.Println("   • X-Custom-*: Application-specific headers")

		ensureMaps(exp)
		original := CloneExpectation(exp)
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
			exp.HttpResponse.Headers[strings.TrimSpace(k)] = []string{v}
		}

		// sanity: forbid duplicate case-insensitive keys (MockServer normalizes)
		if dup := findCaseDupes(exp.HttpResponse.Headers); len(dup) > 0 {
			*exp = *original
			return fmt.Errorf("duplicate header keys (case-insensitive): %v", dup)
		}

		if oldLength < len(exp.HttpResponse.Headers) {
			fmt.Printf("✅ Custom response headers: %d configured\n", len(exp.HttpResponse.Headers))
		} else {
			fmt.Println("ℹ️  No custom response headers added")
		}
		return nil
	}
}

func findCaseDupes(m map[string][]string) []string {
	seen := map[string]string{}
	var d []string
	for k := range m {
		l := strings.ToLower(k)
		if prev, ok := seen[l]; ok {
			d = append(d, fmt.Sprintf("%q vs %q", prev, k))
		} else {
			seen[l] = k
		}
	}
	return d
}
