package eventlog

import (
	"context"
	"testing"
	"time"
)

func attemptEvent(reqID, code string, success bool) Event {
	return Event{Type: EventTypeAttempt, Attempt: &AttemptEvent{
		RequestID: reqID, ProviderID: "dgx", ModelID: "qwen36-27b",
		Success: success, ErrorCode: code, AttemptedAt: time.Unix(1_700_000_000, 0),
	}}
}

func TestErrorRingRecordsOnlyFailures(t *testing.T) {
	r := NewErrorRing(10)
	r.Handle(context.Background(), attemptEvent("ok1", "", true))  // success → ignored
	r.Handle(context.Background(), attemptEvent("dec", "", false)) // no code → ignored
	r.Handle(context.Background(), attemptEvent("bad1", "provider_5xx", false))
	r.Handle(context.Background(), attemptEvent("bad2", "provider_timeout", false))
	// A decision event must be ignored.
	r.Handle(context.Background(), Event{Type: EventTypeDecision})

	got := r.Recent(0)
	if len(got) != 2 {
		t.Fatalf("want 2 failures, got %d (%+v)", len(got), got)
	}
	// Newest first.
	if got[0].RequestID != "bad2" || got[1].RequestID != "bad1" {
		t.Fatalf("order wrong (want newest first): %+v", got)
	}
	if got[0].ErrorCode != "provider_timeout" || got[0].ProviderID != "dgx" {
		t.Fatalf("fields wrong: %+v", got[0])
	}
}

func TestErrorRingWrapsAndBoundsN(t *testing.T) {
	r := NewErrorRing(3)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		r.Handle(context.Background(), attemptEvent(id, "provider_5xx", false))
	}
	got := r.Recent(0)
	if len(got) != 3 {
		t.Fatalf("ring size 3 should hold 3, got %d", len(got))
	}
	// Most recent three, newest first: e, d, c.
	want := []string{"e", "d", "c"}
	for i, w := range want {
		if got[i].RequestID != w {
			t.Fatalf("at %d want %s, got %s (%+v)", i, w, got[i].RequestID, got)
		}
	}
	// n caps the result.
	if two := r.Recent(2); len(two) != 2 || two[0].RequestID != "e" {
		t.Fatalf("Recent(2) wrong: %+v", two)
	}
}

func TestErrorRingNilSafe(t *testing.T) {
	var r *ErrorRing
	if got := r.Recent(5); got != nil {
		t.Fatalf("nil ring should return nil, got %+v", got)
	}
}
