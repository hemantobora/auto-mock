package azure

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

// showSpinner prints a braille dot-style spinner in the terminal until the
// caller closes the done channel. Run the blocking work in a goroutine and
// close done when it finishes; showSpinner blocks the caller until then.
//
//	done := make(chan struct{})
//	go func() { defer close(done); doWork() }()
//	showSpinner("Creating resource group 'auto-mock-rg'", done)
func showSpinner(action string, done <-chan struct{}) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			fmt.Printf("\r\033[K") // clear the spinner line
			return
		case <-ticker.C:
			fmt.Printf("\r%s %s...", frames[i%len(frames)], action)
			i++
		}
	}
}

// computeFileHash computes a SHA-256 digest of the file at filePath.
// Returns the digest as "sha256:<hex>", the byte count, and any error.
// Mirrors the equivalent function in the AWS package so load-test bundle
// upload logic stays consistent across providers.
func computeFileHash(filePath string) (string, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), n, nil
}
