package bandit

import (
	"math"
	"testing"

	"github.com/magnusfroste/sluss/internal/router"
)

const task = router.TaskType("hard_code_debugging")

func TestUnseenArmReturnsFalse(t *testing.T) {
	b := New(1.41)
	if _, ok := b.Quality(task, "never-tried"); ok {
		t.Fatal("an arm with no pulls must return false (fall back to prior)")
	}
}

func TestMeanRewardReflectsRecords(t *testing.T) {
	b := New(0.0001) // near-zero exploration → score ≈ mean
	for i := 0; i < 8; i++ {
		b.Record("t", "m", 1)
	}
	for i := 0; i < 2; i++ {
		b.Record("t", "m", 0)
	}
	s, ok := b.Quality(router.TaskType("t"), "m")
	if !ok {
		t.Fatal("recorded arm should return a score")
	}
	if math.Abs(s-0.8) > 0.01 {
		t.Fatalf("score = %.3f, want ~0.80 (8/10 success)", s)
	}
}

// A good but under-sampled arm should out-score a slightly-better, heavily
// pulled arm because of the exploration bonus — the bandit explores it.
func TestExplorationBonusFavorsUnderSampledArm(t *testing.T) {
	b := New(1.41)
	// Exploited arm: pulled a lot, moderate mean (kept below the clamp so the
	// exploration bonus is visible).
	for i := 0; i < 200; i++ {
		b.Record("t", "exploited", 0.5)
	}
	// Under-sampled arm: pulled a few times, similar mean.
	for i := 0; i < 3; i++ {
		b.Record("t", "fresh", 0.5)
	}
	exploited, _ := b.Quality(router.TaskType("t"), "exploited")
	fresh, _ := b.Quality(router.TaskType("t"), "fresh")
	if fresh <= exploited {
		t.Fatalf("under-sampled arm score %.3f should exceed exploited %.3f (exploration)", fresh, exploited)
	}
}

// As an arm is pulled more, its confidence bonus (and thus score, for a fixed
// mean) shrinks toward the mean — exploitation.
func TestBonusShrinksWithPulls(t *testing.T) {
	few := New(1.41)
	many := New(1.41)
	// Same mean (all reward 0.5), different pull counts. Add a second arm so
	// total task pulls > single-arm pulls (keeps the ln(total) term comparable).
	for i := 0; i < 5; i++ {
		few.Record("t", "m", 0.5)
		few.Record("t", "other", 0.5)
	}
	for i := 0; i < 500; i++ {
		many.Record("t", "m", 0.5)
		many.Record("t", "other", 0.5)
	}
	fewScore, _ := few.Quality(router.TaskType("t"), "m")
	manyScore, _ := many.Quality(router.TaskType("t"), "m")
	if fewScore <= manyScore {
		t.Fatalf("fewer pulls should give a larger bonus: few=%.3f many=%.3f", fewScore, manyScore)
	}
	if manyScore < 0.5 {
		t.Fatalf("well-pulled score %.3f should be >= mean 0.5", manyScore)
	}
}

func TestSeedStartsWarm(t *testing.T) {
	b := New(1.41)
	b.Seed("t", "m", 20, 18) // prior: 18/20 pass
	s, ok := b.Quality(router.TaskType("t"), "m")
	if !ok {
		t.Fatal("seeded arm should return a score")
	}
	if s < 0.9 { // mean 0.9 plus a bonus
		t.Fatalf("seeded score = %.3f, want >= 0.9", s)
	}
}

func TestRewardClampedAndDeterministic(t *testing.T) {
	a := New(1.41)
	c := New(1.41)
	for i := 0; i < 10; i++ {
		a.Record("t", "m", 5) // clamps to 1
		c.Record("t", "m", 1) // exactly 1
	}
	sa, _ := a.Quality(router.TaskType("t"), "m")
	sc, _ := c.Quality(router.TaskType("t"), "m")
	if sa != sc {
		t.Fatalf("clamped reward should equal reward 1: %.4f vs %.4f", sa, sc)
	}
}

func TestNilSafe(t *testing.T) {
	var b *Bandit
	if _, ok := b.Quality(task, "m"); ok {
		t.Fatal("nil bandit should return false")
	}
	b.Record("t", "m", 1) // must not panic
	if b.Arms() != 0 {
		t.Fatal("nil bandit Arms should be 0")
	}
}

func TestSnapshotSorted(t *testing.T) {
	b := New(1.41)
	b.Record("z_task", "m2", 1)
	b.Record("a_task", "m1", 0)
	snap := b.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("snapshot len = %d, want 2", len(snap))
	}
	if snap[0].TaskClass != "a_task" {
		t.Fatalf("snapshot not sorted by task: %+v", snap)
	}
}
