package experiment

import (
	"fmt"
	"math"
	"testing"
)

func TestDisabledIsAllControl(t *testing.T) {
	c := Config{Percentage: 0}
	if c.Enabled() {
		t.Fatal("percentage 0 should be disabled")
	}
	for i := 0; i < 100; i++ {
		if c.Assign(fmt.Sprintf("k%d", i)) != ArmControl {
			t.Fatal("disabled experiment must return control for every key")
		}
	}
}

func TestFullRolloutIsAllTreatment(t *testing.T) {
	c := Config{Percentage: 100}
	for i := 0; i < 100; i++ {
		if c.Assign(fmt.Sprintf("k%d", i)) != ArmTreatment {
			t.Fatal("100%% experiment must return treatment for every key")
		}
	}
}

func TestEmptyKeyIsControl(t *testing.T) {
	if (Config{Percentage: 100}).Assign("") != ArmControl {
		t.Fatal("empty key must never be bucketed into treatment")
	}
}

func TestAssignmentIsDeterministic(t *testing.T) {
	c := Config{Percentage: 50, Salt: "exp1"}
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("tenant-%d", i)
		first := c.Assign(key)
		for j := 0; j < 5; j++ {
			if c.Assign(key) != first {
				t.Fatalf("assignment for %q not stable", key)
			}
		}
	}
}

func TestPercentageIsApproximatelyHonored(t *testing.T) {
	c := Config{Percentage: 30, Salt: "exp1"}
	const n = 10000
	treatment := 0
	for i := 0; i < n; i++ {
		if c.Assign(fmt.Sprintf("key-%d", i)) == ArmTreatment {
			treatment++
		}
	}
	frac := float64(treatment) / float64(n)
	if math.Abs(frac-0.30) > 0.03 {
		t.Fatalf("treatment fraction = %.3f, want ~0.30 (±0.03)", frac)
	}
}

func TestSaltReshufflesAssignment(t *testing.T) {
	a := Config{Percentage: 50, Salt: "saltA"}
	b := Config{Percentage: 50, Salt: "saltB"}
	diff := 0
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("k%d", i)
		if a.Assign(key) != b.Assign(key) {
			diff++
		}
	}
	// Different salts should move a meaningful share of keys across arms.
	if diff < 200 {
		t.Fatalf("salt change moved only %d/1000 keys; expected a real reshuffle", diff)
	}
}
