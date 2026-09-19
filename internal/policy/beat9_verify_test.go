package policy

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/classifier"
	"github.com/magnusfroste/sluss/internal/policy/builtin"
	"github.com/magnusfroste/sluss/internal/registry"
)

// Beat 9 in the CISO demo script, verified end-to-end: the exact demo payload's
// tools classify as destructive, and the NIS2 pack confines the request to
// local — while the same prompt without tools routes free.
func TestDemoBeat9AgentPayload(t *testing.T) {
	tools := []any{
		map[string]any{"type": "function", "function": map[string]any{
			"name": "search_accounts", "description": "Find accounts"}},
		map[string]any{"type": "function", "function": map[string]any{
			"name": "delete_account", "description": "Permanently remove an account"}},
	}
	names, risk := classifier.ClassifyTools(tools)
	if risk != classifier.ToolRiskDestructive {
		t.Fatalf("demo payload should classify destructive, got %q (%v)", risk, names)
	}

	src, _ := builtin.Load("nis2-baseline")
	pol, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	snap, _ := registry.DefaultSnapshot()
	compiled, err := Compile(pol, snap)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	with := compiled.Evaluate(EvaluationInput{
		TaskType: "simple_chat", RiskLevel: "low", Sensitivity: "none",
		ToolRisk: risk, ToolsDeclared: names,
	})
	if with.Route.Constraints == nil || len(with.Route.Constraints.RequireProviderTags) == 0 {
		t.Fatalf("beat 9 should confine to local, got %+v (matched=%v)", with.Route, with.MatchedRuleIDs)
	}
	without := compiled.Evaluate(EvaluationInput{
		TaskType: "simple_chat", RiskLevel: "low", Sensitivity: "none",
	})
	if without.Route.Constraints != nil && len(without.Route.Constraints.RequireProviderTags) > 0 {
		t.Fatalf("same prompt without tools should route free: %+v", without.Route.Constraints)
	}
}
