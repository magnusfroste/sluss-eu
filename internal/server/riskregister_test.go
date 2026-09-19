package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/policy"
)

// registerEnv: roster-backed providers (via modelsTestOpts' store) + a policy
// cache whose active policy requires the "local" tag.
func registerEnv(t *testing.T) ProvidersOptions {
	t.Helper()
	m := modelsTestOpts(t)
	snap, err := m.Engine.Registry.Active()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	pol, err := policy.Parse([]byte(`
version: pv_register_test
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: pii_local
    when:
      sensitivity: pii
    route:
      constraints:
        require_provider_tags: [local]
  - id: default
    when: {}
    route:
      defaults:
        model_profile: balanced
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cache, err := policy.NewCache([]policy.Source{{Policy: pol, Registry: snap}})
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	sink := audit.NewMemorySink(0)
	return ProvidersOptions{
		Engine: m.Engine, Roster: m.Roster, Cache: cache, Auditor: sink, Version: "test",
	}
}

// RequiredProviderTags surfaces the tags the policy can require.
func TestRequiredProviderTags(t *testing.T) {
	o := registerEnv(t)
	p, ok := o.Cache.Active(policy.Scope{})
	if !ok {
		t.Fatal("no active policy")
	}
	tags := p.RequiredProviderTags()
	if len(tags) != 1 || tags[0] != "local" {
		t.Fatalf("required tags = %v, want [local]", tags)
	}
}

// The register page shows the curated matrix and warns about the required tag
// that no provider carries (fail-closed gap); tagging a provider clears it.
func TestRiskRegisterGapWarningAndTagUpdate(t *testing.T) {
	o := registerEnv(t)

	rec := httptest.NewRecorder()
	ProvidersPageHandler(o)(rec, httptest.NewRequest(http.MethodGet, "/router/providers", nil))
	body := rec.Body.String()
	for _, want := range []string{"Risk register", "dpa-signed", "iso27001", "subprocessors-vetted",
		"The active policy requires", "<b>local</b>", "fail-closed"} {
		if !strings.Contains(body, want) {
			t.Errorf("register page missing %q", want)
		}
	}

	// Tag the seeded openrouter provider local (+ extra) via the handler.
	form := url.Values{"id": {"openrouter"}, "tag": {"local", "dpa-signed"}, "extra_tags": {"dev-only"}}
	req := httptest.NewRequest(http.MethodPost, "/router/providers/tags", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tr := httptest.NewRecorder()
	ProvidersTagsHandler(o)(tr, req)
	if loc := tr.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Fatalf("tag update failed: %q", loc)
	}
	ps, _ := o.Roster.LoadRosterProviders()
	var tags []string
	for _, p := range ps {
		if p.ID == "openrouter" {
			tags = p.ComplianceTags
		}
	}
	if strings.Join(tags, ",") != "local,dpa-signed,dev-only" {
		t.Fatalf("tags not persisted: %v", tags)
	}

	// Audited.
	sink := o.Auditor.(*audit.MemorySink)
	var tagged bool
	for _, e := range sink.Entries() {
		if e.Action == audit.ActionProviderTags && e.Target == "openrouter" {
			tagged = true
		}
	}
	if !tagged {
		t.Fatal("tag change should be audited")
	}

	// Gap warning clears (roster now covers "local"; the pending provider row
	// carries the tag even before restart).
	rec = httptest.NewRecorder()
	ProvidersPageHandler(o)(rec, httptest.NewRequest(http.MethodGet, "/router/providers", nil))
	if strings.Contains(rec.Body.String(), "The active policy requires") {
		t.Fatal("gap warning should clear once a provider carries the tag")
	}
}

func TestTagsHandlerRejectsUnknownProvider(t *testing.T) {
	o := registerEnv(t)
	form := url.Values{"id": {"nope"}, "tag": {"local"}}
	req := httptest.NewRequest(http.MethodPost, "/router/providers/tags", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ProvidersTagsHandler(o)(rec, req)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=") {
		t.Fatalf("unknown provider should error, got %q", loc)
	}
}

// Non-curated checkbox values are dropped (only the curated vocabulary plus the
// free-text extras survive) — the checkbox surface can't inject tags.
func TestTagsHandlerFiltersUncurated(t *testing.T) {
	o := registerEnv(t)
	form := url.Values{"id": {"openrouter"}, "tag": {"local", "made-up-tag"}}
	req := httptest.NewRequest(http.MethodPost, "/router/providers/tags", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ProvidersTagsHandler(o)(rec, req)
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "err=") {
		t.Fatalf("update failed: %q", loc)
	}
	ps, _ := o.Roster.LoadRosterProviders()
	for _, p := range ps {
		if p.ID == "openrouter" {
			if strings.Join(p.ComplianceTags, ",") != "local" {
				t.Fatalf("uncurated checkbox tag should be dropped: %v", p.ComplianceTags)
			}
		}
	}
}
