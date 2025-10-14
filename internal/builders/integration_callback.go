package builders

import (
	"fmt"
)

func applyWebhooks(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Webhooks feature...")
		return nil
	}
}

func applyCustomCode(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Custom Code feature...")
		return nil
	}
}

func applyForward(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Forwarding feature...")
		return nil
	}
}

func applyProxy(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Proxy feature...")
		return nil
	}
}

func applyTransformation(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Transformation feature...")
		return nil
	}
}

func applyEventStreaming(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Event Streaming feature...")
		return nil
	}
}
