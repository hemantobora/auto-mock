package mcp

import (
	"context"
)

// What your CLI expects back
type Result struct {
	Provider       string
	MockServerJSON string
	TokensUsed     int
	GenerationTime string // e.g., "3.2s"
	Warnings       []string
	Suggestions    []string
}

type ProviderInfo struct {
	Name      string
	Available bool
	Cost      string // optional display label, e.g. "$$"
}

// Each provider is isolated to its file and implements this.
type Provider interface {
	Name() string
	Available() bool
	CostHint() string
	Generate(ctx context.Context, in GenerateInput) (Result, error)
}

// Input shape passed to providers.
type GenerateInput struct {
	ProjectName string
	Prompt      string   // already shaped by your caller (REST/GraphQL hints etc.)
	StyleHints  []string // optional extra hints
}
