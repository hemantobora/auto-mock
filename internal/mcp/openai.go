package mcp

import (
	"context"
	"os"
)

type openaiProvider struct{}

func (o openaiProvider) Name() string     { return "openai" }
func (o openaiProvider) CostHint() string { return "$$" }
func (o openaiProvider) Available() bool  { return os.Getenv("OPENAI_API_KEY") != "" }
func init()                               { register(openaiProvider{}) }

func (o openaiProvider) Generate(ctx context.Context, in GenerateInput) (Result, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return Result{}, ErrMissingKey("OPENAI_API_KEY")
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5-mini"
	}

	payload := map[string]any{
		"model":       model,
		"temperature": 0,
		"input":       buildOpenAIUserPrompt(in),
	}

	var raw struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	err := doJSON(ctx, "POST", "https://api.openai.com/v1/responses",
		map[string]string{
			"Authorization": "Bearer " + key,
		}, payload, &raw)
	if err != nil {
		return Result{}, err
	}

	text := ""
	if len(raw.Output) > 0 && len(raw.Output[0].Content) > 0 {
		text = raw.Output[0].Content[0].Text
	}

	return Result{
		Provider:       o.Name(),
		MockServerJSON: trimFences(text),
		TokensUsed:     raw.Usage.TotalTokens,
	}, nil
}

// buildOpenAIUserPrompt same idea.
func buildOpenAIUserPrompt(in GenerateInput) string {
	return in.Prompt
}
