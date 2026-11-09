package policy

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/registry"
)

const maskPolicy = `version: pv_mask_test
metadata: {owner: t, description: t}
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: pii_mask
    when:
      sensitivity: pii
    route:
      mask: true
  - id: health_local
    when:
      sensitivity: health
    route:
      constraints:
        require_provider_tags: [local]
  - id: default
    when: {}
    route:
      defaults:
        model_profile: balanced
`

func TestMaskRouteParsesAndMerges(t *testing.T) {
	p, err := Parse([]byte(maskPolicy))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !p.Rules[0].Route.Mask {
		t.Fatal("mask: true must parse")
	}
	var r Route
	mergeRoute(&r, p.Rules[0].Route, "pii_mask")
	mergeRoute(&r, p.Rules[1].Route, "health_local")
	if !r.Mask || r.Constraints == nil || len(r.Constraints.RequireProviderTags) != 1 {
		t.Fatalf("mask must merge alongside a constraint, never replace it: %+v", r)
	}
	if _, err := Parse([]byte(`version: v
metadata: {owner: t, description: t}
settings: {default_model_profile: balanced, conservative_unknowns: true, max_router_overhead_ms: 100, default_timeout_ms: 30000, default_retention: standard}
rules:
  - id: x
    when: {sensitivity: pii}
    route: {mask: {}}
  - id: default
    when: {}
    route: {defaults: {model_profile: balanced}}
`)); err != nil {
		t.Fatalf("mask: {} must parse: %v", err)
	}
}

// The compiled policy must carry mask through to the evaluation: a lost flag
// would send personal data to the cloud unmasked.
func TestMaskSurvivesCompileAndEvaluate(t *testing.T) {
	p, err := Parse([]byte(maskPolicy))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	r := cloneRoute(p.Rules[0].Route)
	if !r.Mask {
		t.Fatal("cloneRoute must keep mask")
	}
}

func TestMaskThroughCompileAndEvaluate(t *testing.T) {
	p, err := Parse([]byte(maskPolicy))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	snap, err := registry.NewSnapshot(registry.DefaultDefinition())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	cp, err := Compile(p, snap)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if ev := cp.Evaluate(EvaluationInput{Sensitivity: "pii", TaskType: "simple_chat", RiskLevel: "medium"}); !ev.Route.Mask {
		t.Fatalf("a pii question must evaluate to mask: %+v", ev.Route)
	}
	if ev := cp.Evaluate(EvaluationInput{Sensitivity: "none", TaskType: "simple_chat", RiskLevel: "low"}); ev.Route.Mask {
		t.Fatal("a harmless question must not be masked")
	}
}
