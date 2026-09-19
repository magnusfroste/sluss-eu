package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/tenant"
)

func roleChain(t *testing.T, role, required, key string) *httptest.ResponseRecorder {
	t.Helper()
	store := NewInMemoryKeyStore()
	store.Add("secret", &tenant.Tenant{ID: "tn", KeyID: "key_1", Role: role})

	handler := Middleware(store)(RequireRole(required)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/router/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestRequireRole_AllowsAdmin(t *testing.T) {
	rec := roleChain(t, RoleAdmin, RoleAdmin, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireRole_AllowsLegacyKey(t *testing.T) {
	rec := roleChain(t, "", RoleAdmin, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy key should remain unrestricted, got %d", rec.Code)
	}
}

func TestRequireRole_RejectsUserForAdminRoute(t *testing.T) {
	rec := roleChain(t, RoleUser, RoleAdmin, "secret")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
	var env openai.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if env.Error.Code != "insufficient_role" {
		t.Errorf("error code = %q, want insufficient_role", env.Error.Code)
	}
}

func TestRequireRole_NoTenantIsForbidden(t *testing.T) {
	handler := RequireRole(RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/router/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}
