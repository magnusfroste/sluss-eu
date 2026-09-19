package registry

import (
	"strings"
	"testing"
)

func TestOpenRouterSnapshotShape(t *testing.T) {
	snap, err := OpenRouterSnapshot()
	if err != nil {
		t.Fatalf("OpenRouterSnapshot: %v", err)
	}

	// Provider is OpenRouter with the OpenAI-compatible base URL and key ref.
	p, ok := snap.Provider("openrouter")
	if !ok {
		t.Fatal("expected openrouter provider")
	}
	if p.BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("base url = %q", p.BaseURL)
	}
	if p.AuthSecretRef != "OPENROUTER_API_KEY" {
		t.Errorf("auth secret ref = %q", p.AuthSecretRef)
	}

	// The three standard tiers exist with the same model IDs as the default
	// registry (so policies and evals are unchanged), routed via openrouter with
	// OpenRouter slugs and usable cost metadata.
	wantTiers := map[string]Tier{
		"cheap-general":     TierCheap,
		"balanced-coder":    TierBalanced,
		"premium-reasoning": TierPremium,
	}
	for id, tier := range wantTiers {
		m, ok := snap.Model(id)
		if !ok {
			t.Fatalf("missing model %q", id)
		}
		if m.ProviderID != "openrouter" {
			t.Errorf("%s provider = %q, want openrouter", id, m.ProviderID)
		}
		if m.Tier != tier {
			t.Errorf("%s tier = %q, want %q", id, m.Tier, tier)
		}
		if !strings.Contains(m.ProviderModelID, "/") {
			t.Errorf("%s provider_model_id %q does not look like an OpenRouter slug", id, m.ProviderModelID)
		}
		if !m.Cost.Available() {
			t.Errorf("%s has no usable cost metadata", id)
		}
		if !m.Capabilities.Streaming {
			t.Errorf("%s should advertise streaming", id)
		}
	}
}
