package evals

import (
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/policy"
)

// tc builds a task-class frontier with the given models.
func tc(name string, models ...FrontierModel) TaskClassFrontier {
	return TaskClassFrontier{TaskClass: name, CaseCount: 10, Models: models}
}

func fm(id, tier string, passRate float64, samples int, costUSD float64) FrontierModel {
	return FrontierModel{
		ModelID:                      id,
		Tier:                         tier,
		EvalSamples:                  samples,
		EvalPassed:                   int(passRate*float64(samples) + 0.5),
		EvalPassRate:                 passRate,
		FrontierQuality:              passRate,
		AverageEstimatedCostUSD:      costUSD,
		AverageEstimatedCostMicroUSD: int64(costUSD * 1_000_000),
	}
}

func find(recs []PolicyRecommendation, task string) PolicyRecommendation {
	for _, r := range recs {
		if r.TaskClass == task {
			return r
		}
	}
	return PolicyRecommendation{}
}

// simple_code_edit floors at cheap; a cheap model that passes → keep.
// hard_code_debugging floors at balanced; a cheap model good enough → downgrade.
func TestRecommendDowngradeWhenCheapIsGoodEnough(t *testing.T) {
	report := FrontierReport{TaskClasses: []TaskClassFrontier{
		tc("hard_code_debugging",
			fm("cheap-general", "cheap", 0.95, 20, 0.001),
			fm("balanced-coder", "balanced", 0.96, 20, 0.010),
		),
	}}
	recs := RecommendPolicy(report, 0.9, 5)
	r := find(recs, "hard_code_debugging")
	if r.BaselineTier != "balanced" {
		t.Fatalf("baseline tier = %q, want balanced", r.BaselineTier)
	}
	if r.Kind != "downgrade" {
		t.Fatalf("kind = %q, want downgrade", r.Kind)
	}
	if r.RecommendedModel != "cheap-general" || r.RecommendedTier != "cheap" {
		t.Fatalf("recommended = %s/%s, want cheap-general/cheap", r.RecommendedModel, r.RecommendedTier)
	}
	if r.CostDeltaUSD >= 0 {
		t.Fatalf("cost delta = %v, want negative (a saving)", r.CostDeltaUSD)
	}
}

func TestRecommendKeepWhenFloorModelIsCheapestGoodEnough(t *testing.T) {
	// security_review floors at premium; only the premium model passes.
	report := FrontierReport{TaskClasses: []TaskClassFrontier{
		tc("security_review",
			fm("cheap-general", "cheap", 0.40, 20, 0.001),
			fm("premium-reasoning", "premium", 0.95, 20, 0.020),
		),
	}}
	r := find(RecommendPolicy(report, 0.9, 5), "security_review")
	if r.BaselineTier != "premium" {
		t.Fatalf("baseline = %q, want premium", r.BaselineTier)
	}
	if r.Kind != "keep" {
		t.Fatalf("kind = %q, want keep", r.Kind)
	}
}

func TestRecommendUpgradeWhenFloorTierFailsEvals(t *testing.T) {
	// summarization floors at cheap; the cheap model fails, only balanced passes.
	report := FrontierReport{TaskClasses: []TaskClassFrontier{
		tc("summarization",
			fm("cheap-general", "cheap", 0.50, 20, 0.001),
			fm("balanced-coder", "balanced", 0.93, 20, 0.010),
		),
	}}
	r := find(RecommendPolicy(report, 0.9, 5), "summarization")
	if r.BaselineTier != "cheap" {
		t.Fatalf("baseline = %q, want cheap", r.BaselineTier)
	}
	if r.Kind != "upgrade" {
		t.Fatalf("kind = %q, want upgrade", r.Kind)
	}
	if r.RecommendedTier != "balanced" {
		t.Fatalf("recommended tier = %q, want balanced", r.RecommendedTier)
	}
	if r.CostDeltaUSD <= 0 {
		t.Fatalf("cost delta = %v, want positive (upgrade costs more)", r.CostDeltaUSD)
	}
}

func TestRecommendInsufficientData(t *testing.T) {
	report := FrontierReport{TaskClasses: []TaskClassFrontier{
		tc("simple_code_edit",
			fm("cheap-general", "cheap", 1.0, 1, 0.001), // only 1 sample < minSamples
		),
	}}
	r := find(RecommendPolicy(report, 0.9, 5), "simple_code_edit")
	if r.Kind != "insufficient_data" {
		t.Fatalf("kind = %q, want insufficient_data", r.Kind)
	}
}

func TestRecommendNoModelMeetsTarget(t *testing.T) {
	report := FrontierReport{TaskClasses: []TaskClassFrontier{
		tc("hard_code_debugging",
			fm("cheap-general", "cheap", 0.60, 20, 0.001),
			fm("balanced-coder", "balanced", 0.70, 20, 0.010),
		),
	}}
	r := find(RecommendPolicy(report, 0.9, 5), "hard_code_debugging")
	if r.Kind != "no_model_meets_target" {
		t.Fatalf("kind = %q, want no_model_meets_target", r.Kind)
	}
}

func TestPolicyRecommendationYAMLEmitsValidRules(t *testing.T) {
	report := FrontierReport{TaskClasses: []TaskClassFrontier{
		tc("hard_code_debugging",
			fm("cheap-general", "cheap", 0.95, 20, 0.001),
			fm("balanced-coder", "balanced", 0.96, 20, 0.010),
		),
	}}
	recs := RecommendPolicy(report, 0.9, 5)
	yaml := PolicyRecommendationYAML(recs)
	for _, want := range []string{"rules:", "task_type: hard_code_debugging", "model_profile: cheap", "REVIEW before applying"} {
		if !strings.Contains(yaml, want) {
			t.Fatalf("yaml missing %q:\n%s", want, yaml)
		}
	}
}

func TestPolicyRecommendationYAMLValidatesAgainstCompiler(t *testing.T) {
	report := FrontierReport{TaskClasses: []TaskClassFrontier{
		tc("hard_code_debugging",
			fm("cheap-general", "cheap", 0.95, 20, 0.001),
			fm("balanced-coder", "balanced", 0.96, 20, 0.010),
		),
	}}
	snippet := PolicyRecommendationYAML(RecommendPolicy(report, 0.9, 5))
	// Wrap the generated rules in a minimal valid policy and compile it, proving
	// the emitted YAML is real policy an operator can paste in.
	full := "version: reco-test\n" +
		"settings:\n" +
		"  default_model_profile: balanced\n" +
		"  conservative_unknowns: true\n" +
		"  max_router_overhead_ms: 100\n" +
		"  default_timeout_ms: 30000\n" +
		"  default_retention: standard\n" +
		rulesSection(snippet)
	if _, err := policy.Parse([]byte(full)); err != nil {
		t.Fatalf("generated policy snippet failed to parse against the schema: %v\n%s", err, full)
	}
}

// rulesSection extracts the `rules:` block (and its rule lines) from a generated
// snippet, dropping leading comment lines so it can be embedded in a policy.
func rulesSection(snippet string) string {
	lines := strings.Split(snippet, "\n")
	var out []string
	inRules := false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "rules:") {
			inRules = true
		}
		if !inRules {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(ln), "#") && !strings.HasPrefix(ln, "  ") {
			continue // trailing top-level comment lines
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}
