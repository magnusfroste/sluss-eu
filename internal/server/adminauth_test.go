package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/history"
)

func TestPasswordHashVerify(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(h, "pbkdf2$sha256$") {
		t.Fatalf("unexpected hash format: %q", h)
	}
	if !VerifyPassword(h, "correct horse battery staple") {
		t.Fatal("correct password should verify")
	}
	if VerifyPassword(h, "wrong") {
		t.Fatal("wrong password must not verify")
	}
	// Two hashes of the same password differ (random salt).
	h2, _ := HashPassword("correct horse battery staple")
	if h == h2 {
		t.Fatal("hashes should differ due to random salt")
	}
	if VerifyPassword("garbage", "x") {
		t.Fatal("malformed hash must not verify")
	}
}

func newUserStore(t *testing.T) *history.Store {
	t.Helper()
	s, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSeedAdminUserOnceThenLoginSession(t *testing.T) {
	store := newUserStore(t)
	if err := SeedAdminUser(store, "anna", "hunter2hunter"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Idempotent: a second seed with different creds must not overwrite.
	_ = SeedAdminUser(store, "anna", "different")
	if u, ok, _ := store.GetAdminUser("anna"); !ok || !VerifyPassword(u.PasswordHash, "hunter2hunter") {
		t.Fatal("seed must not be overwritten by a second seed")
	}

	a := &AdminAuth{Sessions: NewSessionStore(nil), Store: store, BreakGlass: "bg-secret"}

	// Login with the named user sets a session cookie.
	form := url.Values{"username": {"anna"}, "password": {"hunter2hunter"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/router/login?next=/router/dashboard", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.LoginHandler()(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login status=%d body=%s", rec.Code, rec.Body.String())
	}
	cookie := rec.Result().Cookies()
	if len(cookie) == 0 || cookie[0].Name != sessionCookie {
		t.Fatal("login should set a session cookie")
	}

	// The session reaches a guarded handler and carries the user for audit.
	var gotUser string
	guarded := a.Require(func(w http.ResponseWriter, r *http.Request) {
		gotUser = AdminUserFromContext(r.Context())
		w.WriteHeader(200)
	})
	greq := httptest.NewRequest(http.MethodGet, "/router/dashboard", nil)
	greq.AddCookie(cookie[0])
	grec := httptest.NewRecorder()
	guarded.ServeHTTP(grec, greq)
	if grec.Code != 200 || gotUser != "anna" {
		t.Fatalf("guarded status=%d user=%q, want 200/anna", grec.Code, gotUser)
	}
}

func TestGuardRedirectsWithoutSession(t *testing.T) {
	a := &AdminAuth{Sessions: NewSessionStore(nil), Store: newUserStore(t)}
	guarded := a.Require(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/router/keys", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("unauthenticated GET should redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/router/login") {
		t.Fatalf("redirect to %q, want /router/login", loc)
	}
}

func TestBreakGlassLoginWorksWithoutUser(t *testing.T) {
	a := &AdminAuth{Sessions: NewSessionStore(nil), Store: newUserStore(t), BreakGlass: "bg-secret"}
	role, ok := a.authenticate("whoever", "bg-secret")
	if !ok || role != "admin" {
		t.Fatalf("break-glass should authenticate as admin, got %q/%v", role, ok)
	}
	if _, ok := a.authenticate("whoever", "nope"); ok {
		t.Fatal("wrong break-glass password must fail")
	}
}

func TestDisabledUserCannotLogin(t *testing.T) {
	store := newUserStore(t)
	_ = SeedAdminUser(store, "bob", "password123")
	_ = store.SetAdminUserDisabled("bob", time.Now())
	a := &AdminAuth{Sessions: NewSessionStore(nil), Store: store}
	if _, ok := a.authenticate("bob", "password123"); ok {
		t.Fatal("disabled user must not authenticate")
	}
}

// TestSessionSurvivesRestart is the ISSUE-108 regression: a session created
// against one store must still resolve after a "redeploy" — a fresh
// SessionStore built from the same SQLite file (as happens when the process
// restarts and reopens ROUTER_DATA_DIR).
func TestSessionSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "h.db")

	s1, err := history.Open(dbPath)
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	store1 := NewSessionStore(s1)
	id, err := store1.create("magnus", "admin")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, ok := store1.get(id); !ok {
		t.Fatal("session missing from live store")
	}
	s1.Close() // simulate process exit

	// Redeploy: new process, new store, same data dir.
	s2, err := history.Open(dbPath)
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	t.Cleanup(func() { s2.Close() })
	store2 := NewSessionStore(s2)
	sess, ok := store2.get(id)
	if !ok {
		t.Fatal("session did not survive restart — user would be logged out")
	}
	if sess.user != "magnus" || sess.role != "admin" {
		t.Fatalf("restored session wrong: %+v", sess)
	}

	// Logout must also persist: after delete, a fresh store must not see it.
	store2.delete(id)
	s2.Close()
	s3, err := history.Open(dbPath)
	if err != nil {
		t.Fatalf("open 3: %v", err)
	}
	t.Cleanup(func() { s3.Close() })
	if _, ok := NewSessionStore(s3).get(id); ok {
		t.Fatal("deleted session reappeared after restart — logout not persisted")
	}
}
