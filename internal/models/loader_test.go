package models

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestLoader_StartStop_NoPanic(t *testing.T) {
	var buf bytes.Buffer
	l := NewLoader(&buf, "Testing...", WithANSI(true), WithInterval(10*time.Millisecond))
	l.Start()
	time.Sleep(35 * time.Millisecond)
	l.SetMessage("Almost there")
	time.Sleep(25 * time.Millisecond)
	l.Stop()

	out := buf.String()
	if out == "" {
		t.Fatal("expected output, got empty string")
	}
	if !strings.Contains(out, "\x1b[2K") { // erased line at least once
		t.Error("expected ANSI clear line sequence in output")
	}
}
