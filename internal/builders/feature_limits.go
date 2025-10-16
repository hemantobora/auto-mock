package builders

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

func applyLimits(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("\n🔢 Response Limits Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		var mode string
		if err := survey.AskOne(&survey.Select{
			Message: "Limit mode:",
			Options: []string{"unlimited", "fixed-count"},
			Default: "fixed-count",
		}, &mode, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		if exp.Times == nil {
			exp.Times = &Times{}
		}

		switch mode {
		case "unlimited":
			exp.Times.Unlimited = true
			exp.Times.RemainingTimes = 0
		case "fixed-count":
			var nStr string
			if err := survey.AskOne(&survey.Input{
				Message: "How many times should this expectation be served?",
				Default: "1",
			}, &nStr, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
			n, err := strconv.Atoi(strings.TrimSpace(nStr))
			if err != nil || n <= 0 {
				return fmt.Errorf("invalid count: %q", nStr)
			}
			exp.Times.Unlimited = false
			exp.Times.RemainingTimes = n
		default:
			return fmt.Errorf("unknown limit mode")
		}
		if exp.Times.Unlimited {
			fmt.Println("✅ Response limit: unlimited")
		} else {
			fmt.Printf("✅ Response limit: %d times\n", exp.Times.RemainingTimes)
		}

		// Add advanced rate limiting guidance
		fmt.Println("\n📚 Advanced Rate Limiting Patterns:")
		fmt.Println("   • Create additional expectation for post-limit behavior")
		fmt.Println("   • Use 429 status code for rate limit exceeded responses")
		fmt.Println("   • Include Retry-After header for client guidance")

		fmt.Println("\n📚 MockServer Times Documentation:")
		fmt.Println("   Times Configuration: https://mock-server.com/mock_server/times.html")
		fmt.Println("   Rate Limiting Guide: https://mock-server.com/mock_server/response_delays.html")
		return nil
	}
}
