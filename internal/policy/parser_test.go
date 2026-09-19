package policy

import (
	"errors"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/registry"
)

const validPolicy = `
version: pv_2026_05_19
metadata:
  owner: platform
  description: Test
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: block_disabled
    when:
      router_mode: disabled
    route:
      block:
        code: router_disabled
        reason: disabled
  - id: trivial_git_cheap
    when:
      task_type: trivial_git
      risk_level: low
    route:
      defaults:
        model_profile: cheap
        max_cost_usd: 0.002
  - id: auth_premium
    when:
      any_file_matches:
        - "**/auth/**"
      risk_level:
        in: [high, critical]
    route:
      force:
        model_profile_name: premium-reasoning
        verifier: true
      constraints:
        require_capabilities: [tool_use, json_schema]
  - id: default_balanced
    when: {}
    route:
      defaults:
        model_profile: balanced
`

func mustParse(t *testing.T, src string) *Policy {
	t.Helper()
	p, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return p
}

func TestParseValidPolicy(t *testing.T) {
	p := mustParse(t, validPolicy)
	if p.Version != "pv_2026_05_19" {
		t.Errorf("version = %q", p.Version)
	}
	if p.Settings.DefaultModelProfile != ProfileBalanced {
		t.Errorf("default profile = %q", p.Settings.DefaultModelProfile)
	}
	if !p.Settings.ConservativeUnknowns {
		t.Errorf("conservative_unknowns should be true")
	}
	if p.Settings.MaxRouterOverheadMS != 100 {
		t.Errorf("overhead = %d", p.Settings.MaxRouterOverheadMS)
	}
	if got := len(p.Rules); got != 4 {
		t.Fatalf("rules = %d, want 4", got)
	}

	block := p.Rules[0]
	if block.Route.Block == nil || block.Route.Block.Code != "router_disabled" {
		t.Errorf("block rule not parsed: %+v", block.Route.Block)
	}
	if block.When.RouterMode == nil || block.When.RouterMode.Values[0] != "disabled" {
		t.Errorf("router_mode not captured")
	}

	auth := p.Rules[2]
	if auth.Route.Force == nil || auth.Route.Force.ModelProfileName != "premium-reasoning" {
		t.Errorf("force model_profile_name not set")
	}
	if auth.Route.Force.Verifier == nil || !*auth.Route.Force.Verifier {
		t.Errorf("verifier flag not parsed")
	}
	if auth.When.RiskLevel == nil || len(auth.When.RiskLevel.Values) != 2 {
		t.Errorf("risk_level in-list not parsed: %+v", auth.When.RiskLevel)
	}
	if got := auth.Route.Constraints.RequireCapabilities; len(got) != 2 || got[0] != CapToolUse || got[1] != CapJSONSchema {
		t.Errorf("require_capabilities = %v", got)
	}

	last := p.Rules[3]
	if !last.When.Empty {
		t.Errorf("default rule when{} should be marked Empty")
	}
}

func TestParseForceContextPipeline(t *testing.T) {
	src := `
version: pv_context_pipeline
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: enable_context_pipeline
    when:
      tenant: tn_policy
    route:
      force:
        context_pipeline: true
`
	p := mustParse(t, src)
	if len(p.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(p.Rules))
	}
	force := p.Rules[0].Route.Force
	if force == nil || force.ContextPipeline == nil || !*force.ContextPipeline {
		t.Fatalf("context_pipeline not parsed into force: %+v", force)
	}
}

func wantErr(t *testing.T, _ *Policy, err error, contains string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", contains)
	}
	if !errors.Is(err, ErrInvalidPolicy) {
		t.Errorf("error not wrapping ErrInvalidPolicy: %v", err)
	}
	if !strings.Contains(err.Error(), contains) {
		t.Errorf("error %q does not contain %q", err.Error(), contains)
	}
}

func TestParseRejectsMissingVersion(t *testing.T) {
	src := strings.Replace(validPolicy, "version: pv_2026_05_19\n", "", 1)
	p, err := Parse([]byte(src))
	wantErr(t, p, err, "missing version")
}

func TestParseRejectsMissingSettings(t *testing.T) {
	src := `version: pv_2026_05_19
rules:
  - id: only
    when: {}
    route:
      defaults:
        model_profile: balanced
`
	p, err := Parse([]byte(src))
	wantErr(t, p, err, "missing settings")
}

func TestParseRejectsInvalidTaskType(t *testing.T) {
	src := strings.Replace(validPolicy, "task_type: trivial_git", "task_type: nonsense", 1)
	_, err := Parse([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "task_type") {
		t.Fatalf("expected task_type vocab error, got %v", err)
	}
}

func TestParseRejectsInvalidRiskLevel(t *testing.T) {
	src := strings.Replace(validPolicy, "in: [high, critical]", "in: [high, dangerous]", 1)
	_, err := Parse([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "risk_level") {
		t.Fatalf("expected risk_level vocab error, got %v", err)
	}
}

func TestParseRejectsInvalidRouterMode(t *testing.T) {
	src := strings.Replace(validPolicy, "router_mode: disabled", "router_mode: paused", 1)
	_, err := Parse([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "router_mode") {
		t.Fatalf("expected router_mode vocab error, got %v", err)
	}
}

func TestParseRejectsDuplicateRuleID(t *testing.T) {
	src := strings.Replace(validPolicy, "id: default_balanced", "id: trivial_git_cheap", 1)
	_, err := Parse([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "duplicate rule id") {
		t.Fatalf("expected duplicate id error, got %v", err)
	}
}

func TestParseRejectsRuleWithoutAction(t *testing.T) {
	src := `version: pv_2026_05_19
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: empty_route
    when: {}
    route: {}
`
	_, err := Parse([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "must specify block, force, constraints, defaults") {
		t.Fatalf("expected missing action error, got %v", err)
	}
}

func TestParseRejectsUnknownWhenField(t *testing.T) {
	src := strings.Replace(validPolicy, "task_type: trivial_git", "task_type: trivial_git\n      surprise: 1", 1)
	_, err := Parse([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown when field error, got %v", err)
	}
}

func TestParseRejectsInvalidModelProfile(t *testing.T) {
	src := strings.Replace(validPolicy, "model_profile: balanced", "model_profile: shiny", 1)
	_, err := Parse([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "model_profile") {
		t.Fatalf("expected model_profile error, got %v", err)
	}
}

func TestParseRejectsInvalidCapability(t *testing.T) {
	src := strings.Replace(validPolicy, "require_capabilities: [tool_use, json_schema]", "require_capabilities: [tool_use, telepathy]", 1)
	_, err := Parse([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "telepathy") {
		t.Fatalf("expected capability error, got %v", err)
	}
}

func TestParseRejectsUnsupportedFallbackModelsHint(t *testing.T) {
	src := strings.Replace(validPolicy,
		"defaults:\n        model_profile: balanced",
		"fallback_models: [balanced-coder]",
		1,
	)
	_, err := Parse([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "fallback_models") {
		t.Fatalf("expected fallback_models error, got %v", err)
	}
}

func TestParseRejectsEmptyDocument(t *testing.T) {
	_, err := Parse(nil)
	if err == nil {
		t.Fatalf("expected error for empty input")
	}
}

// --- Validate against registry ----------------------------------------------

func testSnapshot(t *testing.T) *registry.Snapshot {
	t.Helper()
	snap, err := registry.NewSnapshot(registry.DefaultDefinition())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return snap
}

func TestValidateAcceptsKnownReferences(t *testing.T) {
	p := mustParse(t, validPolicy)
	if err := Validate(p, testSnapshot(t)); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateRejectsUnknownProvider(t *testing.T) {
	src := strings.Replace(validPolicy,
		"force:\n        model_profile_name: premium-reasoning",
		"force:\n        provider: openrouter",
		1,
	)
	p := mustParse(t, src)
	err := Validate(p, testSnapshot(t))
	if err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("expected unknown provider error, got %v", err)
	}
}

func TestValidateRejectsUnknownModel(t *testing.T) {
	src := strings.Replace(validPolicy,
		"model_profile_name: premium-reasoning",
		"model_profile_name: ghost-model",
		1,
	)
	p := mustParse(t, src)
	err := Validate(p, testSnapshot(t))
	if err == nil || !strings.Contains(err.Error(), "ghost-model") {
		t.Fatalf("expected unknown model error, got %v", err)
	}
}

func TestValidateRejectsUnknownModelInConstraintList(t *testing.T) {
	src := strings.Replace(validPolicy,
		"require_capabilities: [tool_use, json_schema]",
		"allowed_models: [cheap-general, made-up]",
		1,
	)
	p := mustParse(t, src)
	err := Validate(p, testSnapshot(t))
	if err == nil || !strings.Contains(err.Error(), "made-up") {
		t.Fatalf("expected unknown model error in allowed_models, got %v", err)
	}
}

func TestValidateRejectsNilInputs(t *testing.T) {
	if err := Validate(nil, testSnapshot(t)); err == nil {
		t.Errorf("expected error for nil policy")
	}
	p := mustParse(t, validPolicy)
	if err := Validate(p, nil); err == nil {
		t.Errorf("expected error for nil snapshot")
	}
}

// wantErr is reused for inline error assertions where we already have the
// (policy, error) pair. Wrap for compactness.
func wantErrAssert(t *testing.T, p *Policy, err error, contains string) {
	wantErr(t, p, err, contains)
}

// helper to test that parse rejects malformed YAML
func TestParseRejectsMalformedYAML(t *testing.T) {
	_, err := Parse([]byte("version: pv_2026_05_19\nsettings: [not, a, map]\n"))
	if err == nil {
		t.Fatalf("expected error on malformed settings")
	}
}
