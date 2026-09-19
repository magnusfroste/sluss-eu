package eventlog

import (
	"context"
	"testing"
)

func TestRequestLogTrackerMergesDecisionAndAttempt(t *testing.T) {
	tr := NewRequestLogTracker(10)
	ctx := context.Background()

	tr.Handle(ctx, Event{Type: EventTypeDecision, Decision: &DecisionEvent{
		RequestID: "req_1", TaskType: "summarization", SelectedModel: "cheap-general",
		SelectedProvider: "openrouter", PromptTokens: 12, EstimatedCostUSD: 0.001,
	}})
	// Successful attempt fills actual tokens/cost.
	tr.Handle(ctx, Event{Type: EventTypeAttempt, Attempt: &AttemptEvent{
		RequestID: "req_1", ModelID: "cheap-general", ProviderID: "openrouter",
		Success: true, InputTokens: 12, OutputTokens: 40, ActualCostUSD: 0.000123,
	}})

	rec := tr.Recent(10)
	if len(rec) != 1 {
		t.Fatalf("want 1 record, got %d", len(rec))
	}
	r := rec[0]
	if r.TaskType != "summarization" || r.Model != "cheap-general" {
		t.Errorf("classification not preserved: %+v", r)
	}
	if r.OutputTokens != 40 || r.CostUSD != 0.000123 {
		t.Errorf("attempt did not fill tokens/cost: %+v", r)
	}
}

func TestRequestLogTrackerRingAndOrder(t *testing.T) {
	tr := NewRequestLogTracker(3)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		tr.Handle(context.Background(), Event{Type: EventTypeDecision, Decision: &DecisionEvent{RequestID: id, SelectedModel: "m"}})
	}
	rec := tr.Recent(10)
	if len(rec) != 3 {
		t.Fatalf("ring should retain 3, got %d", len(rec))
	}
	// Newest first: e, d, c (a,b evicted).
	if rec[0].RequestID != "e" || rec[1].RequestID != "d" || rec[2].RequestID != "c" {
		t.Errorf("unexpected order: %s %s %s", rec[0].RequestID, rec[1].RequestID, rec[2].RequestID)
	}
}

func TestRequestLogTrackerNilSafe(t *testing.T) {
	var tr *RequestLogTracker
	if got := tr.Recent(5); got != nil {
		t.Errorf("nil tracker Recent should be nil, got %v", got)
	}
}
