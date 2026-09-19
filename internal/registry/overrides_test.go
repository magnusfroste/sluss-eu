package registry

import "testing"

func TestApplyProviderModelOverrides(t *testing.T) {
	def := ApplyProviderModelOverrides(OpenRouterDefinition(), map[Tier]string{
		TierPremium: "z-ai/glm-4.6",
		TierCheap:   "", // empty leaves the default
	})

	got := map[Tier]string{}
	for _, m := range def.Models {
		got[m.Tier] = m.ProviderModelID
	}
	if got[TierPremium] != "z-ai/glm-4.6" {
		t.Errorf("premium override not applied: %q", got[TierPremium])
	}
	if got[TierCheap] != "openai/gpt-4o-mini" {
		t.Errorf("empty override should keep default, got %q", got[TierCheap])
	}

	// The overridden definition must still build a valid snapshot.
	if _, err := NewSnapshot(def); err != nil {
		t.Fatalf("overridden definition should still be valid: %v", err)
	}
}

func TestApplyProviderModelOverridesNoneIsNoop(t *testing.T) {
	def := ApplyProviderModelOverrides(OpenRouterDefinition(), nil)
	for _, m := range def.Models {
		if m.Tier == TierPremium && m.ProviderModelID != "anthropic/claude-sonnet-4.5" {
			t.Errorf("nil overrides changed premium: %q", m.ProviderModelID)
		}
	}
}
