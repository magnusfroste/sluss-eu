package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/registry"
)

func policyEngine(t *testing.T) *engine.Engine {
	t.Helper()
	cost := registry.CostMetadata{Currency: "USD", InputMicrosPerMillionToken: 1_000_000, OutputMicrosPerMillionToken: 1_000_000}
	caps := registry.Capabilities{Chat: true, Streaming: true}
	def := registry.Definition{
		RegistryVersion: "reg-policy-test", CreatedAt: time.Unix(1_700_000_000, 0),
		Providers: []registry.Provider{
			{ID: "openrouter", Name: "OpenRouter", Status: registry.ProviderStatusActive, ComplianceTags: []string{"cloud"}},
			{ID: "dgx", Name: "DGX", Status: registry.ProviderStatusActive, ComplianceTags: []string{"local"}},
		},
		Models: []registry.Model{
			{ID: "cheap-general", ProviderID: "openrouter", ProviderModelID: "x", Tier: registry.TierCheap, Capabilities: caps, Cost: cost, Enabled: true},
			{ID: "qwen", ProviderID: "dgx", ProviderModelID: "qwen36-27b", Tier: registry.TierBalanced, Capabilities: caps, Cost: cost, Enabled: true},
		},
	}
	snap, err := registry.NewSnapshot(def)
	if err != nil {
		t.Fatalf("snap: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return engine.New(store)
}

// The dry-run endpoint classifies a prompt and returns a routing decision with
// egress — no provider call. A trivial prompt routes to the cloud.
func TestPolicyDryRunClassifiesAndRoutes(t *testing.T) {
	h := PolicyDryRunHandler(policyEngine(t), nil)
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/router/policy/dryrun",
		strings.NewReader(`{"prompt":"write a concise git commit message for a bugfix"}`)))
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"task"`, `"sensitivity"`, `"egress"`, `"selected_model"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("dry-run result missing %q: %s", want, body)
		}
	}
}

func TestPolicyDryRunRejectsEmpty(t *testing.T) {
	h := PolicyDryRunHandler(policyEngine(t), nil)
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/router/policy/dryrun", strings.NewReader(`{"prompt":"  "}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty prompt should be 400, got %d", rec.Code)
	}
}

func TestPolicyPageRenders(t *testing.T) {
	h := PolicyPageHandler(PolicyPageOptions{Engine: policyEngine(t), Version: "v-test"})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/router/policy", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Dry-run") {
		t.Fatalf("policy page did not render dry-run: %d", rec.Code)
	}
}
