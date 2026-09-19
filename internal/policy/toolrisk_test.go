package policy

import "testing"

// ISSUE-111: policy must be able to gate on what an agent may INVOKE, not only
// on what the prompt contains — the "agent gateway" control the market now
// expects. Destructive capability stays in the house; external comms is
// blocked outright in this fixture.
const toolRiskPolicy = `
version: pv_tool_risk
metadata:
  owner: platform
  description: Agent capability gating
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: external_tools_blocked
    when:
      tool_risk: external
    route:
      block:
        code: tool_risk_external_denied
        reason: agent may send data outside the organisation
  - id: destructive_tools_stay_local
    when:
      tool_risk: destructive
    route:
      constraints:
        require_provider_tags: [local]
  - id: payment_tools_blocked
    when:
      any_tool_matches: ["*payment*", "transfer_funds"]
    route:
      block:
        code: tool_payment_denied
        reason: payment capability is not delegated to agents
`

func TestPolicyGatesOnToolRisk(t *testing.T) {
	compiled, err := Compile(mustParse(t, toolRiskPolicy), testSnapshot(t))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// Destructive capability → forced to a local provider, not blocked.
	dec := compiled.Evaluate(EvaluationInput{
		TaskType: "simple_chat", RiskLevel: "low", RouterMode: "auto",
		ToolRisk: "destructive", ToolsDeclared: []string{"delete_account"},
	})
	if dec.Blocked {
		t.Fatalf("destructive should route local, not block: %+v", dec)
	}
	if len(dec.Route.Constraints.RequireProviderTags) == 0 {
		t.Fatalf("expected local constraint, got %+v", dec.Route)
	}

	// External comms capability → fail-closed block.
	dec = compiled.Evaluate(EvaluationInput{
		TaskType: "simple_chat", RiskLevel: "low", RouterMode: "auto",
		ToolRisk: "external", ToolsDeclared: []string{"send_email"},
	})
	if !dec.Blocked {
		t.Fatal("external tool risk should be blocked by this policy")
	}

	// Name-level gating catches a specific tool regardless of its class.
	dec = compiled.Evaluate(EvaluationInput{
		TaskType: "simple_chat", RiskLevel: "low", RouterMode: "auto",
		ToolRisk: "write", ToolsDeclared: []string{"update_ledger", "transfer_funds"},
	})
	if !dec.Blocked {
		t.Fatalf("any_tool_matches should block transfer_funds: %+v", dec)
	}

	// A plain prompt with no tools is unaffected by all three rules.
	dec = compiled.Evaluate(EvaluationInput{
		TaskType: "simple_chat", RiskLevel: "low", RouterMode: "auto",
	})
	if dec.Blocked {
		t.Fatalf("toolless prompt must not be blocked: %+v", dec)
	}
}

// An unknown tool_risk value must be rejected at parse time, so a typo cannot
// silently disable a rule (the same contract as sensitivity).
func TestPolicyRejectsUnknownToolRisk(t *testing.T) {
	bad := `
version: pv_bad
metadata:
  owner: platform
  description: bad
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: typo
    when:
      tool_risk: destrucive
    route:
      constraints:
        require_provider_tags: [local]
`
	if _, err := Parse([]byte(bad)); err == nil {
		t.Fatal("expected parse error for unknown tool_risk value")
	}
}
