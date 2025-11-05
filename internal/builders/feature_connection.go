package builders

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

// suppressConnection returns a FeatureFunc for drop connection configuration
func suppressConnectionHeader() FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("\n🔌 Suppress Connection Header Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		if exp.HttpResponse.ConnectionOptions == nil {
			exp.HttpResponse.ConnectionOptions = &ConnectionOptions{}
		}
		exp.HttpResponse.ConnectionOptions.SuppressConnectionHeader = true
		fmt.Println("✅ Suppress connection header enabled")

		return nil
	}
}

// applyChunked returns a FeatureFunc for chunked encoding configuration
func applyChunked() FeatureFunc {
	return func(expectation *MockExpectation) error {
		fmt.Println("\n📦 Chunked Encoding Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		var useChunked string
		if err := survey.AskOne(&survey.Input{
			Message: "Enable chunked transfer encoding? With chunk size in bytes (e.g., 50 for 50 bytes):",
			Default: "50",
			Help:    "Send response in chunks (Transfer-Encoding: chunked)",
		}, &useChunked); err != nil {
			return err
		}

		val, err := strconv.Atoi(strings.TrimSpace(useChunked))
		if err != nil || val < 0 {
			fmt.Printf("invalid chunk size: %q, Chunked encoding not enabled\n", useChunked)
			return nil
		}
		if val > 0 {
			ensureNameValues(expectation)
			if expectation.HttpResponse.ConnectionOptions == nil {
				expectation.HttpResponse.ConnectionOptions = &ConnectionOptions{}
			}
			expectation.HttpResponse.ConnectionOptions.ChunkSize = val

			// update headers ([]NameValues)
			deleteHeader(&expectation.HttpResponse.Headers, "Content-Length")
			deleteHeader(&expectation.HttpResponse.Headers, "Transfer-Encoding")
			fmt.Println("✅ Chunked encoding enabled")
		} else {
			fmt.Println("ℹ️  Chunked encoding not enabled")
		}
		return nil
	}
}

// applyKeepAlive returns a FeatureFunc for keep-alive configuration
func applyKeepAlive() FeatureFunc {
	return func(expectation *MockExpectation) error {
		fmt.Println("\n🔄 Keep-Alive Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		var useKeepAlive bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Override connection keep-alive?",
			Default: true,
			Help:    "Reuse HTTP connection for multiple requests",
		}, &useKeepAlive); err != nil {
			return err
		}

		if useKeepAlive {
			ensureNameValues(expectation)
			if expectation.HttpResponse.ConnectionOptions == nil {
				expectation.HttpResponse.ConnectionOptions = &ConnectionOptions{}
			}
			expectation.HttpResponse.ConnectionOptions.KeepAliveOverride = true
			expectation.HttpResponse.ConnectionOptions.CloseSocket = false
			// update headers ([]NameValues)
			deleteHeader(&expectation.HttpResponse.Headers, "Connection")
			fmt.Println("✅ Keep-alive enabled")
		} else {
			// optional: you could explicitly set "Connection: close" here if desired
			fmt.Println("ℹ️  Keep-alive disabled - connection will close after response")
		}
		return nil
	}
}

func closeSocket() FeatureFunc {
	return func(expectation *MockExpectation) error {
		fmt.Println("\n❌ Close Socket Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		var shouldClose bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Close socket after response?",
			Default: false,
			Help:    "Forcefully close the connection after sending response",
		}, &shouldClose); err != nil {
			return err
		}

		if shouldClose {
			var shouldDelay bool
			if err := survey.AskOne(&survey.Confirm{
				Message: "Would you like to delay closing the socket?",
				Default: false,
				Help:    "Introduce a delay before forcefully closing the connection",
			}, &shouldDelay); err != nil {
				return err
			}

			if expectation.HttpResponse.ConnectionOptions == nil {
				expectation.HttpResponse.ConnectionOptions = &ConnectionOptions{}
			}
			expectation.HttpResponse.ConnectionOptions.CloseSocket = true
			if shouldDelay {
				var fixedStr string
				if err := survey.AskOne(&survey.Input{
					Message: "Delay in milliseconds (e.g., 500):",
					Default: "500",
				}, &fixedStr, survey.WithValidator(survey.Required)); err != nil {
					return err
				}
				val, err := strconv.Atoi(strings.TrimSpace(fixedStr))
				if err != nil || val < 0 {
					fmt.Printf("invalid delay: %q, Socket would be closed immediately\n", fixedStr)
					return nil
				}
				expectation.HttpResponse.ConnectionOptions.CloseSocketDelay = &Delay{TimeUnit: "MILLISECONDS", Value: val}
			}
			fmt.Println("✅ Socket will be closed after response")
		} else {
			fmt.Println("ℹ️  Socket will remain open after response")
		}

		return nil
	}
}
