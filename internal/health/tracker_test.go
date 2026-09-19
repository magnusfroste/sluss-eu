package health_test

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/health"
)

func TestTracker_OptimisticBeforeMinAttempts(t *testing.T) {
	tr := health.New()
	// No attempts recorded — should be 1.0.
	if score := tr.ProviderHealth("openai"); score != 1.0 {
		t.Fatalf("expected 1.0 before any attempts, got %.2f", score)
	}
	// 4 attempts (< minAttempts=5) even with all failures → still 1.0.
	for i := 0; i < 4; i++ {
		tr.RecordFailure("openai")
	}
	if score := tr.ProviderHealth("openai"); score != 1.0 {
		t.Fatalf("expected 1.0 before min attempts, got %.2f", score)
	}
}

func TestTracker_AllSuccesses(t *testing.T) {
	tr := health.New()
	for i := 0; i < 20; i++ {
		tr.RecordSuccess("openai")
	}
	if score := tr.ProviderHealth("openai"); score != 1.0 {
		t.Fatalf("expected 1.0 for all-success window, got %.2f", score)
	}
}

func TestTracker_AllFailures(t *testing.T) {
	tr := health.New()
	for i := 0; i < 20; i++ {
		tr.RecordFailure("anthropic")
	}
	score := tr.ProviderHealth("anthropic")
	if score != 0.0 {
		t.Fatalf("expected 0.0 for all-failure window, got %.2f", score)
	}
}

func TestTracker_HalfFailures(t *testing.T) {
	tr := health.New()
	for i := 0; i < 10; i++ {
		tr.RecordSuccess("prov")
		tr.RecordFailure("prov")
	}
	score := tr.ProviderHealth("prov")
	if score < 0.45 || score > 0.55 {
		t.Fatalf("expected ~0.5 for 50%% failure rate, got %.2f", score)
	}
}

func TestTracker_UnknownProviderIsHealthy(t *testing.T) {
	tr := health.New()
	if score := tr.ProviderHealth("unknown"); score != 1.0 {
		t.Fatalf("expected 1.0 for unknown provider, got %.2f", score)
	}
}

func TestTracker_RollingWindowEvictsOldEntries(t *testing.T) {
	tr := health.New()
	// Fill with failures (> minAttempts to activate scoring).
	for i := 0; i < 100; i++ {
		tr.RecordFailure("p")
	}
	low := tr.ProviderHealth("p")
	if low >= 0.1 {
		t.Fatalf("expected near-zero after 100 failures, got %.2f", low)
	}
	// Now fill entirely with successes — old failures should be evicted.
	for i := 0; i < 100; i++ {
		tr.RecordSuccess("p")
	}
	high := tr.ProviderHealth("p")
	if high < 0.99 {
		t.Fatalf("expected near-1.0 after 100 successes evicting old failures, got %.2f", high)
	}
}

func TestTracker_ConcurrentSafe(t *testing.T) {
	tr := health.New()
	done := make(chan struct{})
	for g := 0; g < 10; g++ {
		go func(id string) {
			for i := 0; i < 200; i++ {
				if i%3 == 0 {
					tr.RecordFailure(id)
				} else {
					tr.RecordSuccess(id)
				}
				_ = tr.ProviderHealth(id)
			}
			done <- struct{}{}
		}([]string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10"}[g])
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
