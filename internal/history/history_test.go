package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/spend"
)

func openTest(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func decision(reqID, task, model string) eventlog.Event {
	return eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
		RequestID: reqID, TenantID: "tn_1", TaskType: task, RiskLevel: "low",
		SelectedModel: model, SelectedProvider: "openrouter", ProviderModelID: "openai/gpt-4o-mini",
		PromptTokens: 10, EstimatedCostUSD: 0.001,
	}}
}

func attempt(reqID string, in, out int, cost float64) eventlog.Event {
	return eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
		RequestID: reqID, ModelID: "cheap-general", ProviderID: "openrouter",
		Success: true, InputTokens: in, OutputTokens: out, ActualCostUSD: cost,
	}}
}

func TestDecisionAttemptJoin(t *testing.T) {
	s, _ := openTest(t)
	ctx := context.Background()

	s.Handle(ctx, decision("req_1", "summarization", "cheap-general"))
	s.Handle(ctx, attempt("req_1", 12, 40, 0.000123))

	rec := s.Recent(10)
	if len(rec) != 1 {
		t.Fatalf("want 1 record, got %d", len(rec))
	}
	r := rec[0]
	if r.TaskType != "summarization" || r.Model != "cheap-general" {
		t.Errorf("classification not preserved: %+v", r)
	}
	if r.InputTokens != 12 || r.OutputTokens != 40 || r.CostUSD != 0.000123 {
		t.Errorf("attempt did not fill tokens/cost: %+v", r)
	}
}

func TestPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s.Handle(context.Background(), decision("req_1", "simple_chat", "cheap-general"))
	s.Handle(context.Background(), attempt("req_1", 5, 9, 0.00001))
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	if got := s2.TotalRequests(); got != 1 {
		t.Fatalf("history did not survive reopen: total=%d", got)
	}
	if rec := s2.Recent(5); len(rec) != 1 || rec[0].OutputTokens != 9 {
		t.Fatalf("record not intact after reopen: %+v", rec)
	}
}

func TestAggregates(t *testing.T) {
	s, _ := openTest(t)
	ctx := context.Background()
	for i, m := range []string{"cheap-general", "cheap-general", "premium-reasoning"} {
		id := string(rune('a' + i))
		s.Handle(ctx, decision("req_"+id, "simple_chat", m))
		s.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
			RequestID: "req_" + id, ModelID: m, ProviderID: "openrouter",
			Success: true, InputTokens: 100, OutputTokens: 50, ActualCostUSD: 0.01,
		}})
	}

	if got := s.TotalRequests(); got != 3 {
		t.Fatalf("total = %d, want 3", got)
	}
	byModel := s.ByModel()
	if len(byModel) != 2 {
		t.Fatalf("want 2 model rows, got %d: %+v", len(byModel), byModel)
	}
	if byModel[0].ModelID != "cheap-general" || byModel[0].Requests != 2 || byModel[0].InputTokens != 200 {
		t.Errorf("cheap aggregate wrong: %+v", byModel[0])
	}
	tenants := s.ByTenant()
	if len(tenants) != 1 || tenants[0].TenantID != "tn_1" || tenants[0].Requests != 3 {
		t.Errorf("tenant aggregate wrong: %+v", tenants)
	}
}

func TestBlockedExcludedFromAggregates(t *testing.T) {
	s, _ := openTest(t)
	e := decision("req_b", "security_review", "premium-reasoning")
	e.Decision.Blocked = true
	s.Handle(context.Background(), e)

	if got := s.TotalRequests(); got != 0 {
		t.Errorf("blocked requests must not count in totals, got %d", got)
	}
	if rec := s.Recent(5); len(rec) != 1 || !rec[0].Blocked {
		t.Errorf("blocked request should still appear in the log: %+v", rec)
	}
}

func TestImportSpendSnapshotOnlyWhenEmpty(t *testing.T) {
	s, _ := openTest(t)
	snap := spend.Snapshot{Models: []spend.ModelRow{
		{ModelID: "cheap-general", ProviderID: "openrouter", Requests: 8, InputTokens: 1000, OutputTokens: 500, CostUSD: 0.02},
	}}
	if err := s.ImportSpendSnapshot(snap); err != nil {
		t.Fatalf("import: %v", err)
	}
	if got := s.TotalCostUSD(); got != 0.02 {
		t.Errorf("imported cost = %v, want 0.02", got)
	}
	// Second import must be a no-op (store not empty).
	if err := s.ImportSpendSnapshot(snap); err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if got := s.TotalCostUSD(); got != 0.02 {
		t.Errorf("re-import must not double-count, got %v", got)
	}
}

func TestByRequestID(t *testing.T) {
	s, _ := openTest(t)
	ctx := context.Background()
	s.Handle(ctx, decision("req_abc", "security_review", "premium-reasoning"))
	s.Handle(ctx, attempt("req_abc", 21, 300, 0.005))

	rec, ok := s.ByRequestID("req_abc")
	if !ok {
		t.Fatal("expected to find req_abc")
	}
	if rec.TaskType != "security_review" || rec.OutputTokens != 300 {
		t.Errorf("wrong record: %+v", rec)
	}
	if _, ok := s.ByRequestID("nope"); ok {
		t.Error("unknown request id should not be found")
	}
	if _, ok := s.ByRequestID(""); ok {
		t.Error("empty request id should not be found")
	}
}

func TestKVRoundTripAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, ok := s.KVGet("demo_sessions"); ok {
		t.Error("missing key should report not-present")
	}
	if err := s.KVSet("demo_sessions", `[{"id":"s1"}]`); err != nil {
		t.Fatalf("KVSet: %v", err)
	}
	if err := s.KVSet("demo_sessions", `[{"id":"s2"}]`); err != nil { // upsert
		t.Fatalf("KVSet upsert: %v", err)
	}
	s.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	v, ok := s2.KVGet("demo_sessions")
	if !ok || v != `[{"id":"s2"}]` {
		t.Fatalf("kv did not persist/upsert: %q ok=%v", v, ok)
	}
}

func TestRetentionSweepDeletesOldRows(t *testing.T) {
	s, _ := openTest(t)
	old := eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
		RequestID: "req_old", SelectedModel: "cheap-general", DecidedAt: time.Now().AddDate(0, 0, -40),
	}}
	s.Handle(context.Background(), old)
	s.Handle(context.Background(), decision("req_new", "simple_chat", "cheap-general"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.StartRetention(ctx, 30, 10*time.Millisecond, nil)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(s.Recent(10)) == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	rec := s.Recent(10)
	if len(rec) != 1 || rec[0].RequestID != "req_new" {
		t.Fatalf("retention should delete only the old row, got %+v", rec)
	}
}
