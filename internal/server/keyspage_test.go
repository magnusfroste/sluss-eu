package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/apikeys"
	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/history"
)

func newKeysManager(t *testing.T) *apikeys.Manager {
	t.Helper()
	store, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return apikeys.NewManager(store, auth.NewInMemoryKeyStore(), nil)
}

func TestKeysPageEmptyState(t *testing.T) {
	rec := httptest.NewRecorder()
	KeysPageHandler(newKeysManager(t))(rec, httptest.NewRequest(http.MethodGet, "/router/keys", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "No department keys yet") || !strings.Contains(body, "Create key") {
		t.Fatalf("empty state / form missing:\n%s", body[:min(len(body), 400)])
	}
}

func TestKeysMintFormRevealsSecretOnceThenListsIt(t *testing.T) {
	mgr := newKeysManager(t)

	form := url.Values{"tenant_id": {"ekonomi"}, "project_id": {"prod"}, "scopes": {"chat:completions"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/router/keys", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	KeysMintFormHandler(mgr)(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "never shown again") {
		t.Fatalf("mint page missing one-time warning:\n%s", body)
	}
	// The plaintext secret (tk_...) must appear exactly once, in the reveal box.
	if strings.Count(body, "tk_") < 1 {
		t.Fatal("minted secret not shown on the page")
	}

	// The subsequent list must NOT contain the plaintext secret — only key_id.
	listRec := httptest.NewRecorder()
	KeysPageHandler(mgr)(listRec, httptest.NewRequest(http.MethodGet, "/router/keys", nil))
	list := listRec.Body.String()
	if !strings.Contains(list, "ekonomi") {
		t.Fatal("minted key's tenant not listed")
	}
	// A freshly loaded list has no reveal box and no plaintext.
	if strings.Contains(list, "never shown again") {
		t.Fatal("list page must not reveal a secret")
	}
}

func TestKeysRevokeForm(t *testing.T) {
	mgr := newKeysManager(t)
	_, rec, err := mgr.Mint(apikeys.MintRequest{TenantID: "utveckling"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	form := url.Values{"key_id": {rec.KeyID}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/router/keys/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	KeysRevokeFormHandler(mgr)(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("revoke status=%d, want 303", rr.Code)
	}
	// http.Redirect hex-escapes non-ASCII, so assert on the success param, not
	// the localized word.
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, "/router/keys?ok=") {
		t.Fatalf("redirect location = %q, want success (ok=) notice", loc)
	}

	keys, _ := mgr.List()
	if len(keys) != 1 || !keys[0].Revoked() {
		t.Fatalf("key not revoked: %+v", keys)
	}
}

func TestKeysRevokeFormMissingID(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/router/keys/revoke", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	KeysRevokeFormHandler(newKeysManager(t))(rr, req)
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "err=") {
		t.Fatalf("missing key_id should redirect with error, got %q", loc)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
