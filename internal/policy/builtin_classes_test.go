package policy

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/policy/builtin"
	"github.com/magnusfroste/sluss/internal/registry"
)

// The shipped packs act on the richer sensitivity classes (ISSUE-093): the
// NIS2 baseline keeps health/financial/legal/security_classified in the house.
func TestNIS2BaselineRoutesRicherClassesLocal(t *testing.T) {
	src, ok := builtin.Load("nis2-baseline")
	if !ok {
		t.Fatal("builtin nis2-baseline should exist")
	}
	pol, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	compiled, err := Compile(pol, snap)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	for _, sensitivity := range []string{"security_classified", "health", "financial", "legal"} {
		ev := compiled.Evaluate(EvaluationInput{TaskType: "summarization", RiskLevel: "medium", Sensitivity: sensitivity})
		if ev.Route.Constraints == nil {
			t.Fatalf("%s: no constraints; matched=%v", sensitivity, ev.MatchedRuleIDs)
		}
		var local bool
		for _, tag := range ev.Route.Constraints.RequireProviderTags {
			if tag == "local" {
				local = true
			}
		}
		if !local {
			t.Fatalf("%s should require the local tag, got %+v (matched=%v)", sensitivity, ev.Route.Constraints, ev.MatchedRuleIDs)
		}
	}

	// A non-sensitive prompt keeps routing free (cloud allowed).
	ev := compiled.Evaluate(EvaluationInput{TaskType: "summarization", RiskLevel: "low", Sensitivity: "none"})
	if ev.Route.Constraints != nil && len(ev.Route.Constraints.RequireProviderTags) > 0 {
		t.Fatalf("non-sensitive prompt should not be tag-constrained: %+v", ev.Route.Constraints)
	}
}

// Rule 4d (ISSUE-111): an agent declaring external/destructive capability is
// confined to local-tagged providers even when the prompt content is harmless.
func TestNIS2BaselineConfinesHighCapabilityAgents(t *testing.T) {
	src, ok := builtin.Load("nis2-baseline")
	if !ok {
		t.Fatal("builtin nis2-baseline should exist")
	}
	pol, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	compiled, err := Compile(pol, snap)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	for _, risk := range []string{"external", "destructive"} {
		ev := compiled.Evaluate(EvaluationInput{
			TaskType: "simple_chat", RiskLevel: "low", Sensitivity: "none",
			ToolRisk: risk, ToolsDeclared: []string{"send_email"},
		})
		var local bool
		if ev.Route.Constraints != nil {
			for _, tag := range ev.Route.Constraints.RequireProviderTags {
				if tag == "local" {
					local = true
				}
			}
		}
		if !local {
			t.Fatalf("tool_risk=%s should require local, got %+v (matched=%v)",
				risk, ev.Route.Constraints, ev.MatchedRuleIDs)
		}
	}

	// Read/write capability — and toolless prompts — keep routing free.
	for _, risk := range []string{"", "read", "write"} {
		ev := compiled.Evaluate(EvaluationInput{
			TaskType: "simple_chat", RiskLevel: "low", Sensitivity: "none", ToolRisk: risk,
		})
		if ev.Route.Constraints != nil && len(ev.Route.Constraints.RequireProviderTags) > 0 {
			t.Fatalf("tool_risk=%q should not be tag-constrained: %+v", risk, ev.Route.Constraints)
		}
	}
}
