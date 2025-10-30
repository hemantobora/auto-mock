package mcp

import (
	"context"
	"os"
	"strings"
)

type anthropicProvider struct{}

func (a anthropicProvider) Name() string     { return "anthropic" }
func (a anthropicProvider) CostHint() string { return "$$" }
func (a anthropicProvider) Available() bool  { return os.Getenv("ANTHROPIC_API_KEY") != "" }
func init()                                  { register(anthropicProvider{}) }

func (a anthropicProvider) Generate(ctx context.Context, in GenerateInput) (Result, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return Result{}, ErrMissingKey("ANTHROPIC_API_KEY")
	}

	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-sonnet-4-5"
	}

	payload := map[string]any{
		"model":       model,
		"max_tokens":  4096,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "user", "content": buildAnthropicUserPrompt(in)},
		},
	}

	var raw struct {
		Content []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	err := doJSON(ctx, "POST", "https://api.anthropic.com/v1/messages",
		map[string]string{
			"x-api-key":         key,
			"anthropic-version": "2023-06-01",
		}, payload, &raw)
	if err != nil {
		return Result{}, err
	}

	var builder strings.Builder
	for _, c := range raw.Content {
		if c.Type == "text" {
			builder.WriteString(c.Text)
		}
	}

	return Result{
		Provider:       a.Name(),
		MockServerJSON: trimFences(builder.String()),
		TokensUsed:     raw.Usage.InputTokens + raw.Usage.OutputTokens,
	}, nil
}

// buildAnthropicUserPrompt is just a thin passthrough.
func buildAnthropicUserPrompt(in GenerateInput) string {
	return in.Prompt
}
