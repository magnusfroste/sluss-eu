package evals

import (
	"fmt"
	"sort"
	"strings"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// DefaultTargetPassRate is the measured eval pass rate a model must reach for a
// task class to be considered "good enough" for a policy recommendation.
const DefaultTargetPassRate = 0.9

// PolicyRecommendation is a deterministic, report-only suggestion to change the
// routing floor for one task class, derived from the eval cost/quality frontier.
// It is never auto-applied — an operator reviews it and edits policy by hand.
type PolicyRecommendation struct {
	TaskClass          string  `json:"task_class"`
	Kind               string  `json:"kind"` // downgrade | upgrade | keep | no_model_meets_target | insufficient_data
	BaselineTier       string  `json:"baseline_tier"`
	RecommendedTier    string  `json:"recommended_tier,omitempty"`
	RecommendedModel   string  `json:"recommended_model,omitempty"`
	PassRate           float64 `json:"pass_rate,omitempty"`
	EvalSamples        int     `json:"eval_samples,omitempty"`
	BaselineCostUSD    float64 `json:"baseline_cost_usd,omitempty"`
	RecommendedCostUSD float64 `json:"recommended_cost_usd,omitempty"`
	CostDeltaUSD       float64 `json:"cost_delta_usd,omitempty"` // recommended - baseline (negative = saving)
	Reason             string  `json:"reason"`
}

// RecommendPolicy derives per-task-class routing-floor recommendations from a
// frontier report. targetPassRate (<=0 → DefaultTargetPassRate) is the measured
// pass rate a model must meet to count as good enough; minSamples (<1 → 1) is
// the minimum eval samples required before a model's pass rate is trusted.
//
// For each task class it finds the cheapest model that is good enough and
// compares that model's tier to the router's baseline floor for the task:
//   - cheaper tier  → downgrade (safe cost saving)
//   - same tier     → keep
//   - higher tier   → upgrade (the floor tier isn't measured good enough)
//
// Task classes with no sufficiently-sampled model report insufficient_data;
// those where nothing meets the target report no_model_meets_target.
func RecommendPolicy(report FrontierReport, targetPassRate float64, minSamples int) []PolicyRecommendation {
	if targetPassRate <= 0 {
		targetPassRate = DefaultTargetPassRate
	}
	if minSamples < 1 {
		minSamples = 1
	}

	recs := make([]PolicyRecommendation, 0, len(report.TaskClasses))
	for _, tc := range report.TaskClasses {
		recs = append(recs, recommendOne(tc, targetPassRate, minSamples))
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].TaskClass < recs[j].TaskClass })
	return recs
}

func recommendOne(tc TaskClassFrontier, target float64, minSamples int) PolicyRecommendation {
	baseline := engine.BaselineTier(router.TaskType(tc.TaskClass), router.RiskLow)
	rec := PolicyRecommendation{
		TaskClass:    tc.TaskClass,
		BaselineTier: string(baseline),
	}

	// Cheapest frontier model at or above the baseline floor — a fair proxy for
	// what the router pays today for this task class.
	if base, ok := cheapestAtOrAbove(tc.Models, baseline); ok {
		rec.BaselineCostUSD = base.AverageEstimatedCostUSD
	}

	sampled := 0
	var best *FrontierModel
	for i := range tc.Models {
		m := &tc.Models[i]
		if m.EvalSamples < minSamples {
			continue
		}
		sampled++
		if m.EvalPassRate < target {
			continue
		}
		if best == nil || cheaper(*m, *best) {
			best = m
		}
	}

	if sampled == 0 {
		rec.Kind = "insufficient_data"
		rec.Reason = fmt.Sprintf("no model has >= %d eval sample(s) for this task class", minSamples)
		return rec
	}
	if best == nil {
		rec.Kind = "no_model_meets_target"
		rec.Reason = fmt.Sprintf("no model reached the %.0f%% pass-rate target; keep the current floor and add coverage", target*100)
		return rec
	}

	rec.RecommendedTier = best.Tier
	rec.RecommendedModel = best.ModelID
	rec.PassRate = best.EvalPassRate
	rec.EvalSamples = best.EvalSamples
	rec.RecommendedCostUSD = best.AverageEstimatedCostUSD
	rec.CostDeltaUSD = roundTo(best.AverageEstimatedCostUSD-rec.BaselineCostUSD, 6)

	switch cmp := engine.TierOrdinal(registry.Tier(best.Tier)) - engine.TierOrdinal(baseline); {
	case cmp < 0:
		rec.Kind = "downgrade"
		rec.Reason = fmt.Sprintf(
			"%s passes %.0f%% of evals (%d samples) at tier %s — below the %s floor; downgrading saves ~$%.6f/req",
			best.ModelID, best.EvalPassRate*100, best.EvalSamples, best.Tier, baseline, -rec.CostDeltaUSD)
	case cmp == 0:
		rec.Kind = "keep"
		rec.Reason = fmt.Sprintf(
			"cheapest good-enough model %s is already at the %s floor (%.0f%% pass, %d samples)",
			best.ModelID, baseline, best.EvalPassRate*100, best.EvalSamples)
	default:
		rec.Kind = "upgrade"
		rec.Reason = fmt.Sprintf(
			"no model at/below the %s floor meets the target; %s at tier %s does (%.0f%% pass, %d samples) — costs ~$%.6f/req more",
			baseline, best.ModelID, best.Tier, best.EvalPassRate*100, best.EvalSamples, rec.CostDeltaUSD)
	}
	return rec
}

// cheapestAtOrAbove returns the cheapest model whose tier is at or above floor.
func cheapestAtOrAbove(models []FrontierModel, floor registry.Tier) (FrontierModel, bool) {
	var best *FrontierModel
	for i := range models {
		m := &models[i]
		if engine.TierOrdinal(registry.Tier(m.Tier)) < engine.TierOrdinal(floor) {
			continue
		}
		if best == nil || cheaper(*m, *best) {
			best = m
		}
	}
	if best == nil {
		return FrontierModel{}, false
	}
	return *best, true
}

// cheaper reports whether a is cheaper than b, breaking ties by higher quality
// then model id for determinism.
func cheaper(a, b FrontierModel) bool {
	if a.AverageEstimatedCostMicroUSD != b.AverageEstimatedCostMicroUSD {
		return a.AverageEstimatedCostMicroUSD < b.AverageEstimatedCostMicroUSD
	}
	if a.FrontierQuality != b.FrontierQuality {
		return a.FrontierQuality > b.FrontierQuality
	}
	return a.ModelID < b.ModelID
}

// FormatPolicyRecommendations renders the recommendations as a readable table.
func FormatPolicyRecommendations(recs []PolicyRecommendation) string {
	if len(recs) == 0 {
		return "\nPolicy recommendations:\n  No task-class data.\n"
	}
	var b strings.Builder
	b.WriteString("\nPolicy recommendations (review before applying — never auto-applied):\n")
	for _, r := range recs {
		fmt.Fprintf(&b, "  %s: %s\n", r.TaskClass, strings.ToUpper(r.Kind))
		fmt.Fprintf(&b, "    %s\n", r.Reason)
	}
	return b.String()
}

// PolicyRecommendationYAML emits a reviewable policy snippet that forces the
// recommended model profile for each task class where a change (downgrade or
// upgrade) is advised. keep / insufficient / no-target classes are listed as
// comments so the operator sees the full picture without silent gaps.
func PolicyRecommendationYAML(recs []PolicyRecommendation) string {
	var b strings.Builder
	b.WriteString("# Generated policy recommendations — REVIEW before applying.\n")
	b.WriteString("# Derived from measured eval pass rates on the cost/quality frontier.\n")
	b.WriteString("# These are suggestions, not an auto-applied policy. Merge the rules you\n")
	b.WriteString("# agree with into your policy file's `rules:` list.\n")

	var actionable []PolicyRecommendation
	for _, r := range recs {
		if r.Kind == "downgrade" || r.Kind == "upgrade" {
			actionable = append(actionable, r)
		}
	}
	if len(actionable) == 0 {
		b.WriteString("# No downgrade/upgrade suggested — current floors look right.\n")
	} else {
		b.WriteString("rules:\n")
		for _, r := range actionable {
			fmt.Fprintf(&b, "  - id: reco_%s\n", r.TaskClass)
			fmt.Fprintf(&b, "    # %s: %s\n", strings.ToUpper(r.Kind), r.Reason)
			fmt.Fprintf(&b, "    when:\n      task_type: %s\n", r.TaskClass)
			fmt.Fprintf(&b, "    route:\n      force:\n        model_profile: %s\n", r.RecommendedTier)
		}
	}

	// Note the non-actionable classes as comments for completeness.
	for _, r := range recs {
		if r.Kind != "downgrade" && r.Kind != "upgrade" {
			fmt.Fprintf(&b, "# %s: %s (%s)\n", r.TaskClass, r.Kind, r.Reason)
		}
	}
	return b.String()
}
