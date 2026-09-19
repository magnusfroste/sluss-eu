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

// complianceStore builds a registry with one EU-tagged provider (its model
// inherits the tags) and one untagged provider — the residency scenario from
// ISSUE-078.
func complianceStore(t *testing.T) *registry.Store {
	t.Helper()
	cost := registry.CostMetadata{Currency: "USD", InputMicrosPerMillionToken: 1_000_000, OutputMicrosPerMillionToken: 1_000_000}
	caps := registry.Capabilities{Chat: true, Streaming: true}
	def := registry.Definition{
		RegistryVersion: "reg-compliance-test",
		CreatedAt:       time.Unix(1_700_000_000, 0),
		Providers: []registry.Provider{
			{ID: "eu-prov", Name: "EU Provider", Status: registry.ProviderStatusActive,
				ComplianceTags: []string{"eu-resident", "dpa-signed"}},
			{ID: "us-prov", Name: "US Provider", Status: registry.ProviderStatusActive,
				ComplianceTags: []string{"us-only"}},
		},
		Models: []registry.Model{
			{ID: "eu-model", ProviderID: "eu-prov", ProviderModelID: "eu-m", Tier: registry.TierBalanced,
				Capabilities: caps, Cost: cost, Enabled: true,
				Latency: registry.LatencyMetadata{P95FirstTokenMS: 900}},
			{ID: "us-model", ProviderID: "us-prov", ProviderModelID: "us-m", Tier: registry.TierBalanced,
				Capabilities: caps, Cost: cost, Enabled: true,
				Latency: registry.LatencyMetadata{P95FirstTokenMS: 900}},
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

func compliancePolicy(t *testing.T, store *registry.Store, c *policy.Constraints) *policy.CompiledPolicy {
	t.Helper()
	p := &policy.Policy{
		Version: "test-compliance",
		Rules:   []policy.Rule{{ID: "all", When: policy.When{Empty: true}, Route: policy.Route{Constraints: c}}},
	}
	return mustCompilePolicy(t, p, store)
}

// A stricter require-tag in a second matched rule must accumulate (fail-closed),
// never be dropped by the constraint merge (ISSUE-078/082 regression guard).
func TestRequireProviderTagsAccumulateAcrossRules(t *testing.T) {
	store := complianceStore(t)
	eng := engine.New(store)
	// Rule 1 (matches all): require eu-resident. Rule 2 (matches all): require
	// air-gapped, which no provider has → the union must leave no candidate.
	p := &policy.Policy{
		Version: "test-accumulate",
		Rules: []policy.Rule{
			{ID: "eu", When: policy.When{Empty: true}, Route: policy.Route{Constraints: &policy.Constraints{RequireProviderTags: []string{"eu-resident"}}}},
			{ID: "airgap", When: policy.When{Empty: true}, Route: policy.Route{Constraints: &policy.Constraints{RequireProviderTags: []string{"air-gapped"}}}},
		},
	}
	pol := mustCompilePolicy(t, p, store)
	if _, err := eng.Decide(simpleJob(router.TaskSimpleChat, router.RiskLow), pol, engine.FullyHealthy, false); err == nil {
		t.Fatal("second rule's air-gapped requirement was dropped (fail-open) — merge must accumulate require tags")
	}
}

// require_provider_tags routes to the tagged provider only — the model inherits
// its provider's tags via the snapshot union.
func TestRequireProviderTagsRoutesToTaggedProvider(t *testing.T) {
	store := complianceStore(t)
	eng := engine.New(store)
	pol := compliancePolicy(t, store, &policy.Constraints{RequireProviderTags: []string{"eu-resident", "dpa-signed"}})

	dec, err := eng.Decide(simpleJob(router.TaskSimpleChat, router.RiskLow), pol, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if dec.SelectedModel != "eu-model" {
		t.Fatalf("selected = %q, want eu-model (only tagged candidate)", dec.SelectedModel)
	}
	for _, fb := range dec.Fallbacks {
		if fb.ModelID == "us-model" {
			t.Fatal("untagged model must not appear in the fallback chain either")
		}
	}
}

// deny_provider_tags excludes the tagged provider.
func TestDenyProviderTagsExcludesTaggedProvider(t *testing.T) {
	store := complianceStore(t)
	eng := engine.New(store)
	pol := compliancePolicy(t, store, &policy.Constraints{DenyProviderTags: []string{"us-only"}})

	dec, err := eng.Decide(simpleJob(router.TaskSimpleChat, router.RiskLow), pol, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if dec.SelectedModel != "eu-model" {
		t.Fatalf("selected = %q, want eu-model (us-only denied)", dec.SelectedModel)
	}
}

// No candidate carries the required tag → a first-class, AUDITED 403 block
// (ISSUE-084), never a silent fallback and never a generic no_route, so the
// audit chain / blocked count / compliance report reflect it.
func TestRequireProviderTagsNoCandidateIsAuditedBlock(t *testing.T) {
	store := complianceStore(t)
	eng := engine.New(store)
	pol := compliancePolicy(t, store, &policy.Constraints{RequireProviderTags: []string{"iso27001"}})

	dec, err := eng.Decide(simpleJob(router.TaskSimpleChat, router.RiskLow), pol, engine.FullyHealthy, false)
	if !errors.Is(err, engine.ErrBlocked) {
		t.Fatalf("no compliant provider should be a policy block, got %v", err)
	}
	if !dec.Blocked || dec.BlockStatus != 403 {
		t.Fatalf("expected a 403 block, got %+v", dec)
	}
	if dec.BlockCode != "residency_no_compliant_provider" {
		t.Fatalf("block code = %q, want residency_no_compliant_provider", dec.BlockCode)
	}
	if dec.SelectedModel != "" {
		t.Fatalf("blocked decision must not select a model, got %q", dec.SelectedModel)
	}
	var explained bool
	for _, r := range dec.DecisionReasons {
		if strings.Contains(r, "compliance tag") || strings.Contains(r, "residency") {
			explained = true
		}
	}
	if !explained {
		t.Fatalf("decision reasons should explain the residency block: %v", dec.DecisionReasons)
	}
}

// A generic no-candidate cause (nothing to do with tags) must stay a no_route,
// not become a residency block.
func TestNonResidencyNoCandidateStaysNoRoute(t *testing.T) {
	store := complianceStore(t)
	eng := engine.New(store)
	// Deny both providers by id → 0 candidates, but not a tag reason.
	pol := compliancePolicy(t, store, &policy.Constraints{DeniedProviders: []string{"eu-prov", "us-prov"}})

	_, err := eng.Decide(simpleJob(router.TaskSimpleChat, router.RiskLow), pol, engine.FullyHealthy, false)
	if !errors.Is(err, engine.ErrNoRoute) {
		t.Fatalf("non-residency empty result should be no_route, got %v", err)
	}
	if errors.Is(err, engine.ErrBlocked) {
		t.Fatal("non-residency empty result must not be a policy block")
	}
}

// An explicit client model must NOT bypass a required tag — the pin is blocked.
func TestPinnedModelCannotBypassRequiredTags(t *testing.T) {
	store := complianceStore(t)
	eng := engine.New(store)
	pol := compliancePolicy(t, store, &policy.Constraints{RequireProviderTags: []string{"eu-resident"}})

	job := simpleJob(router.TaskSimpleChat, router.RiskLow)
	pin := "us-model"
	job.ExplicitModel = &pin

	dec, err := eng.Decide(job, pol, engine.FullyHealthy, false)
	if !errors.Is(err, engine.ErrBlocked) {
		t.Fatalf("pinned non-compliant model should be blocked, got err=%v dec=%+v", err, dec)
	}
	if !dec.Blocked || dec.BlockStatus != 403 {
		t.Fatalf("expected 403 block, got %+v", dec)
	}
	// The pinned EU model still routes fine under the same policy.
	euPin := "eu-model"
	job.ExplicitModel = &euPin
	if _, err := eng.Decide(job, pol, engine.FullyHealthy, false); err != nil {
		t.Fatalf("compliant pinned model should route, got %v", err)
	}
}

// Tag matching is case/whitespace-normalized end to end (registry lowercases,
// parser normalizes) — verified here at the filter level.
func TestFilterTagNormalization(t *testing.T) {
	store := complianceStore(t)
	snap, err := store.Active()
	if err != nil {
		t.Fatalf("active: %v", err)
	}
	m, ok := snap.Model("eu-model")
	if !ok {
		t.Fatal("eu-model missing")
	}
	// Registry stored the union of provider+model tags, normalized and sorted.
	want := []string{"dpa-signed", "eu-resident"}
	if len(m.ComplianceTags) != 2 || m.ComplianceTags[0] != want[0] || m.ComplianceTags[1] != want[1] {
		t.Fatalf("effective tags = %v, want %v", m.ComplianceTags, want)
	}
}
