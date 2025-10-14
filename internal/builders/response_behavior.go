package builders

import "fmt"

func applyPriority(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Priority feature...")
		return nil
	}
}

func applyLimits(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Limits feature...")
		return nil
	}
}

func applyDelays(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		return collectResponseDelay(exp) // your existing dialog
	}
}

func applyResponseHeaders(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Response Headers feature...")
		return nil
	}
}

func applyCaching(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Caching feature...")
		return nil
	}
}

func applyCompression(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Compression feature...")
		return nil
	}
}
