package builders

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
)

// FeatureFunc represents a function that configures a feature on an expectation
type FeatureFunc func(exp *MockExpectation) error

// FeatureItem represents a single configurable feature
type FeatureItem struct {
	Key         string
	Label       string
	Apply       FeatureFunc
	Description string
}

// Category represents a group of related features
type Category struct {
	Key      string
	Label    string
	Features []FeatureItem
}

// Registry creates the complete feature catalog with all available features
func Registry() []Category {
	return []Category{
		{
			Key:   "response-behavior",
			Label: "Response Behavior",
			Features: []FeatureItem{
				{
					Key:         "delays",
					Label:       "Response Delays",
					Apply:       applyDelays(),
					Description: "Add fixed, random, or progressive delays",
				},
				{
					Key:         "limits",
					Label:       "Response Limits",
					Apply:       applyLimits(),
					Description: "Limit how many times expectation matches",
				},
				{
					Key:         "priority",
					Label:       "Expectation Priority",
					Apply:       applyPriority(),
					Description: "Set priority for conflicting expectations",
				},
				{
					Key:         "content-length-headers",
					Label:       "Control Content Length Header",
					Apply:       ControlContentLengthHeaders(),
					Description: "Manually set or remove Content-Length header",
				},
				{
					Key:         "caching",
					Label:       "Cache Control",
					Apply:       applyCaching(),
					Description: "Configure cache headers and ETags",
				},
				{
					Key:         "compression",
					Label:       "Response Compression",
					Apply:       applyCompression(),
					Description: "Enable gzip/deflate compression",
				},
			},
		},
		{
			Key:   "connection",
			Label: "Connection Control",
			Features: []FeatureItem{
				{
					Key:         "suppress-connection-header",
					Label:       "Suppress Connection Header",
					Apply:       suppressConnectionHeader(),
					Description: "Suppress the Connection header in responses",
				},
				{
					Key:         "chunked-encoding",
					Label:       "Chunked Encoding, Specify Chunk Size",
					Apply:       applyChunked(),
					Description: "Control chunked transfer encoding",
				},
				{
					Key:         "keep-alive",
					Label:       "Override Keep-Alive Settings",
					Apply:       applyKeepAlive(),
					Description: "Connection persistence patterns",
				},
				{
					Key:         "close-socket",
					Label:       "Close Socket",
					Apply:       closeSocket(),
					Description: "Forcefully close the connection after response",
				},
			},
		},
	}
}

// PickFeaturesInteractively allows users to select features through an interactive menu
func PickFeaturesInteractively(reg []Category) ([]FeatureItem, error) {
	fmt.Println("\n🎨 Advanced Features Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💡 Select categories and features to configure advanced MockServer behavior")
	fmt.Println()

	// Step 1: Show available categories
	var catLabels []string
	labelToCat := make(map[string]Category)

	for _, c := range reg {
		catLabels = append(catLabels, c.Label)
		labelToCat[c.Label] = c
	}

	var chosenCatLabels []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Select feature categories:",
		Options: catLabels,
		Help:    "Use SPACE to select, ENTER to confirm. Choose categories that interest you.",
	}, &chosenCatLabels); err != nil {
		return nil, err
	}

	if len(chosenCatLabels) == 0 {
		fmt.Println("ℹ️  No categories selected, skipping advanced features")
		return nil, nil
	}

	// Step 2: For each selected category, pick specific features
	var allSelectedFeatures []FeatureItem

	for _, catLabel := range chosenCatLabels {
		cat := labelToCat[catLabel]

		fmt.Printf("\n📂 Category: %s\n", cat.Label)

		var featOptions []string
		labelToFeat := make(map[string]FeatureItem)

		for _, feat := range cat.Features {
			option := fmt.Sprintf("%s — %s", feat.Label, feat.Description)
			featOptions = append(featOptions, option)
			labelToFeat[option] = feat
		}

		var chosenFeatLabels []string
		if err := survey.AskOne(&survey.MultiSelect{
			Message: fmt.Sprintf("Select features from '%s':", cat.Label),
			Options: featOptions,
			Help:    "Use SPACE to select multiple, ENTER to confirm",
		}, &chosenFeatLabels); err != nil {
			return nil, err
		}

		for _, featLabel := range chosenFeatLabels {
			allSelectedFeatures = append(allSelectedFeatures, labelToFeat[featLabel])
		}
	}

	if len(allSelectedFeatures) == 0 {
		fmt.Println("ℹ️  No features selected")
		return nil, nil
	}

	fmt.Printf("\n✅ Selected %d feature(s) to configure\n", len(allSelectedFeatures))
	return allSelectedFeatures, nil
}

// ApplySelectedFeatures applies all selected features to an expectation
func ApplySelectedFeatures(exp *MockExpectation, features []FeatureItem) error {
	if len(features) == 0 {
		return nil
	}

	fmt.Printf("\n🔧 Applying %d Advanced Feature(s)\n", len(features))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	successCount := 0
	failureCount := 0

	for i, feat := range features {
		fmt.Printf("\n[%d/%d] Configuring: %s\n", i+1, len(features), feat.Label)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		if err := feat.Apply(exp); err != nil {
			failureCount++
			fmt.Printf("⚠️  Warning: Failed to configure %s: %v\n", feat.Label, err)

			// Ask if they want to continue
			var continueAnyway bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Continue with remaining features?",
				Default: true,
			}, &continueAnyway); err != nil || !continueAnyway {
				return fmt.Errorf("feature configuration cancelled after %d successes, %d failures", successCount, failureCount)
			}
		} else {
			successCount++
			fmt.Printf("✅ Successfully configured %s\n", feat.Label)
		}
	}

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📊 Feature Configuration Summary:\n")
	fmt.Printf("   ✅ Successful: %d\n", successCount)
	if failureCount > 0 {
		fmt.Printf("   ⚠️  Failed: %d\n", failureCount)
	}
	fmt.Println()

	return nil
}

// CollectAdvancedFeaturesInteractive is the main entry point for feature selection and application
func CollectAdvancedFeaturesInteractive(mc *MockConfigurator, exp *MockExpectation) error {
	var wantsAdvanced bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Configure advanced MockServer features?",
		Default: false,
		Help:    "Delays, callbacks, connection control, testing patterns, and more",
	}, &wantsAdvanced); err != nil {
		return err
	}

	if !wantsAdvanced {
		fmt.Println("ℹ️  Skipping advanced features")
		return nil
	}

	// Get the registry
	registry := Registry()

	// Let user pick features interactively
	selectedFeatures, err := PickFeaturesInteractively(registry)
	if err != nil {
		return fmt.Errorf("failed to select features: %w", err)
	}

	if len(selectedFeatures) == 0 {
		return nil
	}

	// Apply all selected features
	return ApplySelectedFeatures(exp, selectedFeatures)
}
