package builders

import "fmt"

func applyStateful(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Statefulness feature...")
		return nil
	}
}

func applyWorkflow(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Workflow feature...")
		return nil
	}
}

func applyEventDriven(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Event-Driven feature...")
		return nil
	}
}

func applyMicroservices(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Microservices feature...")
		return nil
	}
}

func applyAPIVersioning(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring API Versioning feature...")
		return nil
	}
}

func applyTenantIsolation(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Tenant Isolation feature...")
		return nil
	}
}

// ---- 1) Registry of available features ----
