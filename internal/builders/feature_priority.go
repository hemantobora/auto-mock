package builders

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

func applyPriority() FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("\n⚖️  Expectation Priority Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		fmt.Println("\n💡 Priority Explanation:")
		fmt.Println("   • Lower numbers = higher priority (0 is highest)")
		fmt.Println("   • Higher priority expectations are matched first")
		fmt.Println("   • Use this to resolve conflicts between overlapping expectations")
		fmt.Println("   • Example: Specific /users/123 before generic /users/{id}")
		original := CloneExpectation(exp)

		var pStr string
		if err := survey.AskOne(&survey.Input{
			Message: "Priority (higher wins). Suggest 0..100:",
			Default: "10",
		}, &pStr, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		p, err := strconv.Atoi(strings.TrimSpace(pStr))
		if err != nil || p < 0 {
			*exp = *original
			return fmt.Errorf("invalid priority: %q", pStr)
		}
		exp.Priority = p
		fmt.Printf("✅ Priority set to: %d\n", p)

		fmt.Println("\n📚 MockServer Priority Documentation:")
		fmt.Println("   Priority Guide: https://mock-server.com/mock_server/expectations.html#priority")
		return nil
	}
}
