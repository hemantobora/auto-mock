package builders

import (
	"fmt"
)

func applyTemplating(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Templating feature...")
		return nil
	}
}

func applySequences(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Sequences feature...")
		return nil
	}
}

func applyConditions(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Conditions feature...")
		return nil
	}
}

func applyStateMachine(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring State Machine feature...")
		return nil
	}
}

func applyDataGen(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Data Generation feature...")
		return nil
	}
}

func applyInterpolation(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("⚙️  Configuring Advanced String Interpolation feature...")
		return nil
	}
}
