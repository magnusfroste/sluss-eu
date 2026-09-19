package engine_test

import (
	"errors"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/policy/builtin"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// dgxLikeStore mirrors the real on-prem setup: a cloud provider (default) and a
// `local`-tagged DGX/vLLM provider whose model inherits the tag. This is exactly
// what the Providers page produces when you tag the DGX connection `local`.
func dgxLikeStore(t *testing.T) *registry.Store {
	t.Helper()
	cost := registry.CostMetadata{Currency: "USD", InputMicrosPerMillionToken: 1_000_000, OutputMicrosPerMillionToken: 1_000_000}
	caps := registry.Capabilities{Chat: true, Streaming: true}
	def := registry.Definition{
		RegistryVersion: "reg-dgx-test",
		CreatedAt:       time.Unix(1_700_000_000, 0),
		Providers: []registry.Provider{
			{ID: "openai", Name: "OpenAI (cloud)", Status: registry.ProviderStatusActive,
				ComplianceTags: []string{"cloud"}},
			{ID: "dgx", Name: "PrivAI DGX", Status: registry.ProviderStatusActive,
				ComplianceTags: []string{"local", "on-prem"}},
		},
		Models: []registry.Model{
			{ID: "gpt-cloud", ProviderID: "openai", ProviderModelID: "gpt-4o-mini", Tier: registry.TierBalanced,
				Capabilities: caps, Cost: cost, Enabled: true,
				Latency: registry.LatencyMetadata{P95FirstTokenMS: 400}},
			{ID: "qwen36-27b", ProviderID: "dgx", ProviderModelID: "qwen36-27b", Tier: registry.TierBalanced,
				Capabilities: caps, Cost: cost, Enabled: true,
				Latency: registry.LatencyMetadata{P95FirstTokenMS: 3000}},
		},
	}
	snap, err := registry.NewSnapshot(def)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return store
}

// compileShippedPiiLocalPolicy compiles the real pii-local policy embedded in
// the binary — the one an operator selects with ROUTER_POLICY_PATH=builtin:pii-local.
func compileShippedPiiLocalPolicy(t *testing.T, store *registry.Store) *policy.CompiledPolicy {
	t.Helper()
	data, ok := builtin.Load("pii-local")
	if !ok {
		t.Fatal("built-in policy pii-local missing")
	}
	p, err := policy.Parse(data)
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	return mustCompilePolicy(t, p, store)
}

// TestPiiLocalPolicyRoutesPIIToLocalModel is the end-to-end proof of the CISO
// signature moment with the real shipped policy: a PII-sensitive prompt must
// select the local (DGX) model and never the cloud one.
func TestPiiLocalPolicyRoutesPIIToLocalModel(t *testing.T) {
	store := dgxLikeStore(t)
	eng := engine.New(store)
	pol := compileShippedPiiLocalPolicy(t, store)

	piiJob := simpleJob(router.TaskSimpleChat, router.RiskLow)
	piiJob.Sensitivity = router.SensitivityPII

	dec, err := eng.Decide(piiJob, pol, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("PII decide: %v", err)
	}
	if dec.SelectedModel != "qwen36-27b" {
		t.Fatalf("PII selected = %q, want qwen36-27b (local)", dec.SelectedModel)
	}
	for _, fb := range dec.Fallbacks {
		if fb.ModelID == "gpt-cloud" {
			t.Fatal("cloud model must not be in the fallback chain for PII")
		}
	}

	// A non-sensitive prompt is free to use the cloud model (power when it's safe).
	safeJob := simpleJob(router.TaskSimpleChat, router.RiskLow)
	safeDec, err := eng.Decide(safeJob, pol, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("safe decide: %v", err)
	}
	if safeDec.SelectedModel == "" {
		t.Fatal("non-PII prompt should route to some model")
	}
}

// TestPiiLocalPolicyBlocksWhenNoLocalModel proves the fail-closed guarantee: if
// the DGX (only local provider) is unhealthy/absent, a PII prompt is a first-class
// audited 403 block — never a silent fallback to the cloud.
func TestPiiLocalPolicyBlocksWhenNoLocalModel(t *testing.T) {
	store := dgxLikeStore(t)
	eng := engine.New(store)
	pol := compileShippedPiiLocalPolicy(t, store)

	// DGX down: only the cloud provider is healthy.
	health := engine.StaticHealth{"openai": 1.0, "dgx": 0.0}

	piiJob := simpleJob(router.TaskSimpleChat, router.RiskLow)
	piiJob.Sensitivity = router.SensitivityPII

	dec, err := eng.Decide(piiJob, pol, health, false)
	if !errors.Is(err, engine.ErrBlocked) {
		t.Fatalf("PII with no healthy local model should be a policy block, got err=%v dec=%+v", err, dec)
	}
	if !dec.Blocked || dec.BlockStatus != 403 {
		t.Fatalf("expected a 403 block, got %+v", dec)
	}
	if dec.SelectedModel != "" {
		t.Fatalf("blocked decision must not select a model (no silent cloud fallback), got %q", dec.SelectedModel)
	}
}
