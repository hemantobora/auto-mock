# Feature Registry Pattern - Improvements

## ✅ What's Great

1. **Registry Pattern**: Centralized feature catalog
2. **Category Organization**: Logical grouping (Response Behavior, Dynamic Content, etc.)
3. **Interactive Selection**: Two-level picker (category → features)
4. **Function References**: Using `FeatureFunc` type for consistency

## 🔧 Suggested Improvements

### 1. Fix Function Factory Pattern

**Current Problem:**
```go
Features: []FeatureItem{
    {"delays", "Delays", applyDelays(mc), "Add delays"},
    //                   ^^^^^^^^^^^^^^^^
    //                   This EXECUTES immediately, not stored as reference
}
```

**Solution A: Return Function Factory**
```go
func applyDelays(mc *MockConfigurator) FeatureFunc {
    return func(exp *MockExpectation) error {
        return mc.CollectDelayConfiguration(exp)
    }
}

// Usage in Registry
Features: []FeatureItem{
    {"delays", "Delays", applyDelays(mc), "Add delays"},
    //                   ^^^^^^^^^^^^^^^^
    //                   Now returns a function to be called later
}
```

**Solution B: Inline Functions (More Explicit)**
```go
Features: []FeatureItem{
    {
        Key:   "delays",
        Label: "Delays (fixed / random / progressive)",
        Apply: func(exp *MockExpectation) error {
            return mc.CollectDelayConfiguration(exp)
        },
        Description: "Add response delays",
    },
}
```

### 2. Complete Implementation Example

```go
package builders

import (
    "fmt"
    "github.com/AlecAivazis/survey/v2"
)

type FeatureFunc func(exp *MockExpectation) error

type FeatureItem struct {
    Key         string
    Label       string
    Apply       FeatureFunc
    Description string
}

type Category struct {
    Key      string
    Label    string
    Features []FeatureItem
}

// Registry creates the feature catalog
func Registry(mc *MockConfigurator) []Category {
    return []Category{
        {
            Key:   "response-behavior",
            Label: "Response Behavior",
            Features: []FeatureItem{
                {
                    Key:   "delays",
                    Label: "Delays (fixed / random / progressive)",
                    Apply: func(exp *MockExpectation) error {
                        return mc.CollectDelayConfiguration(exp)
                    },
                    Description: "Add response delays with various patterns",
                },
                {
                    Key:   "times",
                    Label: "Times (limit response count)",
                    Apply: func(exp *MockExpectation) error {
                        return mc.CollectTimesConfiguration(exp)
                    },
                    Description: "Control how many times expectation matches",
                },
                {
                    Key:   "priority",
                    Label: "Priority (conflict resolution)",
                    Apply: func(exp *MockExpectation) error {
                        return mc.CollectPriorityConfiguration(exp)
                    },
                    Description: "Set expectation matching priority",
                },
            },
        },
        {
            Key:   "dynamic-content",
            Label: "Dynamic Content",
            Features: []FeatureItem{
                {
                    Key:   "templating",
                    Label: "Response Templating",
                    Apply: func(exp *MockExpectation) error {
                        return mc.CollectTemplatingConfiguration(exp)
                    },
                    Description: "Use ${variables} in response body",
                },
                {
                    Key:   "sequences",
                    Label: "Response Sequences",
                    Apply: func(exp *MockExpectation) error {
                        return mc.CollectSequenceConfiguration(exp)
                    },
                    Description: "Different responses over time",
                },
            },
        },
        {
            Key:   "connection",
            Label: "Connection Control",
            Features: []FeatureItem{
                {
                    Key:   "drop-connection",
                    Label: "Drop Connection",
                    Apply: func(exp *MockExpectation) error {
                        return mc.CollectConnectionDropConfiguration(exp)
                    },
                    Description: "Simulate network failures",
                },
                {
                    Key:   "chunked-encoding",
                    Label: "Chunked Transfer Encoding",
                    Apply: func(exp *MockExpectation) error {
                        return mc.CollectChunkedEncodingConfiguration(exp)
                    },
                    Description: "Control transfer encoding",
                },
            },
        },
    }
}

// PickFeaturesInteractively allows users to select features interactively
func PickFeaturesInteractively(reg []Category) ([]FeatureItem, error) {
    // Step 1: Pick categories
    var catLabels []string
    labelToCat := make(map[string]Category)
    
    for _, c := range reg {
        catLabels = append(catLabels, c.Label)
        labelToCat[c.Label] = c
    }
    
    var chosenCatLabels []string
    if err := survey.AskOne(&survey.MultiSelect{
        Message: "Select feature categories to configure:",
        Options: catLabels,
        Help:    "Use SPACE to select, ENTER to confirm",
    }, &chosenCatLabels); err != nil {
        return nil, err
    }
    
    if len(chosenCatLabels) == 0 {
        return nil, nil // No categories selected
    }
    
    // Step 2: For each category, pick specific features
    var allSelectedFeatures []FeatureItem
    
    for _, catLabel := range chosenCatLabels {
        cat := labelToCat[catLabel]
        
        var featOptions []string
        labelToFeat := make(map[string]FeatureItem)
        
        for _, feat := range cat.Features {
            option := fmt.Sprintf("%s - %s", feat.Label, feat.Description)
            featOptions = append(featOptions, option)
            labelToFeat[option] = feat
        }
        
        var chosenFeatLabels []string
        if err := survey.AskOne(&survey.MultiSelect{
            Message: fmt.Sprintf("Select features from '%s':", cat.Label),
            Options: featOptions,
            Help:    "Use SPACE to select, ENTER to confirm",
        }, &chosenFeatLabels); err != nil {
            return nil, err
        }
        
        for _, featLabel := range chosenFeatLabels {
            allSelectedFeatures = append(allSelectedFeatures, labelToFeat[featLabel])
        }
    }
    
    return allSelectedFeatures, nil
}

// ApplySelectedFeatures applies all selected features to an expectation
func ApplySelectedFeatures(exp *MockExpectation, features []FeatureItem) error {
    fmt.Printf("\n🎨 Applying %d advanced features...\n", len(features))
    
    for i, feat := range features {
        fmt.Printf("\n[%d/%d] Configuring: %s\n", i+1, len(features), feat.Label)
        fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
        
        if err := feat.Apply(exp); err != nil {
            fmt.Printf("⚠️  Warning: Failed to configure %s: %v\n", feat.Label, err)
            // Ask if they want to continue
            var continueAnyway bool
            if err := survey.AskOne(&survey.Confirm{
                Message: "Continue with remaining features?",
                Default: true,
            }, &continueAnyway); err != nil || !continueAnyway {
                return fmt.Errorf("feature configuration cancelled")
            }
        } else {
            fmt.Printf("✅ Successfully configured %s\n", feat.Label)
        }
    }
    
    return nil
}
```

### 3. Usage in REST Builder

```go
// In internal/builders/rest.go

func (mc *MockConfigurator) CollectAdvancedFeatures(expectation *MockExpectation) error {
    fmt.Println("\n🎨 Step 6: Advanced Features")
    fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    
    var wantsAdvanced bool
    if err := survey.AskOne(&survey.Confirm{
        Message: "Configure advanced MockServer features?",
        Default: false,
        Help:    "Delays, callbacks, connection control, testing patterns, etc.",
    }, &wantsAdvanced); err != nil {
        return err
    }
    
    if !wantsAdvanced {
        return nil
    }
    
    // Get registry
    registry := Registry(mc)
    
    // Let user pick features interactively
    selectedFeatures, err := PickFeaturesInteractively(registry)
    if err != nil {
        return fmt.Errorf("failed to select features: %w", err)
    }
    
    if len(selectedFeatures) == 0 {
        fmt.Println("ℹ️  No advanced features selected")
        return nil
    }
    
    // Apply all selected features
    return ApplySelectedFeatures(expectation, selectedFeatures)
}
```

### 4. Add Feature Dependencies

```go
type FeatureItem struct {
    Key          string
    Label        string
    Apply        FeatureFunc
    Description  string
    Dependencies []string      // ← Feature keys this depends on
    ConflictsWith []string     // ← Features that conflict with this one
}

// Check dependencies before applying
func ValidateFeatureSelection(features []FeatureItem) error {
    featureKeys := make(map[string]bool)
    for _, f := range features {
        featureKeys[f.Key] = true
    }
    
    for _, f := range features {
        // Check dependencies
        for _, dep := range f.Dependencies {
            if !featureKeys[dep] {
                return fmt.Errorf("feature '%s' requires '%s' to be enabled", f.Key, dep)
            }
        }
        
        // Check conflicts
        for _, conflict := range f.ConflictsWith {
            if featureKeys[conflict] {
                return fmt.Errorf("feature '%s' conflicts with '%s'", f.Key, conflict)
            }
        }
    }
    
    return nil
}
```

### 5. Add Feature Metadata

```go
type FeatureItem struct {
    Key         string
    Label       string
    Apply       FeatureFunc
    Description string
    Examples    []string      // ← Show examples
    Difficulty  string        // ← "basic", "intermediate", "advanced"
    DocURL      string        // ← Link to documentation
}
```

## 🎯 Recommended Next Steps

1. **Implement all `applyXXX` functions** or use inline functions
2. **Test the interactive flow** - does the two-level picker work well?
3. **Consider feature groups** - some features naturally go together
4. **Add validation** - check for conflicts/dependencies
5. **Provide examples** - show what each feature does
6. **Document integration** - how does this replace the old approach?

## 📊 Comparison: Old vs New

### Old Approach (rest.go)
```go
// Monolithic, hard to maintain
func (mc *MockConfigurator) CollectAdvancedFeatures(exp *MockExpectation) error {
    // 500+ lines of nested if/else
    // Hard to add new features
    // Difficult to test individual features
}
```

### New Approach (mocking_features.go)
```go
// Modular, easy to extend
registry := Registry(mc)
features, _ := PickFeaturesInteractively(registry)
ApplySelectedFeatures(exp, features)

// Benefits:
// ✅ Easy to add new features (just add to registry)
// ✅ Easy to test (test individual feature functions)
// ✅ Easy to maintain (features are isolated)
// ✅ Easy to discover (clear structure)
```

## 🚀 This is the RIGHT direction!

Your approach is **significantly better** than the current implementation. Once you:
1. Fix the function factory pattern
2. Implement the feature functions
3. Integrate with the builders

This will be **much more maintainable** and **extensible**!
