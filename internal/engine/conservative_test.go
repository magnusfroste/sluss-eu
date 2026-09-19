package engine_test

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// lowConfidenceJob is a default/uncertain classification that would normally
// floor at the cheap tier.
func lowConfidenceJob() *router.JobDescriptor {
	return &router.JobDescriptor{
		TaskType:       router.TaskSimpleChat,
		RiskLevel:      router.RiskLow,
		TaskConfidence: 0.3,
	}
}

func TestConservativeRaisesFloorForLowConfidence(t *testing.T) {
	job := lowConfidenceJob()

	// Without the conservative flag, an uncertain low-risk task floors at cheap.
	if got := engine.MinimumTierForTask(job, policy.Route{}); got != registry.TierCheap {
		t.Fatalf("non-conservative floor = %s, want cheap", got)
	}

	// With the flag set, the floor is raised to balanced.
	job.Conservative = true
	if got := engine.MinimumTierForTask(job, policy.Route{}); got != registry.TierBalanced {
		t.Errorf("conservative floor = %s, want balanced", got)
	}
}

func TestConservativeNeverLowersStrongerFloor(t *testing.T) {
	// A premium task stays premium even under conservative mode (no downgrade).
	job := &router.JobDescriptor{
		TaskType:     router.TaskSecurityReview,
		RiskLevel:    router.RiskHigh,
		Conservative: true,
	}
	if got := engine.MinimumTierForTask(job, policy.Route{}); got != registry.TierPremium {
		t.Errorf("conservative must not lower premium, got %s", got)
	}
}

func TestEngineConservativeFlagGatesByConfidence(t *testing.T) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	eng := engine.New(store)

	// Default: conservative off.
	if eng.Conservative() {
		t.Fatal("conservative should default off")
	}

	lowConf := func() *router.JobDescriptor {
		return &router.JobDescriptor{TaskType: router.TaskSimpleChat, RiskLevel: router.RiskLow, TaskConfidence: 0.3}
	}
	highConf := func() *router.JobDescriptor {
		return &router.JobDescriptor{TaskType: router.TaskSimpleChat, RiskLevel: router.RiskLow, TaskConfidence: 0.95}
	}

	// Off → job not flagged conservative regardless of confidence.
	j := lowConf()
	if _, err := eng.Decide(j, nil, engine.FullyHealthy, false); err != nil {
		t.Fatalf("decide: %v", err)
	}
	if j.Conservative {
		t.Error("conservative off must not flag the job")
	}

	// On → low-confidence flagged, high-confidence not.
	eng.SetConservative(true)
	jl := lowConf()
	if _, err := eng.Decide(jl, nil, engine.FullyHealthy, false); err != nil {
		t.Fatalf("decide low: %v", err)
	}
	if !jl.Conservative {
		t.Error("low-confidence job should be flagged conservative")
	}

	jh := highConf()
	if _, err := eng.Decide(jh, nil, engine.FullyHealthy, false); err != nil {
		t.Fatalf("decide high: %v", err)
	}
	if jh.Conservative {
		t.Error("high-confidence job should not be flagged conservative")
	}
}

func TestEngineConservativeRoutesUncertainAboveCheap(t *testing.T) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	eng := engine.New(store)
	eng.SetConservative(true)

	job := &router.JobDescriptor{TaskType: router.TaskSimpleChat, RiskLevel: router.RiskLow, TaskConfidence: 0.3}
	dec, err := eng.Decide(job, nil, engine.FullyHealthy, false)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	model, ok := snap.Model(dec.SelectedModel)
	if !ok {
		t.Fatalf("selected model %q not in registry", dec.SelectedModel)
	}
	if engine.TierOrdinal(model.Tier) < engine.TierOrdinal(registry.TierBalanced) {
		t.Errorf("conservative routing chose %s (tier %s), want >= balanced", dec.SelectedModel, model.Tier)
	}
}
