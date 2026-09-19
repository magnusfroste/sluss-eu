// Package health implements an in-memory rolling-window provider health tracker
// with a per-provider circuit breaker. It satisfies the engine.HealthSnapshot
// interface so the routing engine can read health scores without any I/O on the
// hot path.
//
// Two signals are combined:
//
//   - A rolling success-rate window ([0,1] score) that softly penalises a
//     degraded provider so scoring prefers healthier ones.
//   - A circuit breaker that hard-trips after a run of consecutive failures:
//     while the circuit is "open" the provider reports health 0.0, which is
//     below the engine's eligibility threshold, so it is excluded from routing
//     entirely (forcing fallback to a healthy provider). After a cooldown the
//     circuit half-opens — the provider becomes eligible for a single probe; a
//     success closes it, another failure re-opens it for a fresh cooldown.
package health

import (
	"sync"
	"time"
)

// windowSize is the number of recent attempts used for error-rate computation.
const windowSize = 100

// minAttempts is the minimum number of recorded attempts before the tracker
// starts reducing the score below 1.0. Fewer attempts → optimistic score.
const minAttempts = 5

// DefaultTripThreshold is the number of consecutive failures that opens a
// provider's circuit when no explicit threshold is configured.
const DefaultTripThreshold = 5

// DefaultCooldown is how long a tripped circuit stays open before half-opening.
const DefaultCooldown = 30 * time.Second

// Tracker is a concurrent, in-memory rolling-window health tracker with a
// per-provider circuit breaker. It records successes and failures per provider
// and exposes a [0, 1] health score used by the routing engine to penalise (or,
// when the circuit is open, exclude) degraded providers.
type Tracker struct {
	tripThreshold int           // consecutive failures to open a circuit (<=0 disables breaking)
	cooldown      time.Duration // how long a circuit stays open
	now           func() time.Time

	mu      sync.RWMutex
	windows map[string]*providerWindow
}

type providerWindow struct {
	buf   [windowSize]bool // true = success
	head  int
	total int // total attempts stored (capped at windowSize)

	consecutiveFailures int
	openUntil           time.Time // zero = circuit closed
}

// New returns a ready-to-use Tracker with the default circuit-breaker settings.
func New() *Tracker {
	return NewWithConfig(DefaultTripThreshold, DefaultCooldown)
}

// NewWithConfig returns a Tracker with an explicit circuit-breaker configuration.
// A tripThreshold <= 0 disables circuit breaking (pure rolling-window behaviour);
// a cooldown <= 0 falls back to DefaultCooldown.
func NewWithConfig(tripThreshold int, cooldown time.Duration) *Tracker {
	if cooldown <= 0 {
		cooldown = DefaultCooldown
	}
	return &Tracker{
		tripThreshold: tripThreshold,
		cooldown:      cooldown,
		now:           time.Now,
		windows:       make(map[string]*providerWindow),
	}
}

// RecordSuccess records a successful call to the given provider.
func (t *Tracker) RecordSuccess(providerID string) {
	t.record(providerID, true)
}

// RecordFailure records a failed call to the given provider.
func (t *Tracker) RecordFailure(providerID string) {
	t.record(providerID, false)
}

func (t *Tracker) record(providerID string, success bool) {
	t.mu.Lock()
	w, ok := t.windows[providerID]
	if !ok {
		w = &providerWindow{}
		t.windows[providerID] = w
	}
	w.buf[w.head%windowSize] = success
	w.head++
	if w.total < windowSize {
		w.total++
	}
	if success {
		// A success (including a half-open probe) closes the circuit.
		w.consecutiveFailures = 0
		w.openUntil = time.Time{}
	} else {
		w.consecutiveFailures++
		if t.tripThreshold > 0 && w.consecutiveFailures >= t.tripThreshold {
			w.openUntil = t.now().Add(t.cooldown)
		}
	}
	t.mu.Unlock()
}

// ProviderHealth returns a health score in [0.0, 1.0] for the given provider.
// While the circuit is open it returns 0.0 (excluded from routing). Otherwise,
// if fewer than minAttempts have been recorded the score is 1.0 (optimistic).
func (t *Tracker) ProviderHealth(providerID string) float64 {
	t.mu.RLock()
	w, ok := t.windows[providerID]
	if !ok {
		t.mu.RUnlock()
		return 1.0
	}
	if t.circuitOpen(w) {
		t.mu.RUnlock()
		return 0.0
	}
	if w.total < minAttempts {
		t.mu.RUnlock()
		return 1.0
	}
	n := w.total
	buf := w.buf
	t.mu.RUnlock()

	successes := 0
	for i := 0; i < n; i++ {
		if buf[i] {
			successes++
		}
	}
	return float64(successes) / float64(n)
}

// circuitOpen reports whether the provider's circuit is currently open (tripped
// and still within its cooldown). Caller must hold at least the read lock.
func (t *Tracker) circuitOpen(w *providerWindow) bool {
	return !w.openUntil.IsZero() && t.now().Before(w.openUntil)
}

// CircuitOpen reports whether the given provider's circuit is currently open.
// Intended for diagnostics and dashboards.
func (t *Tracker) CircuitOpen(providerID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	w, ok := t.windows[providerID]
	if !ok {
		return false
	}
	return t.circuitOpen(w)
}

// Providers returns a snapshot of all tracked provider IDs and their scores.
// A provider whose circuit is open reports 0.0.
func (t *Tracker) Providers() map[string]float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]float64, len(t.windows))
	for id, w := range t.windows {
		switch {
		case t.circuitOpen(w):
			out[id] = 0.0
		case w.total < minAttempts:
			out[id] = 1.0
		default:
			successes := 0
			for i := 0; i < w.total; i++ {
				if w.buf[i] {
					successes++
				}
			}
			out[id] = float64(successes) / float64(w.total)
		}
	}
	return out
}
