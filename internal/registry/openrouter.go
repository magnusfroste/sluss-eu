package registry

import "time"

// OpenRouterDefinition is a registry variant that routes the three standard
// model tiers through OpenRouter (https://openrouter.ai), an OpenAI-compatible
// aggregator. Model IDs match DefaultDefinition (so policies and evals are
// unchanged) but the provider is `openrouter` and ProviderModelID values are
// OpenRouter model slugs.
//
// Slugs, prices and context windows were verified against OpenRouter's
// /api/v1/models on 2026-06-14. To refresh:
//
//	curl -s https://openrouter.ai/api/v1/models | jq '.data[] | select(.id=="<slug>") | {pricing, context_length}'
//
// Cost is stored as micro-USD per million tokens = pricing_per_token_USD * 1e12.
func OpenRouterDefinition() Definition {
	return Definition{
		RegistryVersion: "registry-openrouter-2026-06-14",
		CreatedAt:       time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC),
		Providers: []Provider{
			{
				ID:            "openrouter",
				Name:          "OpenRouter",
				Status:        ProviderStatusActive,
				BaseURL:       "https://openrouter.ai/api/v1",
				AuthSecretRef: "OPENROUTER_API_KEY",
			},
		},
		Models: []Model{
			{
				ID:              "cheap-general",
				ProviderID:      "openrouter",
				ProviderModelID: "openai/gpt-4o-mini",
				Tier:            TierCheap,
				Capabilities: Capabilities{
					Chat:       true,
					Streaming:  true,
					ToolCalls:  true,
					JSONSchema: true,
					Vision:     true,
				},
				Cost: CostMetadata{
					Currency:                    "USD",
					InputMicrosPerMillionToken:  150000,
					OutputMicrosPerMillionToken: 600000,
				},
				ContextWindowTokens: 128000,
				Enabled:             true,
				Latency:             LatencyMetadata{P50FirstTokenMS: 450, P95FirstTokenMS: 1200},
				QualityScores:       map[string]float64{"simple_code_edit": 0.72, "summarization": 0.78},
				Strengths:           []string{"summarization", "simple_edits", "json"},
				Weaknesses:          []string{"hard_reasoning"},
			},
			{
				ID:              "balanced-coder",
				ProviderID:      "openrouter",
				ProviderModelID: "openai/gpt-4o",
				Tier:            TierBalanced,
				Capabilities: Capabilities{
					Chat:        true,
					Streaming:   true,
					ToolCalls:   true,
					JSONSchema:  true,
					LongContext: true,
					Vision:      true,
				},
				Cost: CostMetadata{
					Currency:                    "USD",
					InputMicrosPerMillionToken:  2500000,
					OutputMicrosPerMillionToken: 10000000,
				},
				ContextWindowTokens: 128000,
				Enabled:             true,
				Latency:             LatencyMetadata{P50FirstTokenMS: 700, P95FirstTokenMS: 1800},
				QualityScores:       map[string]float64{"simple_code_edit": 0.84, "hard_code_debugging": 0.68},
				Strengths:           []string{"code", "tool_use", "long_context"},
			},
			{
				ID:              "premium-reasoning",
				ProviderID:      "openrouter",
				ProviderModelID: "anthropic/claude-sonnet-4.5",
				Tier:            TierPremium,
				Capabilities: Capabilities{
					Chat:        true,
					Streaming:   true,
					ToolCalls:   true,
					JSONSchema:  true,
					LongContext: true,
					Vision:      true,
				},
				Cost: CostMetadata{
					Currency:                    "USD",
					InputMicrosPerMillionToken:  3000000,
					OutputMicrosPerMillionToken: 15000000,
				},
				ContextWindowTokens: 1000000,
				Enabled:             true,
				Latency:             LatencyMetadata{P50FirstTokenMS: 850, P95FirstTokenMS: 2200},
				QualityScores:       map[string]float64{"hard_reasoning": 0.88, "security_review": 0.82},
				Strengths:           []string{"reasoning", "security_review", "analysis"},
			},
		},
	}
}

// OpenRouterSnapshot builds a snapshot from OpenRouterDefinition.
func OpenRouterSnapshot() (*Snapshot, error) {
	return NewSnapshot(OpenRouterDefinition())
}
