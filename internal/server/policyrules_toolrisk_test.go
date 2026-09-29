package server

import (
	"strings"
	"testing"
)

// ISSUE-111 in the console: a rule may gate on the capability an agent
// declares, so "delete-capable agents stay in the house" is one click — no
// YAML, no restart, no pack switch.
func TestConsoleRuleToolRiskRoundTrip(t *testing.T) {
	r := ConsoleRule{ToolRisks: []string{"destructive", "external"}, Action: "require_tags", Tags: []string{"local"}}
	if err := r.Validate(); err != nil {
		t.Fatalf("tool_risk-only rule should be valid: %v", err)
	}
	if !strings.Contains(r.WhenSummary(), "agent capability: destructive, external") {
		t.Fatalf("summary should name the capability: %q", r.WhenSummary())
	}
	yaml := GenerateConsolePolicyYAML("pv_test", []ConsoleRule{r})
	if !strings.Contains(yaml, "tool_risk: { in: [destructive, external] }") {
		t.Fatalf("YAML should carry tool_risk condition:\n%s", yaml)
	}

	bad := ConsoleRule{ToolRisks: []string{"destrucive"}, Action: "block"}
	if err := bad.Validate(); err == nil {
		t.Fatal("unknown capability class must be rejected")
	}
}
