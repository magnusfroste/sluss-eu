package engine_test

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// stubQuality is a measured-quality source keyed by model id (task-agnostic for
// the test).
type stubQuality map[string]float64

func (s stubQuality) Quality(_ router.TaskType, modelID string) (float64, bool) {
	v, ok := s[modelID]
	return v, ok
}

func measuredCandidates() []registry.Model {
	cost := registry.CostMetadata{Currency: "USD", InputMicrosPerMillionToken: 1_000_000, OutputMicrosPerMillionToken: 1_000_000}
	return []registry.Model{
		{
			ID: "cheapo", ProviderID: "p", Tier: registry.TierBalanced,
			Capabilities:  registry.Capabilities{Chat: true},
			Cost:          registry.CostMetadata{Currency: "USD", InputMicrosPerMillionToken: 200_000, OutputMicrosPerMillionToken: 200_000},
			Enabled:       true,
			Latency:       registry.LatencyMetadata{P95FirstTokenMS: 800},
			QualityScores: map[string]float64{"simple_code_edit": 0.55}, // weak static prior
		},
		{
			ID: "fancy", ProviderID: "p", Tier: registry.TierBalanced,
			Capabilities:  registry.Capabilities{Chat: true},
			Cost:          cost,
			Enabled:       true,
			Latency:       registry.LatencyMetadata{P95FirstTokenMS: 800},
			QualityScores: map[string]float64{"simple_code_edit": 0.85}, // strong static prior
		},
	}
}

func topModel(scored []engine.ScoredCandidate) string {
	if len(scored) == 0 {
		return ""
	}
	return scored[0].Model.ID
}

// Without a measured source the strong static prior (fancy) should win on a
// quality-sensitive task despite costing more.
func TestScoringStaticPriorPicksFancy(t *testing.T) {
	job := &router.JobDescriptor{TaskType: router.TaskSimpleCodeEdit, RiskLevel: router.RiskLow}
	scored := engine.ScoreCandidates(measuredCandidates(), job, registry.TierCheap, engine.FullyHealthy, engine.DefaultWeights())
	if got := topModel(scored); got != "fancy" {
		t.Fatalf("static-prior top = %q, want fancy", got)
	}
}

// With measured evidence that the cheap model is actually good enough (and the
// fancy one is not), routing flips to the cheaper model — the "Efficient" idea.
func TestScoringMeasuredQualityFlipsToCheaper(t *testing.T) {
	job := &router.JobDescriptor{TaskType: router.TaskSimpleCodeEdit, RiskLevel: router.RiskLow}
	q := stubQuality{"cheapo": 0.95, "fancy": 0.60}
	scored := engine.ScoreCandidates(measuredCandidates(), job, registry.TierCheap, engine.FullyHealthy, engine.DefaultWeights(), q)
	if got := topModel(scored); got != "cheapo" {
		t.Fatalf("measured-quality top = %q, want cheapo", got)
	}
}

// A measured source with no entry for a model must not change its scoring
// (falls back to the static prior).
func TestScoringMeasuredFallsBackWhenMissing(t *testing.T) {
	job := &router.JobDescriptor{TaskType: router.TaskSimpleCodeEdit, RiskLevel: router.RiskLow}
	q := stubQuality{} // empty → no measurements
	scored := engine.ScoreCandidates(measuredCandidates(), job, registry.TierCheap, engine.FullyHealthy, engine.DefaultWeights(), q)
	if got := topModel(scored); got != "fancy" {
		t.Fatalf("empty measured source should keep static ranking, got %q", got)
	}
}
