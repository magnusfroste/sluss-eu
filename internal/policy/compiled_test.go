package policy

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestCompileBuildsFastPathMatchers(t *testing.T) {
	p := mustParse(t, validPolicy)
	compiled, err := Compile(p, testSnapshot(t))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if compiled.Version() != "pv_2026_05_19" {
		t.Fatalf("version = %q", compiled.Version())
	}
	if compiled.RegistryVersion() == "" {
		t.Fatalf("registry version should be captured")
	}
	if got := compiled.RuleCount(); got != 4 {
		t.Fatalf("rule count = %d, want 4", got)
	}

	decision := compiled.Evaluate(EvaluationInput{
		TaskType:     "hard_code_debugging",
		RiskLevel:    "critical",
		FilesTouched: []string{"src/auth/session.ts"},
		RouterMode:   "auto",
	})
	if strings.Join(decision.MatchedRuleIDs, ",") != "auth_premium,default_balanced" {
		t.Fatalf("matched rules = %v", decision.MatchedRuleIDs)
	}
	if !containsExplanation(decision.Explanations, "Policy rule auth_premium matched") {
		t.Fatalf("missing matched-rule explanation: %v", decision.Explanations)
	}
	if decision.Route.Force == nil || decision.Route.Force.ModelProfileName != "premium-reasoning" {
		t.Fatalf("force route not applied: %+v", decision.Route.Force)
	}
	if decision.Route.Defaults == nil || decision.Route.Defaults.ModelProfile != ProfileBalanced {
		t.Fatalf("default route not applied: %+v", decision.Route.Defaults)
	}
}

func TestCompileBlockStopsEvaluation(t *testing.T) {
	compiled, err := Compile(mustParse(t, validPolicy), testSnapshot(t))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	decision := compiled.Evaluate(EvaluationInput{RouterMode: "disabled"})
	if !decision.Blocked {
		t.Fatalf("decision should be blocked")
	}
	if strings.Join(decision.MatchedRuleIDs, ",") != "block_disabled" {
		t.Fatalf("matched rules = %v", decision.MatchedRuleIDs)
	}
	if decision.Route.Block == nil || decision.Route.Block.Code != "router_disabled" {
		t.Fatalf("block route not applied: %+v", decision.Route.Block)
	}
}

func TestCompileExplainsShadowedForceFields(t *testing.T) {
	src := `
version: pv_force_shadow
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: first_force
    when:
      task_type: security_review
    route:
      force:
        model_profile: premium
        timeout_ms: 45000
  - id: second_force
    when:
      task_type: security_review
    route:
      force:
        model_profile: cheap
        timeout_ms: 10000
`
	compiled, err := Compile(mustParse(t, src), testSnapshot(t))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	decision := compiled.Evaluate(EvaluationInput{TaskType: "security_review"})
	if decision.Route.Force == nil || decision.Route.Force.ModelProfile != ProfilePremium {
		t.Fatalf("first force model_profile should win: %+v", decision.Route.Force)
	}
	if decision.Route.Force.TimeoutMS == nil || *decision.Route.Force.TimeoutMS != 45000 {
		t.Fatalf("first force timeout should win: %+v", decision.Route.Force)
	}
	if !containsExplanation(decision.Explanations, "force.model_profile ignored") {
		t.Fatalf("missing shadow explanation: %v", decision.Explanations)
	}
	if !containsExplanation(decision.Explanations, "force.timeout_ms ignored") {
		t.Fatalf("missing timeout shadow explanation: %v", decision.Explanations)
	}
}

func TestExplainEnabledReadsRouterHeader(t *testing.T) {
	headers := http.Header{}
	if ExplainEnabled(headers) {
		t.Fatalf("explain should default off")
	}
	headers.Set("X-Router-Explain", "true")
	if !ExplainEnabled(headers) {
		t.Fatalf("explain should be enabled")
	}
	headers.Set("X-Router-Explain", "0")
	if ExplainEnabled(headers) {
		t.Fatalf("explain should reject non-truthy values")
	}
}

func TestEvaluationLogFieldsAreSafeAndStructured(t *testing.T) {
	compiled, err := Compile(mustParse(t, validPolicy), testSnapshot(t))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	decision := compiled.Evaluate(EvaluationInput{
		TaskType:     "hard_code_debugging",
		RiskLevel:    "critical",
		FilesTouched: []string{"src/auth/session.ts"},
		ContainsText: "raw prompt should not appear in log fields unless a policy reason includes it",
	})
	fields := decision.LogFields()
	if len(fields)%2 != 0 {
		t.Fatalf("log fields should be key/value pairs: %v", fields)
	}
	joined := fmt.Sprint(fields)
	if strings.Contains(joined, "raw prompt") {
		t.Fatalf("log fields leaked ContainsText: %v", fields)
	}
	if !strings.Contains(joined, "auth_premium") {
		t.Fatalf("log fields should include matched rules: %v", fields)
	}
}

func TestCompileMapsRouteHintsAndMergesConstraints(t *testing.T) {
	src := `
version: pv_hints
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: hints
    when:
      task_type: simple_code_edit
    route:
      tier: cheap
      provider: openai
      verifier: true
      timeout_ms: 15000
      fallback_tier: balanced
      max_cost_usd: 0.05
      require_capability: json_schema
  - id: stricter
    when:
      task_type: simple_code_edit
    route:
      constraints:
        max_cost_usd: 0.01
        retention: none
        require_capabilities: [tool_use]
  - id: default
    when: {}
    route:
      defaults:
        model_profile: premium
`
	compiled, err := Compile(mustParse(t, src), testSnapshot(t))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	decision := compiled.Evaluate(EvaluationInput{TaskType: "simple_code_edit"})
	if strings.Join(decision.MatchedRuleIDs, ",") != "hints,stricter,default" {
		t.Fatalf("matched rules = %v", decision.MatchedRuleIDs)
	}
	if decision.Route.Force == nil || decision.Route.Force.Provider != "openai" {
		t.Fatalf("provider hint not mapped to force: %+v", decision.Route.Force)
	}
	if decision.Route.Force.Verifier == nil || !*decision.Route.Force.Verifier {
		t.Fatalf("verifier hint not mapped to force: %+v", decision.Route.Force)
	}
	if decision.Route.Force.TimeoutMS == nil || *decision.Route.Force.TimeoutMS != 15000 {
		t.Fatalf("timeout hint not mapped to force: %+v", decision.Route.Force)
	}
	if decision.Route.Constraints == nil || decision.Route.Constraints.MaxCostUSD == nil || *decision.Route.Constraints.MaxCostUSD != 0.01 {
		t.Fatalf("max cost should keep strictest value: %+v", decision.Route.Constraints)
	}
	if decision.Route.Constraints.Retention != RetentionNone {
		t.Fatalf("retention should keep strictest value: %+v", decision.Route.Constraints)
	}
	if got := decision.Route.Constraints.RequireCapabilities; len(got) != 2 || got[0] != CapJSONSchema || got[1] != CapToolUse {
		t.Fatalf("require capabilities = %v", got)
	}
	if got := decision.Route.Constraints.FallbackModelProfiles; len(got) != 1 || got[0] != ProfileBalanced {
		t.Fatalf("fallback profiles = %v", got)
	}
}

func TestCacheScopeLookupAndFallback(t *testing.T) {
	defaultPolicy := mustParse(t, strings.Replace(validPolicy, "pv_2026_05_19", "pv_default", 1))
	tenantPolicy := mustParse(t, strings.Replace(validPolicy, "pv_2026_05_19", "pv_tenant", 1))
	projectPolicy := mustParse(t, strings.Replace(validPolicy, "pv_2026_05_19", "pv_project", 1))
	snapshot := testSnapshot(t)

	cache, err := NewCache([]Source{
		{Scope: Scope{}, Policy: defaultPolicy, Registry: snapshot},
		{Scope: Scope{TenantID: "tn_1"}, Policy: tenantPolicy, Registry: snapshot},
		{Scope: Scope{TenantID: "tn_1", ProjectID: "prj_1"}, Policy: projectPolicy, Registry: snapshot},
	})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	if p, ok := cache.Active(Scope{TenantID: "tn_1", ProjectID: "prj_1"}); !ok || p.Version() != "pv_project" {
		t.Fatalf("project policy = %v, %v", p, ok)
	}
	if p, ok := cache.Active(Scope{TenantID: "tn_1", ProjectID: "missing"}); !ok || p.Version() != "pv_tenant" {
		t.Fatalf("tenant fallback policy = %v, %v", p, ok)
	}
	if p, ok := cache.Active(Scope{TenantID: "unknown", ProjectID: "missing"}); !ok || p.Version() != "pv_default" {
		t.Fatalf("default fallback policy = %v, %v", p, ok)
	}
}

func TestCacheReloadKeepsPreviousPolicyOnFailure(t *testing.T) {
	snapshot := testSnapshot(t)
	cache, err := NewCache([]Source{{Policy: mustParse(t, validPolicy), Registry: snapshot}})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	bad := mustParse(t, strings.Replace(validPolicy, "premium-reasoning", "ghost-model", 1))
	if err := cache.Reload([]Source{{Policy: bad, Registry: snapshot}}); err == nil {
		t.Fatalf("expected reload validation error")
	}
	active, ok := cache.Active(Scope{})
	if !ok {
		t.Fatalf("expected previous policy to remain active")
	}
	if active.Version() != "pv_2026_05_19" {
		t.Fatalf("active version = %q", active.Version())
	}

	next := mustParse(t, strings.Replace(validPolicy, "pv_2026_05_19", "pv_next", 1))
	if err := cache.Reload([]Source{{Policy: next, Registry: snapshot}}); err != nil {
		t.Fatalf("valid reload: %v", err)
	}
	active, ok = cache.Active(Scope{})
	if !ok || active.Version() != "pv_next" {
		t.Fatalf("active after reload = %v, %v", active, ok)
	}
}

func TestCacheRejectsDuplicateScope(t *testing.T) {
	snapshot := testSnapshot(t)
	_, err := NewCache([]Source{
		{Scope: Scope{TenantID: "tn_1"}, Policy: mustParse(t, validPolicy), Registry: snapshot},
		{Scope: Scope{TenantID: "tn_1"}, Policy: mustParse(t, validPolicy), Registry: snapshot},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate policy scope") {
		t.Fatalf("expected duplicate scope error, got %v", err)
	}
}

func TestCacheRejectsProjectScopeWithoutTenant(t *testing.T) {
	_, err := NewCache([]Source{
		{Scope: Scope{ProjectID: "prj_1"}, Policy: mustParse(t, validPolicy), Registry: testSnapshot(t)},
	})
	if err == nil || !strings.Contains(err.Error(), "requires tenant id") {
		t.Fatalf("expected project without tenant error, got %v", err)
	}
}

func TestCompileContextPipelineForceDefaultsOffAndFirstMatchWins(t *testing.T) {
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
      tenant: tn_enabled
    route:
      force:
        context_pipeline: true
  - id: disable_context_pipeline_later
    when:
      tenant: tn_enabled
    route:
      force:
        context_pipeline: false
`
	compiled, err := Compile(mustParse(t, src), testSnapshot(t))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	enabled := compiled.Evaluate(EvaluationInput{TenantID: "tn_enabled"})
	if !enabled.Route.ContextPipelineEnabled() {
		t.Fatalf("context pipeline should be enabled: %+v", enabled.Route.Force)
	}
	if !containsExplanation(enabled.Explanations, "force.context_pipeline ignored") {
		t.Fatalf("missing force shadow explanation: %v", enabled.Explanations)
	}

	disabled := compiled.Evaluate(EvaluationInput{TenantID: "tn_other"})
	if disabled.Route.ContextPipelineEnabled() {
		t.Fatalf("context pipeline should default off: %+v", disabled.Route.Force)
	}
}

func TestNewDefaultRuntimeCache(t *testing.T) {
	cache, err := NewDefaultRuntimeCache(testSnapshot(t))
	if err != nil {
		t.Fatalf("NewDefaultRuntimeCache: %v", err)
	}
	active, ok := cache.Active(Scope{})
	if !ok {
		t.Fatal("expected default scope policy")
	}
	if got := active.Version(); got != "pv_runtime_2026_06_12" {
		t.Fatalf("version = %q", got)
	}
	eval := active.Evaluate(EvaluationInput{RouterMode: "auto"})
	if eval.Route.ContextPipelineEnabled() {
		t.Fatalf("default runtime policy should leave context pipeline off")
	}
}

func containsExplanation(explanations []string, fragment string) bool {
	for _, explanation := range explanations {
		if strings.Contains(explanation, fragment) {
			return true
		}
	}
	return false
}
