package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/magnusfroste/sluss/internal/history"
)

func demoLinkStore(t *testing.T) *history.Store {
	t.Helper()
	s, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// A valid share token starts a demo session and lands on /chat; that session
// then reaches the demo chat but is barred from admin pages.
func TestDemoLinkGrantsDemoOnlyAccess(t *testing.T) {
	store := demoLinkStore(t)
	tok, err := GenerateDemoShareToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if err := SetDemoShareToken(store, tok); err != nil {
		t.Fatalf("set token: %v", err)
	}
	a := &AdminAuth{Sessions: NewSessionStore(nil), Store: store}

	// Open the link.
	rec := httptest.NewRecorder()
	DemoEntryHandler(a, store, nil)(rec, httptest.NewRequest(http.MethodGet, "/demo?token="+tok, nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/chat" {
		t.Fatalf("valid token should redirect to /chat, got %d %q", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != sessionCookie {
		t.Fatal("demo link should set a session cookie")
	}
	cookie := cookies[0]

	// The demo session reaches the demo chat (RequireDemo) with role=demo.
	var gotRole string
	demo := a.RequireDemo(func(w http.ResponseWriter, r *http.Request) {
		gotRole = RoleFromContext(r.Context())
		w.WriteHeader(200)
	})
	dreq := httptest.NewRequest(http.MethodGet, "/chat", nil)
	dreq.AddCookie(cookie)
	drec := httptest.NewRecorder()
	demo.ServeHTTP(drec, dreq)
	if drec.Code != 200 || gotRole != roleDemo {
		t.Fatalf("demo session should reach /chat as demo, got %d role=%q", drec.Code, gotRole)
	}

	// But the SAME session is bounced from an admin page (Require).
	admin := a.Require(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	areq := httptest.NewRequest(http.MethodGet, "/router/keys", nil)
	areq.AddCookie(cookie)
	arec := httptest.NewRecorder()
	admin.ServeHTTP(arec, areq)
	if arec.Code != http.StatusFound || arec.Header().Get("Location") != "/chat" {
		t.Fatalf("demo session must be barred from admin, got %d %q", arec.Code, arec.Header().Get("Location"))
	}
	// A non-GET admin action by a demo session is forbidden, not redirected.
	preq := httptest.NewRequest(http.MethodPost, "/router/keys", nil)
	preq.AddCookie(cookie)
	prec := httptest.NewRecorder()
	admin.ServeHTTP(prec, preq)
	if prec.Code != http.StatusForbidden {
		t.Fatalf("demo POST to admin should be 403, got %d", prec.Code)
	}
}

// A wrong or absent token, or a disabled link, never starts a session.
func TestDemoLinkRejectsBadOrDisabledToken(t *testing.T) {
	store := demoLinkStore(t)
	a := &AdminAuth{Sessions: NewSessionStore(nil), Store: store}

	// Disabled (no token stored): any token is rejected.
	rec := httptest.NewRecorder()
	DemoEntryHandler(a, store, nil)(rec, httptest.NewRequest(http.MethodGet, "/demo?token=anything", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/router/login" {
		t.Fatalf("disabled link should redirect to login, got %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("rejected demo entry must not set a cookie")
	}

	// Enabled, but wrong token.
	_ = SetDemoShareToken(store, "the-real-token")
	rec2 := httptest.NewRecorder()
	DemoEntryHandler(a, store, nil)(rec2, httptest.NewRequest(http.MethodGet, "/demo?token=wrong", nil))
	if rec2.Code != http.StatusFound || rec2.Header().Get("Location") != "/router/login" {
		t.Fatalf("wrong token should redirect to login, got %d", rec2.Code)
	}
	if len(rec2.Result().Cookies()) != 0 {
		t.Fatal("wrong token must not set a cookie")
	}
}

func TestDemoShareURL(t *testing.T) {
	if got := DemoShareURL("https://x.example.com/", "abc"); got != "https://x.example.com/demo?token=abc" {
		t.Fatalf("url = %q", got)
	}
	if got := DemoShareURL("", "abc"); got != "/demo?token=abc" {
		t.Fatalf("relative url = %q", got)
	}
	if got := DemoShareURL("https://x", ""); got != "" {
		t.Fatalf("no token should yield empty, got %q", got)
	}
}
