package builders

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
)

// applyDropConnection returns a FeatureFunc for drop connection configuration
func applyDropConnection(mc *MockConfigurator) FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("\n🔌 Drop Connection Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		var dropConnection bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Drop connection (simulate network failure)?",
			Default: false,
			Help:    "Close connection immediately without sending response",
		}, &dropConnection); err != nil {
			return err
		}

		if dropConnection {
			if exp.ConnectionOptions == nil {
				exp.ConnectionOptions = &ConnectionOptions{}
			}
			exp.ConnectionOptions.DropConnection = true
			exp.HttpResponse.Body = nil
			fmt.Println("✅ Drop connection enabled - will simulate network failure")
		} else {
			fmt.Println("ℹ️  Connection drop not enabled")
		}

		return nil
	}
}

// applyChunked returns a FeatureFunc for chunked encoding configuration
func applyChunked(mc *MockConfigurator) FeatureFunc {
	return func(expectation *MockExpectation) error {
		fmt.Println("\n📦 Chunked Encoding Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		var useChunked bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Enable chunked transfer encoding?",
			Default: false,
			Help:    "Send response in chunks (Transfer-Encoding: chunked)",
		}, &useChunked); err != nil {
			return err
		}

		if useChunked {
			ensureMaps(expectation)
			if expectation.ConnectionOptions == nil {
				expectation.ConnectionOptions = &ConnectionOptions{}
			}
			expectation.ConnectionOptions.ChunkedEncoding = true
			delete(expectation.HttpResponse.Headers, "Content-Length")
			expectation.HttpResponse.Headers["Transfer-Encoding"] = []string{"chunked"}
			fmt.Println("✅ Chunked encoding enabled")
		} else {
			fmt.Println("ℹ️  Chunked encoding not enabled")
		}

		return nil
	}
}

// applyKeepAlive returns a FeatureFunc for keep-alive configuration
func applyKeepAlive(mc *MockConfigurator) FeatureFunc {
	return func(expectation *MockExpectation) error {
		fmt.Println("\n🔄 Keep-Alive Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		var useKeepAlive bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Enable connection keep-alive?",
			Default: true,
			Help:    "Reuse HTTP connection for multiple requests",
		}, &useKeepAlive); err != nil {
			return err
		}

		if useKeepAlive {
			ensureMaps(expectation)
			if expectation.ConnectionOptions == nil {
				expectation.ConnectionOptions = &ConnectionOptions{}
			}
			expectation.ConnectionOptions.KeepAlive = true
			expectation.ConnectionOptions.CloseSocket = false
			expectation.HttpResponse.Headers["Connection"] = []string{"keep-alive"}
			fmt.Println("✅ Keep-alive enabled")
		} else {
			fmt.Println("ℹ️  Keep-alive disabled - connection will close after response")
		}

		return nil
	}
}
