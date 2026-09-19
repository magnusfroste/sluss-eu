package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/apikeys"
	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/tenant"
)

func testKeyManager(t *testing.T) *apikeys.Manager {
	t.Helper()
	store, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return apikeys.NewManager(store, auth.NewInMemoryKeyStore(), nil)
}

func TestKeysMintListRevokeFlow(t *testing.T) {
	mgr := testKeyManager(t)

	// Mint.
	body, _ := json.Marshal(map[string]any{"tenant_id": "tn_eko", "project_id": "prj", "role": "user"})
	rec := httptest.NewRecorder()
	KeysMintHandler(mgr)(rec, httptest.NewRequest(http.MethodPost, "/router/keys", bytes.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("mint status=%d body=%s", rec.Code, rec.Body.String())
	}
	var mint struct {
		KeyID string `json:"key_id"`
		Key   string `json:"key"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &mint)
	if !strings.HasPrefix(mint.Key, "tk_") || mint.KeyID == "" {
		t.Fatalf("mint response missing key/key_id: %s", rec.Body.String())
	}

	// List — secretless, shows the key.
	rec = httptest.NewRecorder()
	KeysListHandler(mgr)(rec, httptest.NewRequest(http.MethodGet, "/router/keys", nil))
	if strings.Contains(rec.Body.String(), mint.Key) {
		t.Fatal("list response leaked the secret")
	}
	if !strings.Contains(rec.Body.String(), mint.KeyID) {
		t.Fatalf("list should include the key id: %s", rec.Body.String())
	}

	// Revoke via path value.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/router/keys/"+mint.KeyID, nil)
	req.SetPathValue("key_id", mint.KeyID)
	KeysRevokeHandler(mgr)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Revoking again → 404.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/router/keys/"+mint.KeyID, nil)
	req.SetPathValue("key_id", mint.KeyID)
	KeysRevokeHandler(mgr)(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("second revoke status=%d, want 404", rec.Code)
	}
}

func TestKeysMintRejectsMissingTenant(t *testing.T) {
	mgr := testKeyManager(t)
	rec := httptest.NewRecorder()
	KeysMintHandler(mgr)(rec, httptest.NewRequest(http.MethodPost, "/router/keys", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 for missing tenant_id", rec.Code)
	}
}

// The mint route is admin-gated: a non-admin key is rejected before it reaches
// the handler.
func TestKeysRouteRequiresAdmin(t *testing.T) {
	store, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	ks := auth.NewInMemoryKeyStore()
	ks.Add("user_key", &tenant.Tenant{ID: "tn_x", Role: auth.RoleUser, Scopes: auth.AllScopes()})
	mgr := apikeys.NewManager(store, ks, nil)

	h := New(Config{KeyStore: ks, KeyManager: mgr})
	req := httptest.NewRequest(http.MethodGet, "/router/keys", nil)
	req.Header.Set("Authorization", "Bearer user_key")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin GET /router/keys = %d, want 403", rec.Code)
	}
}
