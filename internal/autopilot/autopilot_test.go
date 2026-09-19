package autopilot

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/evals"
)

func rec(task, kind, tier string, pass float64, samples int, delta float64) evals.PolicyRecommendation {
	return evals.PolicyRecommendation{
		TaskClass:       task,
		Kind:            kind,
		RecommendedTier: tier,
		PassRate:        pass,
		EvalSamples:     samples,
		CostDeltaUSD:    delta,
	}
}

func decisionFor(res Result, task string) Decision {
	for _, d := range res.Decisions {
		if d.TaskClass == task {
			return d
		}
	}
	return Decision{}
}

func TestApprovesStrongDowngrade(t *testing.T) {
	res := Evaluate([]evals.PolicyRecommendation{
		rec("hard_code_debugging", "downgrade", "cheap", 0.98, 40, -0.01),
	}, DefaultGuardrails())
	if d := decisionFor(res, "hard_code_debugging"); d.Action != "apply" {
		t.Fatalf("action = %q (%s), want apply", d.Action, d.Reason)
	}
	if len(res.Approved) != 1 {
		t.Fatalf("approved = %d, want 1", len(res.Approved))
	}
}

func TestRejectsProtectedTask(t *testing.T) {
	res := Evaluate([]evals.PolicyRecommendation{
		rec("security_review", "downgrade", "balanced", 0.99, 100, -0.02),
	}, DefaultGuardrails())
	d := decisionFor(res, "security_review")
	if d.Action != "reject" {
		t.Fatalf("action = %q, want reject (protected)", d.Action)
	}
	if len(res.Approved) != 0 {
		t.Fatal("protected task must not be approved")
	}
}

func TestRejectsUnderSampledDowngrade(t *testing.T) {
	res := Evaluate([]evals.PolicyRecommendation{
		rec("summarization", "downgrade", "cheap", 0.99, 5, -0.01), // only 5 samples
	}, DefaultGuardrails())
	if decisionFor(res, "summarization").Action != "reject" {
		t.Fatal("under-sampled downgrade should be rejected")
	}
}

func TestRejectsLowPassRateDowngrade(t *testing.T) {
	res := Evaluate([]evals.PolicyRecommendation{
		rec("summarization", "downgrade", "cheap", 0.93, 50, -0.01), // below 0.95 floor
	}, DefaultGuardrails())
	if decisionFor(res, "summarization").Action != "reject" {
		t.Fatal("downgrade below the pass-rate floor should be rejected")
	}
}

func TestApprovesUpgradeWithoutQualityFloor(t *testing.T) {
	// Upgrades raise the floor (safety-positive); they only need enough samples,
	// not the downgrade pass-rate floor.
	res := Evaluate([]evals.PolicyRecommendation{
		rec("summarization", "upgrade", "balanced", 0.93, 30, 0.02),
	}, DefaultGuardrails())
	if decisionFor(res, "summarization").Action != "apply" {
		t.Fatal("well-sampled upgrade should be approved")
	}
}

func TestKeepIsSkipped(t *testing.T) {
	res := Evaluate([]evals.PolicyRecommendation{
		rec("simple_chat", "keep", "cheap", 1.0, 50, 0),
	}, DefaultGuardrails())
	if decisionFor(res, "simple_chat").Action != "skip" {
		t.Fatal("keep should be skipped, not applied/rejected")
	}
	if len(res.Approved) != 0 {
		t.Fatal("keep must not be approved")
	}
}

func TestBlastRadiusCapKeepsBiggestSavings(t *testing.T) {
	g := DefaultGuardrails()
	g.MaxChanges = 2
	res := Evaluate([]evals.PolicyRecommendation{
		rec("t_small", "downgrade", "cheap", 0.99, 50, -0.001),
		rec("t_big", "downgrade", "cheap", 0.99, 50, -0.050),
		rec("t_mid", "downgrade", "cheap", 0.99, 50, -0.010),
	}, g)
	if len(res.Approved) != 2 {
		t.Fatalf("approved = %d, want 2 (cap)", len(res.Approved))
	}
	// The two biggest savings (t_big, t_mid) should win; t_small is capped out.
	if decisionFor(res, "t_small").Action != "reject" {
		t.Fatalf("smallest-saving change should be capped out, got %q", decisionFor(res, "t_small").Action)
	}
	if decisionFor(res, "t_big").Action != "apply" || decisionFor(res, "t_mid").Action != "apply" {
		t.Fatal("the two biggest savings should be approved")
	}
}

func TestApprovedRenderToValidPolicy(t *testing.T) {
	res := Evaluate([]evals.PolicyRecommendation{
		rec("hard_code_debugging", "downgrade", "cheap", 0.98, 40, -0.01),
	}, DefaultGuardrails())
	yaml := evals.PolicyRecommendationYAML(res.Approved)
	if want := "task_type: hard_code_debugging"; !contains(yaml, want) {
		t.Fatalf("approved policy missing %q:\n%s", want, yaml)
	}
	if !contains(yaml, "model_profile: cheap") {
		t.Fatalf("approved policy missing forced profile:\n%s", yaml)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
