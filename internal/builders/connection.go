package builders

import "fmt"

func applyDropConnection(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Drop Connection feature...")
		return nil
	}
}

func applyChunked(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Chunked Responses feature...")
		return nil
	}
}

func applyKeepAlive(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Keep Alive feature...")
		return nil
	}
}

func applyErrors(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Error Simulation feature...")
		return nil
	}
}

func applyBandwidth(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Bandwidth Throttling feature...")
		return nil
	}
}

func applySSL(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring SSL/TLS Behavior feature...")
		return nil
	}
}
