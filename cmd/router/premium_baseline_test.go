package main

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/registry"
)

// With several enabled premium models the savings baseline must be
// deterministic: the most expensive one (combined in+out price), never
// whichever iteration order happens to yield first.
func TestPremiumPricingPicksMostExpensivePremium(t *testing.T) {
	def := registry.Definition{
		RegistryVersion: "test",
		Providers:       []registry.Provider{{ID: "p", Status: registry.ProviderStatusActive}},
		Models: []registry.Model{
			{
				ID: "glm-premium", ProviderID: "p", ProviderModelID: "glm-4.6",
				Tier: registry.TierPremium, Enabled: true,
				Capabilities: registry.Capabilities{Chat: true},
				Cost:         registry.CostMetadata{InputMicrosPerMillionToken: 600_000, OutputMicrosPerMillionToken: 2_200_000},
			},
			{
				ID: "flagship", ProviderID: "p", ProviderModelID: "openai/gpt-4o",
				Tier: registry.TierPremium, Enabled: true,
				Capabilities: registry.Capabilities{Chat: true},
				Cost:         registry.CostMetadata{InputMicrosPerMillionToken: 2_500_000, OutputMicrosPerMillionToken: 10_000_000},
			},
			{
				ID: "cheap", ProviderID: "p", ProviderModelID: "qwen36-35b",
				Tier: registry.TierCheap, Enabled: true,
				Capabilities: registry.Capabilities{Chat: true},
				Cost:         registry.CostMetadata{InputMicrosPerMillionToken: 100_000, OutputMicrosPerMillionToken: 200_000},
			},
		},
	}
	snap, err := registry.NewSnapshot(def)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	in, out, name := premiumPricing(snap)
	if name != "openai/gpt-4o" {
		t.Fatalf("baseline model = %q, want openai/gpt-4o (most expensive premium)", name)
	}
	if in != 2_500_000 || out != 10_000_000 {
		t.Fatalf("baseline pricing = %d/%d, want 2500000/10000000", in, out)
	}
}
