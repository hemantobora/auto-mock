package builders

import "fmt"

func applyCircuitBreaker(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Circuit Breaker feature...")
		return nil
	}
}

func applyRateLimiting(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Rate Limiting feature...")
		return nil
	}
}

func applyChaos(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Chaos Engineering feature...")
		return nil
	}
}

func applyLoad(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Load Testing feature...")
		return nil
	}
}

func applyResilience(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Resilience Testing feature...")
		return nil
	}
}

func applySecurity(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Security Testing feature...")
		return nil
	}
}
