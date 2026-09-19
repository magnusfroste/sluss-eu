package spend

import (
	"context"
	"testing"

	"github.com/magnusfroste/sluss/internal/eventlog"
)

// decision then successful attempt for the same request must end at the actual
// cost — counted once, never estimate + actual.
func TestActualCostCountedOnceNoDoubleCount(t *testing.T) {
	tr := New()
	ctx := context.Background()

	tr.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
		TenantID: "tn", SelectedModel: "m", SelectedProvider: "p", EstimatedCostUSD: 0.10,
	}})
	tr.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
		TenantID: "tn", ModelID: "m", ProviderID: "p", Success: true,
		InputTokens: 100, OutputTokens: 50, ActualCostUSD: 0.04, EstimatedCostUSD: 0.10,
	}})

	if got := tr.TotalCostUSD(); got != 0.04 {
		t.Fatalf("total cost = %v, want 0.04 (actual, not estimate+actual)", got)
	}
	model := tr.ByModel()
	if len(model) != 1 || model[0].Requests != 1 || model[0].CostUSD != 0.04 {
		t.Fatalf("model row = %+v, want 1 request @ 0.04", model)
	}
	ten := tr.ByTenant()
	if len(ten) != 1 || ten[0].CostUSD != 0.04 {
		t.Fatalf("tenant row = %+v, want cost 0.04", ten)
	}
}

// When provider usage is unavailable (e.g. streaming), spend falls back to the
// decision-time estimate carried on the attempt.
func TestEstimateFallbackWhenNoActual(t *testing.T) {
	tr := New()
	ctx := context.Background()
	tr.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
		TenantID: "tn", SelectedModel: "m", SelectedProvider: "p", EstimatedCostUSD: 0.07,
	}})
	tr.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
		TenantID: "tn", ModelID: "m", ProviderID: "p", Success: true,
		ActualCostUSD: 0, EstimatedCostUSD: 0.07,
	}})
	if got := tr.TotalCostUSD(); got != 0.07 {
		t.Fatalf("total cost = %v, want 0.07 (estimate fallback)", got)
	}
}

// Failed attempts never reach recordAttempt (gated on Success), so a blocked or
// failed request accrues no cost.
func TestFailedAttemptAccruesNoCost(t *testing.T) {
	tr := New()
	ctx := context.Background()
	tr.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
		TenantID: "tn", SelectedModel: "m", SelectedProvider: "p", EstimatedCostUSD: 0.10,
	}})
	tr.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
		TenantID: "tn", ModelID: "m", ProviderID: "p", Success: false, EstimatedCostUSD: 0.10,
	}})
	if got := tr.TotalCostUSD(); got != 0 {
		t.Fatalf("total cost = %v, want 0 (no successful attempt)", got)
	}
	if tr.TotalRequests() != 1 {
		t.Fatalf("requests = %d, want 1 (counted on decision)", tr.TotalRequests())
	}
}
