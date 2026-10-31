package engine_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// A residency block must also say why the compliant model was not used — here
// a local model below the task's minimum tier — not only that the cloud model
// lacks the tag (the demo case: personal data, "unknown_high_risk" task).
func TestResidencyBlockNamesTheCompliantModelsOtherExclusion(t *testing.T) {
	cost := registry.CostMetadata{Currency: "USD", InputMicrosPerMillionToken: 1, OutputMicrosPerMillionToken: 1}
	caps := registry.Capabilities{Chat: true, Streaming: true}
	snap, err := registry.NewSnapshot(registry.Definition{
		RegistryVersion: "reg-residency-reasons", CreatedAt: time.Unix(1_700_000_000, 0),
		Providers: []registry.Provider{
			{ID: "dgx", Name: "DGX", Status: registry.ProviderStatusActive, ComplianceTags: []string{"local"}},
			{ID: "cloud", Name: "Cloud", Status: registry.ProviderStatusActive},
		},
		Models: []registry.Model{
			{ID: "local-balanced", ProviderID: "dgx", ProviderModelID: "l", Tier: registry.TierBalanced, Capabilities: caps, Cost: cost, Enabled: true},
			{ID: "cloud-premium", ProviderID: "cloud", ProviderModelID: "c", Tier: registry.TierPremium, Capabilities: caps, Cost: cost, Enabled: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatal(err)
	}
	pol := compliancePolicy(t, store, &policy.Constraints{RequireProviderTags: []string{"local"}})
	dec, err := engine.New(store).Decide(simpleJob(router.TaskUnknownHighRisk, router.RiskHigh), pol, engine.FullyHealthy, false)
	if !errors.Is(err, engine.ErrBlocked) {
		t.Fatalf("want a residency block, got %v", err)
	}
	joined := strings.Join(dec.DecisionReasons, " | ")
	if !strings.Contains(joined, "model local-balanced excluded: tier balanced below minimum premium") {
		t.Fatalf("block does not explain the local model's exclusion: %s", joined)
	}
}
