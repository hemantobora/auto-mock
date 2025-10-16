package builders

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

func applyDelays(mc *MockConfigurator) func(exp *MockExpectation) error {
	return func(exp *MockExpectation) error {
		fmt.Println("\n⏱️  Response Delay Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		ensureMaps(exp)

		var mode string
		if err := survey.AskOne(&survey.Select{
			Message: "Select delay mode:",
			Options: []string{
				"fixed - single delay (ms)",
				"range - random between min-max (ms). Auto mock will randomly pick",
				"progressive - grows across hits",
			},
			Default: "fixed - single delay (ms)",
		}, &mode, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		switch {
		case strings.HasPrefix(mode, "fixed"):
			var fixedStr string
			if err := survey.AskOne(&survey.Input{
				Message: "Delay in milliseconds (e.g., 500):",
				Default: "500",
			}, &fixedStr, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
			val, err := strconv.Atoi(strings.TrimSpace(fixedStr))
			if err != nil || val < 0 {
				return fmt.Errorf("invalid delay: %q", fixedStr)
			}
			exp.HttpResponse.Delay = &Delay{TimeUnit: "MILLISECONDS", Value: val}

		case strings.HasPrefix(mode, "range"):
			var rng string
			if err := survey.AskOne(&survey.Input{
				Message: "Range in ms as min-max (e.g., 400-900):",
				Default: "400-900",
			}, &rng, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
			min, max, err := parseRange(rng)
			if err != nil {
				return err
			}
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			pick := min + r.Intn(max-min+1)
			exp.HttpResponse.Delay = &Delay{TimeUnit: "MILLISECONDS", Value: pick}

			// Optional: annotate so users know it was range-chosen
			exp.Description = strings.TrimSpace(exp.Description + fmt.Sprintf(" [delay %dms from %d-%d]", pick, min, max))

		case strings.HasPrefix(mode, "progressive"):
			// Simple progressive pattern: base, step, max
			var baseStr, stepStr, maxStr string
			if err := survey.AskOne(&survey.Input{Message: "Base delay (ms):", Default: "200"}, &baseStr); err != nil {
				return err
			}
			if err := survey.AskOne(&survey.Input{Message: "Increment per hit (ms):", Default: "100"}, &stepStr); err != nil {
				return err
			}
			if err := survey.AskOne(&survey.Input{Message: "Max delay cap (ms):", Default: "1500"}, &maxStr); err != nil {
				return err
			}

			base, err1 := strconv.Atoi(strings.TrimSpace(baseStr))
			step, err2 := strconv.Atoi(strings.TrimSpace(stepStr))
			capv, err3 := strconv.Atoi(strings.TrimSpace(maxStr))
			if err := firstErr(err1, err2, err3); err != nil || base < 0 || step < 0 || capv < base {
				return fmt.Errorf("invalid progressive inputs")
			}

			exp.HttpResponse.Delay = &Delay{TimeUnit: "MILLISECONDS", Value: base} // Clear any fixed delay
			exp.Times = &models.Times{
				RemainingTimes: 1,
				Unlimited:      false,
			}
			exp.Progressive = &Progressive{Base: base, Step: step, Cap: capv}
			// Since MockServer itself only supports a single fixed "delay" per expectation,
			// we snapshot a reasonable starting delay and annotate the scheme so your CLI can rotate the delay across re-applies if you want.
			exp.Description = strings.TrimSpace(exp.Description +
				fmt.Sprintf(" [progressive delay: base=%d, step=%d, cap=%d]", base, step, capv))
		default:
			return fmt.Errorf("unsupported delay mode")
		}

		fmt.Println("✅ Delay configured.")
		fmt.Println("\n📚 MockServer Delay Documentation:")
		fmt.Println("   Delay Configuration: https://mock-server.com/mock_server/response_delays.html")
		fmt.Println("   Advanced Timing: https://mock-server.com/mock_server/times.html")

		return nil
	}
}

func parseRange(s string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected min-max format")
	}
	min, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	max, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err := firstErr(err1, err2); err != nil || min < 0 || max < min {
		return 0, 0, fmt.Errorf("invalid range %q", s)
	}
	return min, max, nil
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
