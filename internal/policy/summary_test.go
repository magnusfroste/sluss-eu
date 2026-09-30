package policy

import (
	"strings"
	"testing"
)

// ISSUE-116: the console shows the active policy in plain words.
func TestSummariesReadLikeFirewallRules(t *testing.T) {
	src := `
version: pv_sum
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: pii_local
    description: personal data stays home
    when:
      sensitivity: pii
    route:
      constraints:
        require_provider_tags: [local]
  - id: agents
    when:
      tool_risk: { in: [external, destructive] }
    route:
      block:
        code: agent_blocked
        reason: no
  - id: default
    when: {}
    route:
      defaults:
        model_profile: balanced
`
	compiled, err := Compile(mustParse(t, src), testSnapshot(t))
	if err != nil {
		t.Fatal(err)
	}
	s := compiled.Summaries()
	if len(s) != 3 {
		t.Fatalf("got %d summaries", len(s))
	}
	if s[0].When != "data is pii" || s[0].Then != "only models tagged local" || s[0].Kind != "require" || s[0].Description == "" {
		t.Fatalf("rule 1 = %+v", s[0])
	}
	if s[1].When != "agent can external or destructive" || s[1].Kind != "block" || !strings.Contains(s[1].Then, "agent_blocked") {
		t.Fatalf("rule 2 = %+v", s[1])
	}
	if s[2].When != "every request" || s[2].Kind != "default" {
		t.Fatalf("rule 3 = %+v", s[2])
	}
	var nilPolicy *CompiledPolicy
	if nilPolicy.Summaries() != nil {
		t.Fatal("nil policy should have no summaries")
	}
}
