package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// scopeChain wires Middleware + RequireScope around a 200 handler, mirroring the
// server wiring, and returns the recorder for a request authenticated with key.
func scopeChain(t *testing.T, keyScopes []string, required, key string) *httptest.ResponseRecorder {
	t.Helper()
	store := NewInMemoryKeyStore()
	store.Add("secret", &tenant.Tenant{ID: "tn", KeyID: "key_1", Scopes: keyScopes})

	handler := Middleware(store)(RequireScope(required)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestRequireScope_AllowsWhenScopePresent(t *testing.T) {
	rec := scopeChain(t, []string{ScopeChatCompletions}, ScopeChatCompletions, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireScope_AllowsUnrestrictedKey(t *testing.T) {
	rec := scopeChain(t, nil, ScopeRouterDecision, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("empty scope set should be unrestricted, got %d", rec.Code)
	}
}

func TestRequireScope_RejectsMissingScope(t *testing.T) {
	rec := scopeChain(t, []string{ScopeChatCompletions}, ScopeRouterDecision, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
	var env openai.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if env.Error.Code != "insufficient_scope" {
		t.Errorf("error code = %q, want insufficient_scope", env.Error.Code)
	}
}

func TestRequireScope_NoTenantIsForbidden(t *testing.T) {
	// RequireScope without a preceding Middleware → no tenant on context.
	handler := RequireScope(ScopeChatCompletions)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}
