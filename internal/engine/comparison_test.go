package engine_test

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
)

func TestCompareDecisionsFlagsDeterministicDifferences(t *testing.T) {
	primary := engine.RouteDecision{
		SelectedModel:    "balanced-coder",
		SelectedProvider: "openai",
		ProviderModelID:  "gpt-4.1",
		Fallbacks: []engine.FallbackEntry{
			{ModelID: "cheap-general", ProviderID: "openai", ProviderModelID: "gpt-4.1-mini"},
		},
		TimeoutMS:        30000,
		RequiresVerifier: false,
		PolicyVersion:    "policy-a",
		EstimatedCostUSD: 0.012345,
		DecisionReasons:  []string{"primary reason"},
	}
	secondary := engine.RouteDecision{
		SelectedModel:    "premium-reasoning",
		SelectedProvider: "anthropic",
		ProviderModelID:  "claude-sonnet-4",
		Fallbacks: []engine.FallbackEntry{
			{ModelID: "balanced-coder", ProviderID: "openai", ProviderModelID: "gpt-4.1"},
		},
		TimeoutMS:        45000,
		RequiresVerifier: true,
		PolicyVersion:    "policy-b",
		EstimatedCostUSD: 0.045678,
		DecisionReasons:  []string{"secondary reason"},
	}

	comparison := engine.CompareDecisions(primary, secondary)
	if !comparison.Changed {
		t.Fatalf("expected comparison to report a change")
	}
	if !comparison.RouteChanged {
		t.Fatalf("expected route change")
	}
	if !comparison.FallbackChanged {
		t.Fatalf("expected fallback change")
	}
	if !comparison.TimeoutChanged {
		t.Fatalf("expected timeout change")
	}
	if !comparison.RequiresVerifierChanged {
		t.Fatalf("expected verifier change")
	}
	if !comparison.PolicyVersionChanged {
		t.Fatalf("expected policy version change")
	}
	if !comparison.CostChanged {
		t.Fatalf("expected cost change")
	}
	if comparison.EstimatedCostDeltaMicroUSD != 33333 {
		t.Fatalf("delta micro USD = %d, want 33333", comparison.EstimatedCostDeltaMicroUSD)
	}
}

func TestCompareDecisionsIgnoresReasonOnlyChanges(t *testing.T) {
	primary := engine.RouteDecision{
		SelectedModel:    "balanced-coder",
		SelectedProvider: "openai",
		ProviderModelID:  "gpt-4.1",
		TimeoutMS:        30000,
		PolicyVersion:    "same-policy",
		EstimatedCostUSD: 0.010000,
		DecisionReasons:  []string{"old reason"},
	}
	secondary := primary
	secondary.DecisionReasons = []string{"new reason"}

	comparison := engine.CompareDecisions(primary, secondary)
	if comparison.Changed {
		t.Fatalf("decision reasons alone must not count as a routing diff: %+v", comparison)
	}
}
