package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/policy"
)

// setupEnv: seeded roster (cloud models only) and the built-in default
// policy, which does NOT keep personal data local.
func setupEnv(t *testing.T) SetupOptions {
	t.Helper()
	// A model counts as routable only when its provider's key is in the
	// environment — exactly what a real instance needs.
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	m := modelsTestOpts(t)
	snap, err := m.Engine.Registry.Active()
	if err != nil {
		t.Fatal(err)
	}
	cache, err := policy.NewDefaultRuntimeCache(snap)
	if err != nil {
		t.Fatal(err)
	}
	h := m.Roster.(*history.Store)
	sink := audit.NewMemorySink(0)
	return SetupOptions{Engine: m.Engine, Cache: cache, Roster: m.Roster, History: h, Auditor: sink,
		Rules: PolicyRulesOptions{Engine: m.Engine, Cache: cache, History: h, Auditor: sink}}
}

func post(h http.HandlerFunc, form url.Values) string {
	req := httptest.NewRequest(http.MethodPost, "/router/setup/x", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h(rec, req)
	loc, _ := url.QueryUnescape(rec.Header().Get("Location"))
	return loc
}

// ISSUE-124: the quick start reports live state, adds a tagged local model in
// one step, and adds the "personal data stays local" rule only when the
// active policy does not already protect it.
func TestQuickStartFlow(t *testing.T) {
	o := setupEnv(t)
	st := o.status()
	if len(st.CloudModels) == 0 || len(st.LocalModels) != 0 || st.Protects || st.Done() != 1 {
		t.Fatalf("fresh state = %+v (done %d)", st, st.Done())
	}

	// Validation: a key VALUE instead of an env var name is refused.
	if loc := post(SetupLocalHandler(o), url.Values{"name": {"GPU box"}, "base_url": {"https://gpu.example/v1"},
		"model": {"qwen3-27b"}, "key_env": {"sk-abc123secret"}}); !strings.Contains(loc, "err=") || !strings.Contains(loc, "NAME") {
		t.Fatalf("key value should be refused, got %q", loc)
	}
	loc := post(SetupLocalHandler(o), url.Values{"name": {"GPU box"}, "base_url": {"https://gpu.example/v1"},
		"model": {"qwen3-27b"}, "key_env": {"local_llm_key"}})
	if strings.Contains(loc, "err=") {
		t.Fatalf("add local: %q", loc)
	}
	provs, _ := o.Roster.LoadRosterProviders()
	var tagged bool
	for _, p := range provs {
		if p.ID == "gpu-box" && strings.Join(p.ComplianceTags, ",") == "local,on-prem" && p.KeyEnv == "LOCAL_LLM_KEY" {
			tagged = true
		}
	}
	if !tagged {
		t.Fatalf("provider gpu-box not saved with local tags: %+v", provs)
	}
	models, _ := o.Roster.LoadRosterModels()
	var priced bool
	for _, m := range models {
		if m.ID == "gpu-box-local" && m.ProviderModelID == "qwen3-27b" && m.Tier == "balanced" && m.InputUSDPerMTok == 1 && m.Enabled {
			priced = true
		}
	}
	if !priced {
		t.Fatalf("model gpu-box-local not saved as expected: %+v", models)
	}

	// Protect: adds and activates the console rule; the policy then keeps PII local.
	if loc := post(SetupProtectHandler(o), url.Values{}); strings.Contains(loc, "err=") {
		t.Fatalf("protect: %q", loc)
	}
	if ok, v, _ := policyProtectsPII(o.Cache); !ok || !strings.HasPrefix(v, "pv_console_") {
		t.Fatalf("after protect: protects=%v version=%q", ok, v)
	}
	// Second press is a no-op — it never stacks rules or replaces a policy.
	before := len(LoadConsoleRules(o.History))
	if loc := post(SetupProtectHandler(o), url.Values{}); !strings.Contains(loc, "already keeps personal data local") {
		t.Fatalf("second protect should be a no-op, got %q", loc)
	}
	if after := len(LoadConsoleRules(o.History)); after != before {
		t.Fatalf("rules changed on no-op: %d → %d", before, after)
	}

	rec := httptest.NewRecorder()
	SetupPageHandler(o)(rec, httptest.NewRequest(http.MethodGet, "/router/setup", nil))
	body := rec.Body.String()
	for _, want := range []string{"Quick start", "protected", "Run the dry-run", "Personal data stays in the house"} {
		if !strings.Contains(body, want) {
			t.Errorf("setup page missing %q", want)
		}
	}
}

func TestSlugID(t *testing.T) {
	for in, want := range map[string]string{"PrivAI RTX": "privai-rtx", "  GPU box!! ": "gpu-box", "Ö": ""} {
		if got := slugID(in); got != want {
			t.Errorf("slugID(%q) = %q, want %q", in, got, want)
		}
	}
}
