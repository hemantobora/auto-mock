package models

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Loader is a lightweight, visually appealing CLI spinner/loader that works on POSIX and Windows.
//
// Features:
// - Cross-platform: uses ANSI when available, falls back to plain-carriage spinners otherwise
// - Pluggable: provide custom frames, interval, colors, output writer
// - Safe: Start/Stop idempotent; uses a goroutine with cooperative shutdown
//
// Typical usage:
//
//	l := models.NewLoader(os.Stdout, "Deploying infrastructure...")
//	l.Start()
//	// do work
//	l.SetMessage("Finalizing")
//	l.StopWithMessage("Done ✔")
type Loader struct {
	mu           sync.Mutex
	msg          string
	frames       []string
	interval     time.Duration
	out          io.Writer
	stopCh       chan struct{}
	doneCh       chan struct{}
	active       bool
	supportsANSI bool
	color        string // ANSI color code, e.g., "36" for cyan
	hideCursor   bool
}

// Option configures the loader.
type Option func(*Loader)

// WithFrames sets custom spinner frames.
func WithFrames(frames []string) Option {
	return func(l *Loader) { l.frames = append([]string(nil), frames...) }
}

// WithInterval sets frame interval.
func WithInterval(d time.Duration) Option { return func(l *Loader) { l.interval = d } }

// WithANSI forces ANSI on/off (useful for tests or specific terminals).
func WithANSI(enabled bool) Option { return func(l *Loader) { l.supportsANSI = enabled } }

// WithColor sets ANSI color (e.g., "36" for cyan). Only used if ANSI is enabled.
func WithColor(ansiColorCode string) Option { return func(l *Loader) { l.color = ansiColorCode } }

// WithWriter overrides output writer (defaults to os.Stdout).
func WithWriter(w io.Writer) Option { return func(l *Loader) { l.out = w } }

// WithoutCursor disables cursor hiding/showing even if ANSI is supported.
func WithoutCursor() Option { return func(l *Loader) { l.hideCursor = false } }

// NewLoader creates a loader with sensible defaults.
func NewLoader(out io.Writer, message string, opts ...Option) *Loader {
	// Fancy braille spinner (works great on POSIX terminals); fallback set added for non-ANSI.
	defaultFrames := []string{"⠋", "⠙", "⠚", "⠞", "⠖", "⠦", "⠴", "⠲", "⠳", "⠓"}
	l := &Loader{
		msg:          message,
		frames:       defaultFrames,
		interval:     90 * time.Millisecond,
		out:          out,
		supportsANSI: supportsANSIByDefault(),
		color:        "36", // cyan
		hideCursor:   true,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
	if l.out == nil {
		l.out = os.Stdout
	}
	for _, opt := range opts {
		opt(l)
	}
	// If ANSI is not supported, use basic ASCII frames that render cleanly.
	if !l.supportsANSI {
		l.frames = []string{"-", "\\", "|", "/"}
	}
	return l
}

func supportsANSIByDefault() bool {
	// macOS/Linux: true by default
	if runtime.GOOS != "windows" {
		return true
	}
	// On Windows, modern terminals often support ANSI, but to be safe default to false here.
	// Caller may override with WithANSI(true) if desired.
	return false
}

// Start begins the spinner. Safe to call only once; repeated calls are ignored.
func (l *Loader) Start() {
	l.mu.Lock()
	if l.active {
		l.mu.Unlock()
		return
	}
	l.active = true
	stopCh := l.stopCh
	doneCh := l.doneCh
	msg := l.msg
	supports := l.supportsANSI
	hideCursor := l.hideCursor && supports
	out := l.out
	interval := l.interval
	frames := append([]string(nil), l.frames...)
	color := l.color
	l.mu.Unlock()

	// Hide cursor
	if hideCursor {
		fmt.Fprint(out, "\x1b[?25l")
	}

	go func() {
		defer close(doneCh)
		i := 0
		for {
			select {
			case <-stopCh:
				// Clear line and restore cursor
				if supports {
					fmt.Fprint(out, "\r\x1b[2K")
					if hideCursor {
						fmt.Fprint(out, "\x1b[?25h")
					}
				} else {
					// Carriage return and simple spaces clear
					fmt.Fprint(out, "\r"+spaces(len(stripANSI(msg))+4)+"\r")
				}
				return
			default:
				frame := frames[i%len(frames)]
				i++
				l.mu.Lock()
				msg = l.msg
				l.mu.Unlock()
				if supports {
					// Erase line, print colored spinner + message
					// e.g., "\r\x1b[2K\x1b[36m⠋\x1b[0m Deploying..."
					fmt.Fprintf(out, "\r\x1b[2K\x1b[%sm%s\x1b[0m %s", color, frame, msg)
				} else {
					// Basic carriage return update
					fmt.Fprintf(out, "\r%s %s", frame, msg)
				}
				time.Sleep(interval)
			}
		}
	}()
}

// Stop stops the spinner and prints a newline.
func (l *Loader) Stop() {
	l.mu.Lock()
	if !l.active {
		l.mu.Unlock()
		return
	}
	l.active = false
	close(l.stopCh)
	done := l.doneCh
	out := l.out
	l.mu.Unlock()
	<-done
	fmt.Fprint(out, "\n")
}

// StopWithMessage stops the spinner and prints a final message on a new line.
func (l *Loader) StopWithMessage(finalMsg string) {
	l.Stop()
	if strings.TrimSpace(finalMsg) != "" {
		fmt.Fprintln(l.out, finalMsg)
	}
}

// SetMessage updates the message displayed after the spinner.
func (l *Loader) SetMessage(m string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msg = m
}

// Active returns whether the loader is currently running.
func (l *Loader) Active() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

// stripANSI removes ANSI escape sequences for width calculations (minimal variant).
func stripANSI(s string) string {
	// Very small/naive remover for CSI sequences; good enough for our needs.
	b := make([]rune, 0, len(s))
	skip := false
	for i := 0; i < len(s); i++ {
		if !skip && i+1 < len(s) && s[i] == 0x1b && s[i+1] == '[' {
			skip = true
			i++
			continue
		}
		if skip {
			// End on letter (A-Za-z)
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				skip = false
			}
			continue
		}
		b = append(b, rune(s[i]))
	}
	return string(b)
}
