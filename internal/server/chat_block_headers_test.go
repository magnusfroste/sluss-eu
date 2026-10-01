package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
)

// A fail-closed block tells the client what was stopped and why as stable
// ASCII codes (task, data class, block code) so the demo chat can explain the
// block instead of showing a raw error. Classes only — never content.
func TestBlockedResponseCarriesExplainHeaders(t *testing.T) {
	snap, err := registry.NewSnapshot(registry.DefaultDefinition())
	if err != nil {
		t.Fatal(err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatal(err)
	}
	cache, err := policy.NewRuntimeCache(snap, "builtin:nis2-baseline")
	if err != nil {
		t.Fatal(err)
	}
	mock := &provider.MockAdapter{BaseURL: "http://127.0.0.1:1"}
	h := ChatCompletionsHandler(mock, ChatOptions{
		Engine:      engine.New(store),
		Adapters:    map[string]provider.Adapter{"openai": mock, "anthropic": mock},
		PolicyCache: cache,
	})
	body := `{"model":"auto","messages":[{"role":"user","content":"security review our internal SSO login flow for auth bypass and secret leakage"}]}`
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (no air-gapped provider)", rec.Code)
	}
	if got := rec.Header().Get("X-Router-Blocked"); got != "residency_no_compliant_provider" {
		t.Fatalf("X-Router-Blocked = %q", got)
	}
	if got := rec.Header().Get("X-Router-Route-Class"); got != "security_review" {
		t.Fatalf("X-Router-Route-Class = %q", got)
	}
	if strings.Contains(rec.Header().Get("X-Router-Sensitivity"), "SSO") {
		t.Fatal("headers must carry classes, never prompt content")
	}
}
