package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"
)

var (
	regMu     sync.RWMutex
	providers = map[string]Provider{} // name -> provider
)

// Called by each provider in its init()
func register(p Provider) {
	regMu.Lock()
	providers[p.Name()] = p
	regMu.Unlock()
}

// Used by your CLI to show available providers
func ListProviders() []ProviderInfo {
	regMu.RLock()
	defer regMu.RUnlock()

	out := make([]ProviderInfo, 0, len(providers))
	for _, p := range providers {
		out = append(out, ProviderInfo{
			Name:      p.Name(),
			Available: p.Available(),
			Cost:      p.CostHint(),
		})
	}
	return out
}

// Single public entry your CLI uses
func GenerateWithProvider(ctx context.Context, prompt, providerName, projectName string) (Result, error) {
	regMu.RLock()
	p, ok := providers[providerName]
	regMu.RUnlock()
	if !ok {
		return Result{}, fmt.Errorf("unknown provider: %s", providerName)
	}

	start := time.Now()
	fmt.Print("🤖 Generating with AI")
	done := make(chan struct{})

	// background ticker that prints dots every 2 s
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Print(".")
			}
		}
	}()

	res, err := p.Generate(ctx, GenerateInput{
		ProjectName: projectName,
		Prompt:      prompt,
	})
	close(done)
	if err != nil {
		return Result{}, err
	}
	if res.Provider == "" {
		res.Provider = providerName
	}
	if res.GenerationTime == "" {
		res.GenerationTime = time.Since(start).Round(100 * time.Millisecond).String()
	}
	return res, nil
}
