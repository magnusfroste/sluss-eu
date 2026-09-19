package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestRecordNilSinkIsNoop(t *testing.T) {
	// Must not panic on a nil sink.
	Record(context.Background(), nil, Entry{Action: ActionPolicyReload})
}

func TestRecordDefaultsTimeAndOutcome(t *testing.T) {
	mem := NewMemorySink(0)
	Record(context.Background(), mem, Entry{Action: ActionAPIKeyAdd, Target: "key_1"})

	entries := mem.Entries()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Time.IsZero() {
		t.Error("Time should be defaulted to now")
	}
	if e.Outcome != OutcomeSuccess {
		t.Errorf("Outcome = %q, want %q", e.Outcome, OutcomeSuccess)
	}
	if e.Action != ActionAPIKeyAdd || e.Target != "key_1" {
		t.Errorf("entry not preserved: %+v", e)
	}
}

func TestRecordPreservesExplicitOutcome(t *testing.T) {
	mem := NewMemorySink(0)
	Record(context.Background(), mem, Entry{Action: ActionRequestBlocked, Outcome: OutcomeBlocked})
	if got := mem.Entries()[0].Outcome; got != OutcomeBlocked {
		t.Errorf("Outcome = %q, want %q", got, OutcomeBlocked)
	}
}

func TestMemorySinkRingBufferBounds(t *testing.T) {
	mem := NewMemorySink(3)
	for i := 0; i < 5; i++ {
		Record(context.Background(), mem, Entry{Action: ActionPolicyReload, Target: string(rune('a' + i))})
	}
	entries := mem.Entries()
	if len(entries) != 3 {
		t.Fatalf("want 3 retained, got %d", len(entries))
	}
	// Oldest two ('a','b') dropped; should retain c,d,e in order.
	got := entries[0].Target + entries[1].Target + entries[2].Target
	if got != "cde" {
		t.Errorf("retained targets = %q, want %q", got, "cde")
	}
}

func TestMemorySinkEntriesIsCopy(t *testing.T) {
	mem := NewMemorySink(0)
	Record(context.Background(), mem, Entry{Action: ActionPolicyReload, Target: "v1"})
	got := mem.Entries()
	got[0].Target = "mutated"
	if mem.Entries()[0].Target != "v1" {
		t.Error("Entries() must return a copy, not the backing slice")
	}
}

func TestMultiSinkFansOutAndSkipsNil(t *testing.T) {
	a := NewMemorySink(0)
	b := NewMemorySink(0)
	sink := MultiSink(a, nil, b)
	Record(context.Background(), sink, Entry{Action: ActionAPIKeyDisable, Target: "key_x"})
	if len(a.Entries()) != 1 || len(b.Entries()) != 1 {
		t.Fatalf("both sinks should receive the entry: a=%d b=%d", len(a.Entries()), len(b.Entries()))
	}
}

func TestLogSinkEmitsDeterministicAuditLine(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	sink := &LogSink{Logger: logger}

	Record(context.Background(), sink, Entry{
		Action:   ActionRequestBlocked,
		Actor:    "tn_1",
		TenantID: "tn_1",
		Target:   "gpt-4o",
		Outcome:  OutcomeBlocked,
		Reason:   "blocked by policy",
		Detail:   map[string]string{"block_code": "model_not_allowed", "task_type": "security_review"},
	})

	var line map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, buf.String())
	}
	if line["msg"] != "audit" {
		t.Errorf("msg = %v, want audit", line["msg"])
	}
	if line["audit_action"] != string(ActionRequestBlocked) {
		t.Errorf("audit_action = %v", line["audit_action"])
	}
	if line["detail_block_code"] != "model_not_allowed" {
		t.Errorf("detail_block_code = %v", line["detail_block_code"])
	}
	if !strings.Contains(buf.String(), "security_review") {
		t.Error("expected task_type detail in log line")
	}
}

func TestMemorySinkConcurrentRecord(t *testing.T) {
	mem := NewMemorySink(0)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Record(context.Background(), mem, Entry{Action: ActionPolicyReload})
		}()
	}
	wg.Wait()
	if len(mem.Entries()) != 50 {
		t.Errorf("want 50 entries, got %d", len(mem.Entries()))
	}
}
