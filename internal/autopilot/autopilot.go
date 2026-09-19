// Package autopilot turns policy recommendations (from the eval frontier) into a
// guarded set of automatically-approved policy changes. It encodes the operator
// judgment as hard guardrails so the human is removed from the per-change
// decision loop but never from the safety envelope: only changes that clear
// every guardrail are approved, everything else is rejected with a reason, and
// the result is an auditable, ready-to-adopt policy artifact.
//
// Autopilot is an offline governance step — it never touches the request path.
// Adopting the produced policy is still an explicit operator action (copy +
// reload), so a bad change can never silently reach production.
package autopilot

import (
	"fmt"
	"sort"

	"github.com/magnusfroste/sluss/internal/evals"
)

// Guardrail defaults. Conservative on purpose: autopilot should only ever make
// changes it is very sure about.
const (
	DefaultMinSamples  = 20   // a change needs at least this many eval samples
	DefaultMinPassRate = 0.95 // absolute measured pass rate a downgrade target must clear
	DefaultMargin      = 0.05 // pass rate must beat the recommendation target by this much
	DefaultMaxChanges  = 3    // blast-radius cap: at most this many auto-applied changes
)

// DefaultProtectedTasks are task classes autopilot never auto-changes — the
// premium+verifier tier where a wrong downgrade is most expensive.
var DefaultProtectedTasks = map[string]bool{
	"security_review":    true,
	"database_migration": true,
	"unknown_high_risk":  true,
}

// Guardrails constrains which recommendations autopilot may auto-apply.
type Guardrails struct {
	MinSamples     int
	MinPassRate    float64
	Margin         float64
	MaxChanges     int
	AllowDowngrade bool
	AllowUpgrade   bool
	ProtectedTasks map[string]bool
}

// DefaultGuardrails returns a conservative, production-safe guardrail set:
// downgrades allowed only with strong evidence, upgrades allowed (they raise the
// floor), high-risk classes protected, blast radius capped.
func DefaultGuardrails() Guardrails {
	return Guardrails{
		MinSamples:     DefaultMinSamples,
		MinPassRate:    DefaultMinPassRate,
		Margin:         DefaultMargin,
		MaxChanges:     DefaultMaxChanges,
		AllowDowngrade: true,
		AllowUpgrade:   true,
		ProtectedTasks: DefaultProtectedTasks,
	}
}

// Decision is autopilot's verdict on one recommendation.
type Decision struct {
	TaskClass      string                     `json:"task_class"`
	Action         string                     `json:"action"` // apply | reject | skip
	Recommendation evals.PolicyRecommendation `json:"recommendation"`
	Reason         string                     `json:"reason"`
}

// Result is the outcome of an autopilot run.
type Result struct {
	Decisions []Decision                   `json:"decisions"`
	Approved  []evals.PolicyRecommendation `json:"approved"`
}

// Evaluate applies the guardrails to the recommendations and returns a decision
// per recommendation plus the approved subset (deterministically ordered). The
// input recommendations are not mutated.
func Evaluate(recs []evals.PolicyRecommendation, g Guardrails) Result {
	if g.MinSamples < 1 {
		g.MinSamples = 1
	}
	if g.MaxChanges < 0 {
		g.MaxChanges = 0
	}

	// Order recommendations deterministically, changes first, biggest cost win
	// first (largest saving for downgrades), so the MaxChanges cap keeps the most
	// valuable changes.
	ordered := append([]evals.PolicyRecommendation(nil), recs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		ci, cj := isChange(ordered[i].Kind), isChange(ordered[j].Kind)
		if ci != cj {
			return ci // changes before non-changes
		}
		if ordered[i].CostDeltaUSD != ordered[j].CostDeltaUSD {
			return ordered[i].CostDeltaUSD < ordered[j].CostDeltaUSD // more negative (bigger saving) first
		}
		return ordered[i].TaskClass < ordered[j].TaskClass
	})

	res := Result{}
	applied := 0
	for _, rec := range ordered {
		d := Decision{TaskClass: rec.TaskClass, Recommendation: rec}

		if !isChange(rec.Kind) {
			d.Action = "skip"
			d.Reason = "no change recommended (" + rec.Kind + ")"
			res.Decisions = append(res.Decisions, d)
			continue
		}
		if g.ProtectedTasks[rec.TaskClass] {
			d.Action = "reject"
			d.Reason = "protected task class — never auto-changed"
			res.Decisions = append(res.Decisions, d)
			continue
		}
		if reason, ok := guardrailReject(rec, g); !ok {
			d.Action = "reject"
			d.Reason = reason
			res.Decisions = append(res.Decisions, d)
			continue
		}
		if applied >= g.MaxChanges {
			d.Action = "reject"
			d.Reason = fmt.Sprintf("blast-radius cap reached (%d change(s) max per run)", g.MaxChanges)
			res.Decisions = append(res.Decisions, d)
			continue
		}
		d.Action = "apply"
		d.Reason = "clears all guardrails"
		res.Decisions = append(res.Decisions, d)
		res.Approved = append(res.Approved, rec)
		applied++
	}
	return res
}

// guardrailReject returns ("", true) when the recommendation clears the
// guardrails, or (reason, false) when it is blocked.
func guardrailReject(rec evals.PolicyRecommendation, g Guardrails) (string, bool) {
	switch rec.Kind {
	case "downgrade":
		if !g.AllowDowngrade {
			return "downgrades disabled", false
		}
		if rec.EvalSamples < g.MinSamples {
			return fmt.Sprintf("only %d eval samples (need >= %d)", rec.EvalSamples, g.MinSamples), false
		}
		if rec.PassRate < g.MinPassRate {
			return fmt.Sprintf("pass rate %.2f below the %.2f floor for a downgrade", rec.PassRate, g.MinPassRate), false
		}
		if rec.PassRate < evals.DefaultTargetPassRate+g.Margin {
			return fmt.Sprintf("pass rate %.2f lacks the %.2f margin above the target", rec.PassRate, g.Margin), false
		}
		return "", true
	case "upgrade":
		if !g.AllowUpgrade {
			return "upgrades disabled", false
		}
		if rec.EvalSamples < g.MinSamples {
			return fmt.Sprintf("only %d eval samples (need >= %d)", rec.EvalSamples, g.MinSamples), false
		}
		return "", true
	default:
		return "not an actionable change", false
	}
}

func isChange(kind string) bool {
	return kind == "downgrade" || kind == "upgrade"
}

// FormatDecisions renders the per-recommendation decisions as a readable log.
func FormatDecisions(res Result) string {
	out := "\nAutopilot decisions (guardrail-gated):\n"
	if len(res.Decisions) == 0 {
		return out + "  No recommendations to evaluate.\n"
	}
	for _, d := range res.Decisions {
		out += fmt.Sprintf("  %-22s %-7s %s\n", d.TaskClass, upper(d.Action), d.Reason)
	}
	out += fmt.Sprintf("  -> %d change(s) approved for adoption.\n", len(res.Approved))
	return out
}

func upper(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}
