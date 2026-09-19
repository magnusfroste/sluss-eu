package health

import (
	"testing"
	"time"
)

// clock is a controllable time source for circuit-breaker tests.
type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestTracker(threshold int, cooldown time.Duration) (*Tracker, *clock) {
	c := &clock{t: time.Unix(1_700_000_000, 0)}
	tr := NewWithConfig(threshold, cooldown)
	tr.now = c.now
	return tr, c
}

func TestCircuitOpensAfterConsecutiveFailures(t *testing.T) {
	tr, _ := newTestTracker(3, 30*time.Second)

	// Two failures is below threshold → still eligible (score, not excluded).
	tr.RecordFailure("p")
	tr.RecordFailure("p")
	if tr.CircuitOpen("p") {
		t.Fatal("circuit opened before reaching threshold")
	}
	if h := tr.ProviderHealth("p"); h == 0.0 {
		t.Fatalf("provider excluded before trip, health=%v", h)
	}

	// Third consecutive failure trips the circuit → health 0.0 (excluded).
	tr.RecordFailure("p")
	if !tr.CircuitOpen("p") {
		t.Fatal("circuit did not open at threshold")
	}
	if h := tr.ProviderHealth("p"); h != 0.0 {
		t.Fatalf("open circuit health = %v, want 0.0", h)
	}
}

func TestCircuitHalfOpensAfterCooldown(t *testing.T) {
	tr, clk := newTestTracker(3, 30*time.Second)
	for i := 0; i < 3; i++ {
		tr.RecordFailure("p")
	}
	if h := tr.ProviderHealth("p"); h != 0.0 {
		t.Fatalf("circuit should be open, health=%v", h)
	}

	// Still within cooldown → stays open.
	clk.advance(29 * time.Second)
	if h := tr.ProviderHealth("p"); h != 0.0 {
		t.Fatalf("circuit should still be open at 29s, health=%v", h)
	}

	// After cooldown → half-open, provider eligible again for a probe.
	clk.advance(2 * time.Second)
	if tr.CircuitOpen("p") {
		t.Fatal("circuit should have half-opened after cooldown")
	}
	if h := tr.ProviderHealth("p"); h == 0.0 {
		t.Fatal("half-open provider should be eligible (non-zero health)")
	}
}

func TestHalfOpenProbeSuccessClosesCircuit(t *testing.T) {
	tr, clk := newTestTracker(3, 30*time.Second)
	for i := 0; i < 3; i++ {
		tr.RecordFailure("p")
	}
	clk.advance(31 * time.Second) // half-open

	tr.RecordSuccess("p") // probe succeeds → circuit closes, streak resets
	if tr.CircuitOpen("p") {
		t.Fatal("successful probe should close the circuit")
	}
	// One more failure must not immediately re-trip (streak was reset).
	tr.RecordFailure("p")
	if tr.CircuitOpen("p") {
		t.Fatal("single failure after reset should not re-open the circuit")
	}
}

func TestHalfOpenProbeFailureReopensCircuit(t *testing.T) {
	tr, clk := newTestTracker(3, 30*time.Second)
	for i := 0; i < 3; i++ {
		tr.RecordFailure("p")
	}
	clk.advance(31 * time.Second) // half-open
	if tr.CircuitOpen("p") {
		t.Fatal("should be half-open after cooldown")
	}

	// Probe fails → re-open for a fresh cooldown.
	tr.RecordFailure("p")
	if !tr.CircuitOpen("p") {
		t.Fatal("failed probe should re-open the circuit")
	}
	if h := tr.ProviderHealth("p"); h != 0.0 {
		t.Fatalf("re-opened circuit health = %v, want 0.0", h)
	}
}

func TestSuccessResetsFailureStreak(t *testing.T) {
	tr, _ := newTestTracker(3, 30*time.Second)
	tr.RecordFailure("p")
	tr.RecordFailure("p")
	tr.RecordSuccess("p") // resets streak
	tr.RecordFailure("p")
	tr.RecordFailure("p")
	if tr.CircuitOpen("p") {
		t.Fatal("interleaved success should have reset the streak, circuit must stay closed")
	}
}

func TestCircuitBreakingDisabled(t *testing.T) {
	tr, _ := newTestTracker(0, 30*time.Second) // threshold 0 disables breaking
	for i := 0; i < 20; i++ {
		tr.RecordFailure("p")
	}
	if tr.CircuitOpen("p") {
		t.Fatal("threshold 0 must disable circuit breaking")
	}
	// Score still reflects the failures via the rolling window.
	if h := tr.ProviderHealth("p"); h != 0.0 && h >= 1.0 {
		t.Fatalf("rolling-window score should reflect failures, got %v", h)
	}
}

func TestProvidersReflectsOpenCircuit(t *testing.T) {
	tr, _ := newTestTracker(3, 30*time.Second)
	for i := 0; i < 3; i++ {
		tr.RecordFailure("p")
	}
	if got := tr.Providers()["p"]; got != 0.0 {
		t.Fatalf("Providers() for open circuit = %v, want 0.0", got)
	}
}
