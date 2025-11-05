package builders

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

// applyCaching returns a FeatureFunc that collects cache control configuration
// and stores it into exp.HttpResponse.Headers ([]NameValues).
func applyCaching() FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("\n🗄️  Cache Control Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		// ensure slice exists
		if exp.HttpResponse.Headers == nil {
			exp.HttpResponse.Headers = []models.NameValues{}
		}

		// Cache-Control
		var cc string
		if err := survey.AskOne(&survey.Select{
			Message: "Cache policy:",
			Options: []string{
				"no-store",
				"no-cache",
				"private, max-age=60",
				"public, max-age=300",
				"custom",
			},
			Default: "no-cache",
		}, &cc, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		switch cc {
		case "no-store":
			SetNameValues(&exp.HttpResponse.Headers, "Cache-Control", []string{"no-store"})
		case "no-cache":
			SetNameValues(&exp.HttpResponse.Headers, "Cache-Control", []string{"no-cache"})
		case "private, max-age=60":
			SetNameValues(&exp.HttpResponse.Headers, "Cache-Control", []string{"private, max-age=60"})
		case "public, max-age=300":
			SetNameValues(&exp.HttpResponse.Headers, "Cache-Control", []string{"public, max-age=300"})
		case "custom":
			var custom string
			if err := survey.AskOne(&survey.Input{
				Message: "Enter Cache-Control value:",
				Default: "public, max-age=120",
			}, &custom, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
			SetNameValues(&exp.HttpResponse.Headers, "Cache-Control", []string{strings.TrimSpace(custom)})
		}

		// ETag
		var addETag bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Generate ETag from response body?",
			Default: true,
		}, &addETag); err != nil {
			return err
		}
		if addETag {
			etag := computeETag(exp.HttpResponse.Body)
			SetNameValues(&exp.HttpResponse.Headers, "ETag", []string{etag})
		}

		fmt.Println("\n📚 Cache Control Resources:")
		fmt.Println("   MDN Cache-Control: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cache-Control")
		fmt.Println("   ETag Documentation: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/ETag")
		return nil
	}
}

func computeETag(body any) string {
	// strong ETag based on SHA-1 of canonical JSON
	b, _ := json.Marshal(body)
	sum := sha1.Sum(b)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}
