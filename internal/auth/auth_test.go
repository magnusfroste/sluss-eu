package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/tenant"
)

func newStore(t *testing.T) (*InMemoryKeyStore, *tenant.Tenant) {
	t.Helper()
	s := NewInMemoryKeyStore()
	target := &tenant.Tenant{ID: "tn_test", Project: "prj_test", KeyID: "key_test"}
	s.Add("super-secret", target)
	return s, target
}

func TestMiddleware_ValidBearerAttachesTenant(t *testing.T) {
	store, target := newStore(t)

	var got *tenant.Tenant
	h := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = tenant.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer super-secret")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got != target {
		t.Fatalf("tenant not attached: got %#v", got)
	}
}

func TestMiddleware_RejectsMissingHeader(t *testing.T) {
	store, _ := newStore(t)

	h := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be reached")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var env openai.ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response is not a valid error envelope: %v", err)
	}
	if env.Error.Type != "unauthorized" {
		t.Fatalf("expected type=unauthorized, got %q", env.Error.Type)
	}
}

func TestMiddleware_RejectsUnknownKey(t *testing.T) {
	store, _ := newStore(t)

	h := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be reached")
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer something-else")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestKeyStore_DoesNotRetainPlaintext(t *testing.T) {
	s := NewInMemoryKeyStore()
	s.Add("super-secret", &tenant.Tenant{ID: "tn"})

	for k := range s.keys {
		if k == "super-secret" {
			t.Fatal("plaintext key found in store; must store SHA-256 hash only")
		}
	}
}
