package builders

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
)

type FeatureFunc func(exp *MockExpectation) error

type FeatureItem struct {
	Key         string
	Label       string
	Apply       FeatureFunc
	Description string
}

type Category struct {
	Key      string
	Label    string
	Features []FeatureItem
}

// ---- 1) Registry: wire categories -> features (hook to your existing collectors) ----

func Registry(mc *MockConfigurator) []Category {
	return []Category{
		{
			Key:   "response-behavior",
			Label: "Response Behavior",
			Features: []FeatureItem{
				{"delays", "Delays (fixed / random / progressive)", applyDelays(mc), "Add response delays"},
				{"limits", "Limits (times / reset patterns)", applyLimits(mc), "Limit response count/times"},
				{"priority", "Priority (conflict resolution)", applyPriority(mc), "Set expectation priority"},
				{"headers", "Custom Response Headers", applyResponseHeaders(mc), "Add dynamic headers"},
				{"caching", "Caching (Cache-Control / ETag)", applyCaching(mc), "ETags and cache control"},
				{"compression", "Compression (gzip/deflate)", applyCompression(mc), "Response compression"},
			},
		},
		{
			Key:   "dynamic-content",
			Label: "Dynamic Content",
			Features: []FeatureItem{
				{"templating", "Templating / Echo request", applyTemplating(mc), "Velocity-like templates"},
				{"sequences", "Sequences (multi-stage)", applySequences(mc), "Stage responses"},
				{"conditions", "Conditions (logic trees)", applyConditions(mc), "Conditional bodies"},
				{"state-machine", "State Machine", applyStateMachine(mc), "Stateful transitions"},
				{"data-generation", "Data Generation (fake data)", applyDataGen(mc), "Realistic fakes"},
				{"interpolation", "Advanced String Interpolation", applyInterpolation(mc), "Rich interpolation"},
			},
		},
		{
			Key:   "integration",
			Label: "Integration & Callbacks",
			Features: []FeatureItem{
				{"webhooks", "Webhooks (HTTP callbacks + retry)", applyWebhooks(mc), "Callback hooks"},
				{"custom-code", "Custom Code (Java callback)", applyCustomCode(mc), "Java class callback"},
				{"forward", "Forwarding (smart + fallbacks)", applyForward(mc), "Forward with rules"},
				{"proxy", "Proxy (advanced)", applyProxy(mc), "Proxy upstream"},
				{"transformation", "Transform request/response", applyTransformation(mc), "Mutate payloads"},
				{"event-streaming", "Event Streaming (SSE)", applyEventStreaming(mc), "Stream events"},
			},
		},
		{
			Key:   "connection",
			Label: "Connection Control",
			Features: []FeatureItem{
				{"drop-connection", "Drop Connection (failures)", applyDropConnection(mc), "Network failures"},
				{"chunked-encoding", "Chunked Transfer Encoding", applyChunked(mc), "Chunked control"},
				{"keep-alive", "Keep-Alive / Persistence", applyKeepAlive(mc), "Connection reuse"},
				{"error-simulation", "TCP/HTTP Error Simulation", applyErrors(mc), "Synthetic errors"},
				{"bandwidth", "Bandwidth Throttling", applyBandwidth(mc), "Throttle bytes/sec"},
				{"ssl-behavior", "SSL/TLS Behavior Simulation", applySSL(mc), "TLS quirks"},
			},
		},
		{
			Key:   "testing",
			Label: "Testing Scenarios",
			Features: []FeatureItem{
				{"circuit-breaker", "Circuit Breaker", applyCircuitBreaker(mc), "Breaker patterns"},
				{"rate-limiting", "Rate Limiting (w/ backoff)", applyRateLimiting(mc), "429s + backoff"},
				{"chaos-engineering", "Chaos Engineering", applyChaos(mc), "Random faults"},
				{"load-testing", "Load / Perf Testing", applyLoad(mc), "Perf patterns"},
				{"resilience", "Resilience Testing", applyResilience(mc), "Resilience scenarios"},
				{"security", "Security Testing", applySecurity(mc), "Auth/headers/etc."},
			},
		},
		{
			Key:   "advanced",
			Label: "Advanced Patterns",
			Features: []FeatureItem{
				{"stateful-mocking", "Stateful Mocking", applyStateful(mc), "Stateful interactions"},
				{"workflow-simulation", "Workflow Simulation", applyWorkflow(mc), "Multi-step flows"},
				{"event-driven", "Event-Driven", applyEventDriven(mc), "Emit/consume events"},
				{"microservice-patterns", "Microservice Patterns", applyMicroservices(mc), "Service mesh-ish"},
				{"api-versioning", "API Versioning", applyAPIVersioning(mc), "v1/v2 behavior"},
				{"tenant-isolation", "Tenant Isolation", applyTenantIsolation(mc), "Per-tenant logic"},
			},
		},
	}
}

// ---- 2) Interactive picker ----

func PickFeaturesInteractively(reg []Category) (map[string][]FeatureItem, error) {
	// pick categories
	var catLabels []string
	labelToCat := map[string]Category{}
	for _, c := range reg {
		catLabels = append(catLabels, c.Label)
		labelToCat[c.Label] = c
	}
	var chosenCats []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Pick categories to configure:",
		Options: catLabels,
	}, &chosenCats, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	// for each category, pick features
	result := make(map[string][]FeatureItem)
	for _, cl := range chosenCats {
		c := labelToCat[cl]
		var featLabels []string
		labelToFeat := map[string]FeatureItem{}
		for _, f := range c.Features {
			featLabels = append(featLabels, fmt.Sprintf("%s — %s", f.Label, f.Description))
			labelToFeat[fmt.Sprintf("%s — %s", f.Label, f.Description)] = f
		}
		var chosenFeats []string
		if err := survey.AskOne(&survey.MultiSelect{
			Message: fmt.Sprintf("Pick features in %s:", c.Label),
			Options: featLabels,
		}, &chosenFeats, survey.WithValidator(survey.Required)); err != nil {
			return nil, err
		}
		for _, fl := range chosenFeats {
			result[c.Key] = append(result[c.Key], labelToFeat[fl])
		}
	}
	return result, nil
}
