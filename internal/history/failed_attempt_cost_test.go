package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/eventlog"
)

// ISSUE-113: a request whose provider call failed must not keep the pre-call
// estimate as spend — that made a dead provider look like negative savings.
// A later successful fallback attempt restores real cost.
func TestFailedAttemptZeroesEstimatedCost(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	ctx := context.Background()

	decide := func(id string) {
		store.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
			RequestID: id, TenantID: "tn", TaskType: "simple_chat", RiskLevel: "low",
			SelectedModel: "glm-premium", SelectedProvider: "zai",
			EstimatedCostUSD: 0.005, DecidedAt: time.Now(),
		}})
	}
	attempt := func(id, model, prov string, idx int, ok bool, actual float64, in, out int) {
		store.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
			RequestID: id, TenantID: "tn", ProviderID: prov, ModelID: model, AttemptIndex: idx,
			Success: ok, ActualCostUSD: actual, EstimatedCostUSD: 0.005,
			InputTokens: in, OutputTokens: out, AttemptedAt: time.Now(),
		}})
	}

	// Request 1: primary fails, nothing else — cost must be 0, not the estimate.
	decide("r_fail")
	attempt("r_fail", "glm-premium", "zai", 0, false, 0, 0, 0)

	// Request 2: primary fails, fallback succeeds with real usage — real cost wins.
	decide("r_fb")
	attempt("r_fb", "glm-premium", "zai", 0, false, 0, 0, 0)
	attempt("r_fb", "cheap-general", "openrouter", 1, true, 0.0001, 10, 5)

	total := store.TotalCostUSD()
	if total < 0.00009 || total > 0.00011 {
		t.Fatalf("total cost = %v, want ~0.0001 (only the successful fallback attempt)", total)
	}
	for _, r := range store.ByModel() {
		if r.ModelID == "cheap-general" && r.CostUSD < 0.00009 {
			t.Fatalf("fallback success should carry real cost, got %+v", r)
		}
		if r.ModelID == "glm-premium" && r.CostUSD != 0 {
			t.Fatalf("failed request should carry zero cost, got %+v", r)
		}
	}
}
