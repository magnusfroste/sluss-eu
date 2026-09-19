package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/policy"
)

// ruleTestEnv builds a live cache over the local/cloud test registry plus a
// SQLite store — the full console-rule machinery without a provider call.
func ruleTestEnv(t *testing.T) (PolicyRulesOptions, *audit.MemorySink) {
	t.Helper()
	eng := localRegistryStore(t)
	snap, err := eng.Registry.Active()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	cache, err := policy.NewRuntimeCache(snap, "")
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	store, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	sink := audit.NewMemorySink(0)
	return PolicyRulesOptions{
		Engine: eng, Cache: cache, History: store, Auditor: sink, BaselinePath: "",
	}, sink
}

func postForm(t *testing.T, h http.HandlerFunc, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h(rec, req)
	return rec
}

func activeVersion(t *testing.T, o PolicyRulesOptions) string {
	t.Helper()
	p, ok := o.Cache.Active(policy.Scope{})
	if !ok {
		t.Fatal("no active policy")
	}
	return p.Version()
}

// Add rule → activate → the live cache serves the console policy and the rule
// actually constrains routing; rollback returns to the baseline. All audited.
func TestConsoleRulesActivateAndRollback(t *testing.T) {
	o, sink := ruleTestEnv(t)
	base := activeVersion(t, o)

	// Add: health/financial → require local.
	rec := postForm(t, PolicyRuleAddHandler(o), "/router/policy/rules", url.Values{
		"sensitivity": {"health", "financial"},
		"action":      {"require_tags"},
		"tags":        {"local"},
		"note":        {"special-category data stays in the house"},
	})
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Fatalf("add failed: %q", loc)
	}
	if got := LoadConsoleRules(o.History); len(got) != 1 || got[0].Sensitivities[1] != "health" {
		t.Fatalf("rule not stored: %+v", got)
	}
	// Draft: the live policy is untouched until Activate.
	if v := activeVersion(t, o); v != base {
		t.Fatalf("draft must not change the live policy: %q", v)
	}

	// Activate.
	rec = postForm(t, PolicyActivateHandler(o), "/router/policy/activate", url.Values{})
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Fatalf("activate failed: %q", loc)
	}
	v := activeVersion(t, o)
	if !strings.HasPrefix(v, "pv_console_") {
		t.Fatalf("live policy should be the console ruleset, got %q", v)
	}
	if !ConsolePolicyActive(o.History) {
		t.Fatal("active flag not set")
	}
	// The rule constrains for health, not for none.
	p, _ := o.Cache.Active(policy.Scope{})
	ev := p.Evaluate(policy.EvaluationInput{TaskType: "summarization", RiskLevel: "medium", Sensitivity: "health"})
	if ev.Route.Constraints == nil || len(ev.Route.Constraints.RequireProviderTags) == 0 {
		t.Fatalf("health should be tag-constrained: %+v", ev)
	}
	ev = p.Evaluate(policy.EvaluationInput{TaskType: "summarization", RiskLevel: "low", Sensitivity: "none"})
	if ev.Route.Constraints != nil && len(ev.Route.Constraints.RequireProviderTags) > 0 {
		t.Fatalf("non-sensitive prompt should be unconstrained: %+v", ev)
	}

	// Rollback → baseline version again, flag cleared, rules preserved.
	rec = postForm(t, PolicyRollbackHandler(o), "/router/policy/rollback", url.Values{})
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Fatalf("rollback failed: %q", loc)
	}
	if got := activeVersion(t, o); got != base {
		t.Fatalf("rollback should restore baseline %q, got %q", base, got)
	}
	if ConsolePolicyActive(o.History) {
		t.Fatal("active flag should clear on rollback")
	}
	if got := LoadConsoleRules(o.History); len(got) != 1 {
		t.Fatalf("rules should survive a rollback: %+v", got)
	}

	// Audit: console activate + rollback entries present.
	var console int
	for _, e := range sink.Entries() {
		if e.Action == audit.ActionPolicyConsole {
			console++
		}
	}
	if console < 2 {
		t.Fatalf("expected activate + rollback audit entries, got %d", console)
	}
}

// Editing a LIVE ruleset re-activates immediately so the table never lies.
func TestConsoleRuleEditWhileActiveReapplies(t *testing.T) {
	o, _ := ruleTestEnv(t)
	postForm(t, PolicyRuleAddHandler(o), "/router/policy/rules", url.Values{
		"sensitivity": {"legal"}, "action": {"block"},
	})
	postForm(t, PolicyActivateHandler(o), "/router/policy/activate", url.Values{})
	v1 := activeVersion(t, o)

	// Delete the only rule while live → re-activated (new version), legal unblocked.
	postForm(t, PolicyRuleDeleteHandler(o), "/router/policy/rules/delete", url.Values{"index": {"0"}})
	v2 := activeVersion(t, o)
	if v1 == v2 || !strings.HasPrefix(v2, "pv_console_") {
		t.Fatalf("live edit should re-activate with a new version: %q → %q", v1, v2)
	}
	p, _ := o.Cache.Active(policy.Scope{})
	ev := p.Evaluate(policy.EvaluationInput{TaskType: "summarization", RiskLevel: "medium", Sensitivity: "legal"})
	if ev.Blocked {
		t.Fatalf("deleted block rule still fires: %+v", ev)
	}
}

func TestConsoleRuleValidation(t *testing.T) {
	o, _ := ruleTestEnv(t)
	// No condition at all → error redirect, nothing stored.
	rec := postForm(t, PolicyRuleAddHandler(o), "/router/policy/rules", url.Values{
		"action": {"require_tags"}, "tags": {"local"},
	})
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=") {
		t.Fatalf("conditionless rule should be rejected, got %q", loc)
	}
	// Bad tag characters → rejected (nothing user-controlled leaks into YAML).
	rec = postForm(t, PolicyRuleAddHandler(o), "/router/policy/rules", url.Values{
		"sensitivity": {"pii"}, "action": {"require_tags"}, "tags": {"local, bad tag!"},
	})
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=") {
		t.Fatalf("invalid tag should be rejected, got %q", loc)
	}
	if got := LoadConsoleRules(o.History); len(got) != 0 {
		t.Fatalf("invalid rules must not persist: %+v", got)
	}
	// Activate with no rules → error.
	rec = postForm(t, PolicyActivateHandler(o), "/router/policy/activate", url.Values{})
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=") {
		t.Fatalf("empty activate should error, got %q", loc)
	}
}

// A live console ruleset is re-activated from the DB at boot (redeploy-safe),
// keeping its version.
func TestApplyStoredConsolePolicyAtBoot(t *testing.T) {
	o, _ := ruleTestEnv(t)
	postForm(t, PolicyRuleAddHandler(o), "/router/policy/rules", url.Values{
		"sensitivity": {"security_classified"}, "action": {"block"},
	})
	postForm(t, PolicyActivateHandler(o), "/router/policy/activate", url.Values{})
	v := activeVersion(t, o)

	// Simulate a restart: fresh cache from the baseline, then boot re-apply.
	snap, _ := o.Engine.Registry.Active()
	fresh, err := policy.NewRuntimeCache(snap, "")
	if err != nil {
		t.Fatalf("fresh cache: %v", err)
	}
	applied, err := ApplyStoredConsolePolicy(fresh, snap, o.History)
	if err != nil || applied != v {
		t.Fatalf("boot re-apply = %q, %v; want %q", applied, err, v)
	}
	p, _ := fresh.Active(policy.Scope{})
	ev := p.Evaluate(policy.EvaluationInput{TaskType: "summarization", RiskLevel: "high", Sensitivity: "security_classified"})
	if !ev.Blocked {
		t.Fatalf("re-applied ruleset should block classified: %+v", ev)
	}

	// Not active → no-op.
	postForm(t, PolicyRollbackHandler(o), "/router/policy/rollback", url.Values{})
	applied, err = ApplyStoredConsolePolicy(fresh, snap, o.History)
	if err != nil || applied != "" {
		t.Fatalf("inactive ruleset should not re-apply, got %q, %v", applied, err)
	}
}

// The generated YAML is a valid policy in the standard dialect.
func TestGenerateConsolePolicyYAMLParses(t *testing.T) {
	yaml := GenerateConsolePolicyYAML("pv_console_007", []ConsoleRule{
		{ID: "r1", Sensitivities: []string{"pii", "health"}, Action: "require_tags", Tags: []string{"local", "eu-resident"}},
		{ID: "r2", TaskType: "security_review", RiskLevel: "high", Action: "block"},
	})
	p, err := policy.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("generated YAML should parse: %v\n%s", err, yaml)
	}
	if p.Version != "pv_console_007" || len(p.Rules) != 3 { // 2 rules + default
		t.Fatalf("unexpected parse result: version=%q rules=%d", p.Version, len(p.Rules))
	}
}

// The policy page renders the rules card with the console vocabulary.
func TestPolicyPageShowsRulesCard(t *testing.T) {
	o, _ := ruleTestEnv(t)
	postForm(t, PolicyRuleAddHandler(o), "/router/policy/rules", url.Values{
		"sensitivity": {"health"}, "action": {"require_tags"}, "tags": {"local"},
	})
	h := PolicyPageHandler(PolicyPageOptions{Engine: o.Engine, Cache: o.Cache, History: o.History})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/router/policy", nil))
	body := rec.Body.String()
	for _, want := range []string{"Rules — no YAML", "security_classified", "Activate ruleset", "sensitivity: health", "require provider tags: local"} {
		if !strings.Contains(body, want) {
			t.Errorf("policy page missing %q", want)
		}
	}
}
