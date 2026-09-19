package registry

import "time"

func DefaultDefinition() Definition {
	return Definition{
		RegistryVersion: "registry-mvp-2026-05-19",
		CreatedAt:       time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC),
		Providers: []Provider{
			{
				ID:            "openai",
				Name:          "OpenAI",
				Status:        ProviderStatusActive,
				BaseURL:       "https://api.openai.com/v1",
				AuthSecretRef: "OPENAI_API_KEY",
			},
			{
				ID:            "anthropic",
				Name:          "Anthropic",
				Status:        ProviderStatusActive,
				BaseURL:       "https://api.anthropic.com",
				AuthSecretRef: "ANTHROPIC_API_KEY",
			},
		},
		Models: []Model{
			{
				ID:              "cheap-general",
				ProviderID:      "openai",
				ProviderModelID: "gpt-4.1-mini",
				Tier:            TierCheap,
				Capabilities: Capabilities{
					Chat:       true,
					Streaming:  true,
					ToolCalls:  true,
					JSONSchema: true,
				},
				Cost: CostMetadata{
					Currency:                    "USD",
					InputMicrosPerMillionToken:  400000,
					OutputMicrosPerMillionToken: 1600000,
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
				ProviderID:      "openai",
				ProviderModelID: "gpt-4.1",
				Tier:            TierBalanced,
				Capabilities: Capabilities{
					Chat:        true,
					Streaming:   true,
					ToolCalls:   true,
					JSONSchema:  true,
					LongContext: true,
				},
				Cost: CostMetadata{
					Currency:                    "USD",
					InputMicrosPerMillionToken:  2000000,
					OutputMicrosPerMillionToken: 8000000,
				},
				ContextWindowTokens: 1000000,
				Enabled:             true,
				Latency:             LatencyMetadata{P50FirstTokenMS: 700, P95FirstTokenMS: 1800},
				QualityScores:       map[string]float64{"simple_code_edit": 0.84, "hard_code_debugging": 0.68},
				Strengths:           []string{"code", "tool_use", "long_context"},
			},
			{
				ID:              "premium-reasoning",
				ProviderID:      "anthropic",
				ProviderModelID: "claude-sonnet-4",
				Tier:            TierPremium,
				Capabilities: Capabilities{
					Chat:       true,
					Streaming:  true,
					ToolCalls:  true,
					JSONSchema: true,
				},
				Cost: CostMetadata{
					Currency:                    "USD",
					InputMicrosPerMillionToken:  3000000,
					OutputMicrosPerMillionToken: 15000000,
				},
				ContextWindowTokens: 200000,
				Enabled:             true,
				Latency:             LatencyMetadata{P50FirstTokenMS: 850, P95FirstTokenMS: 2200},
				QualityScores:       map[string]float64{"hard_reasoning": 0.88, "security_review": 0.82},
				Strengths:           []string{"reasoning", "security_review", "analysis"},
			},
		},
	}
}

func DefaultSnapshot() (*Snapshot, error) {
	return NewSnapshot(DefaultDefinition())
}
