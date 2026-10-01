package server

// Named admin users: individual console logins so the audit trail attributes
// console actions to a PERSON, not a shared password — the accountability the
// NIS2 pitch needs. Passwords are hashed with PBKDF2-HMAC-SHA256 (stdlib, no new
// dependency; a NIST-listed KDF), stored in a self-describing string that can be
// swapped for bcrypt/argon2 later. Sessions are opaque random cookies held in
// memory. The env dashboard password remains a break-glass login so an operator
// can never be locked out.

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/history"
)

const (
	sessionCookie   = "tk_session"
	sessionTTL      = 12 * time.Hour
	pbkdf2Iter      = 210_000 // OWASP 2023 guidance for PBKDF2-HMAC-SHA256
	pbkdf2KeyLen    = 32
	breakGlassActor = "break-glass"

	// Session roles. roleDemo is the least-privileged role handed to a visitor who
	// arrives via the shareable demo link: it may use the /chat demo routes only,
	// never an admin page.
	roleAdmin = "admin"
	roleDemo  = "demo"
)

// --- password KDF (PBKDF2-HMAC-SHA256, stdlib) ---------------------------------

// HashPassword returns a self-describing hash: "pbkdf2$sha256$<iter>$<salt>$<dk>"
// (salt and derived key base64). No external dependency.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk := pbkdf2SHA256([]byte(plain), salt, pbkdf2Iter, pbkdf2KeyLen)
	return fmt.Sprintf("pbkdf2$sha256$%d$%s$%s", pbkdf2Iter,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk)), nil
}

// VerifyPassword checks a plaintext against a stored hash in constant time.
func VerifyPassword(stored, plain string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 5 || parts[0] != "pbkdf2" || parts[1] != "sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[2])
	if err != nil || iter < 1 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	got := pbkdf2SHA256([]byte(plain), salt, iter, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// pbkdf2SHA256 is a minimal RFC 2898 PBKDF2 with HMAC-SHA256 (stdlib only).
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	h := sha256.New
	hashLen := sha256.Size
	numBlocks := (keyLen + hashLen - 1) / hashLen
	var dk []byte
	buf := make([]byte, 4)
	for block := 1; block <= numBlocks; block++ {
		prf := hmac.New(h, password)
		prf.Write(salt)
		buf[0] = byte(block >> 24)
		buf[1] = byte(block >> 16)
		buf[2] = byte(block >> 8)
		buf[3] = byte(block)
		prf.Write(buf)
		u := prf.Sum(nil)
		t := make([]byte, len(u))
		copy(t, u)
		for n := 2; n <= iter; n++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for i := range t {
				t[i] ^= u[i]
			}
		}
		dk = append(dk, t...)
	}
	return dk[:keyLen]
}

// --- sessions ------------------------------------------------------------------

type session struct {
	user    string
	role    string
	expires time.Time
}

// SessionPersister persists console sessions so they survive a redeploy
// (ISSUE-108). Implemented by *history.Store; nil in tests keeps sessions
// purely in-memory. The request path never calls this — the DB is loaded into
// the cache at boot and written through on create/delete only.
type SessionPersister interface {
	InsertSession(history.Session) error
	DeleteSession(id string) error
	ActiveSessions() ([]history.Session, error)
}

// SessionStore is a concurrency-safe session table: an in-memory cache on the
// fast path, backed by a SessionPersister so sessions outlive process restarts.
type SessionStore struct {
	mu    sync.Mutex
	m     map[string]session
	store SessionPersister
}

// NewSessionStore builds the store and, when persistence is available, rebuilds
// the cache from any sessions that survived the last restart.
func NewSessionStore(p SessionPersister) *SessionStore {
	s := &SessionStore{m: map[string]session{}, store: p}
	if p != nil {
		if rows, err := p.ActiveSessions(); err == nil {
			for _, r := range rows {
				s.m[r.ID] = session{user: r.Username, role: r.Role, expires: r.ExpiresAt}
			}
		}
	}
	return s
}

func (s *SessionStore) create(user, role string) (string, error) {
	tok := make([]byte, 24)
	if _, err := rand.Read(tok); err != nil {
		return "", err
	}
	id := base64.RawURLEncoding.EncodeToString(tok)
	exp := time.Now().Add(sessionTTL)
	s.mu.Lock()
	s.m[id] = session{user: user, role: role, expires: exp}
	store := s.store
	s.mu.Unlock()
	if store != nil {
		if err := store.InsertSession(history.Session{ID: id, Username: user, Role: role, ExpiresAt: exp}); err != nil {
			return "", err
		}
	}
	return id, nil
}

func (s *SessionStore) get(id string) (session, bool) {
	s.mu.Lock()
	v, ok := s.m[id]
	if !ok {
		s.mu.Unlock()
		return session{}, false
	}
	if time.Now().After(v.expires) {
		delete(s.m, id)
		store := s.store
		s.mu.Unlock()
		if store != nil {
			_ = store.DeleteSession(id)
		}
		return session{}, false
	}
	s.mu.Unlock()
	return v, true
}

func (s *SessionStore) delete(id string) {
	s.mu.Lock()
	delete(s.m, id)
	store := s.store
	s.mu.Unlock()
	if store != nil {
		_ = store.DeleteSession(id)
	}
}

// --- request context: the authenticated console user ---------------------------

type ctxKeyAdminUser struct{}
type ctxKeyRole struct{}

func withAdminUser(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, ctxKeyAdminUser{}, user)
}

func withRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, ctxKeyRole{}, role)
}

// AdminUserFromContext returns the console user for audit attribution, or
// "admin" when none is set (legacy / no-user deployments).
func AdminUserFromContext(ctx context.Context) string {
	if u, ok := ctx.Value(ctxKeyAdminUser{}).(string); ok && u != "" {
		return u
	}
	return "admin"
}

// RoleFromContext returns the authenticated session role (e.g. "admin", "demo",
// "api"), or "" when unknown.
func RoleFromContext(ctx context.Context) string {
	if r, ok := ctx.Value(ctxKeyRole{}).(string); ok {
		return r
	}
	return ""
}

// grant serves next with both the user and role attached to the context.
func grant(next http.HandlerFunc, w http.ResponseWriter, r *http.Request, user, role string) {
	ctx := withRole(withAdminUser(r.Context(), user), role)
	next.ServeHTTP(w, r.WithContext(ctx))
}

// SeedAdminUser creates an initial admin account from env credentials when the
// user table is empty, so a deployment can bootstrap named users without the
// break-glass password. No-op when a user already exists or creds are blank.
func SeedAdminUser(store *history.Store, username, password string) error {
	if store == nil || username == "" || password == "" || store.CountAdminUsers() > 0 {
		return nil
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return store.UpsertAdminUser(history.AdminUser{
		Username: username, PasswordHash: hash, Role: "admin", CreatedAt: time.Now(),
	})
}

// --- the guard -----------------------------------------------------------------

// AdminAuth gates the admin console with named-user sessions, with the env
// dashboard password as a break-glass login and Bearer admin keys for API use.
type AdminAuth struct {
	Sessions   *SessionStore
	Store      *history.Store // user accounts; nil disables named users
	BreakGlass string         // env dashboard password; "" disables break-glass
	KeyStore   auth.KeyStore  // for programmatic Bearer admin access (API clients)
}

// Require wraps a handler so only an authenticated admin reaches it. Order:
// valid session cookie → serve (user in context); valid Bearer admin key →
// serve (programmatic API access); otherwise redirect a browser to the login
// page (or 401 for non-GET). A least-privileged demo-link session is explicitly
// rejected here — a demo visitor must never reach an admin page.
func (a *AdminAuth) Require(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			if s, ok := a.Sessions.get(c.Value); ok {
				if s.role == roleDemo {
					// Demo guests belong in the demo, not the admin console.
					if r.Method == http.MethodGet {
						http.Redirect(w, r, "/chat", http.StatusFound)
						return
					}
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				grant(next, w, r, s.user, s.role)
				return
			}
		}
		if a.bearerAdmin(r) {
			grant(next, w, r, "api", "api")
			return
		}
		if r.Method == http.MethodGet {
			http.Redirect(w, r, "/router/login?next="+urlQueryEscape(r.URL.Path), http.StatusFound)
			return
		}
		http.Error(w, "authentication required", http.StatusUnauthorized)
	})
}

// RequireDemo gates the live demo chat. A full admin session, a shareable
// demo-link session, or a Bearer admin key all pass. It differs from Require
// only by also admitting the least-privileged demo role — and only the /chat
// routes use it, so a demo visitor is confined to the demo.
func (a *AdminAuth) RequireDemo(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			if s, ok := a.Sessions.get(c.Value); ok {
				grant(next, w, r, s.user, s.role)
				return
			}
		}
		if a.bearerAdmin(r) {
			grant(next, w, r, "api", "api")
			return
		}
		if r.Method == http.MethodGet {
			http.Redirect(w, r, "/router/login?next="+urlQueryEscape(r.URL.Path), http.StatusFound)
			return
		}
		http.Error(w, "authentication required", http.StatusUnauthorized)
	})
}

// issueSession creates a session and sets the cookie. Shared by password login
// and the shareable demo link.
func (a *AdminAuth) issueSession(w http.ResponseWriter, r *http.Request, user, role string) error {
	id, err := a.Sessions.create(user, role)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: id, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: requestIsHTTPS(r), Expires: time.Now().Add(sessionTTL),
	})
	return nil
}

// requestIsHTTPS reports whether the client reached us over HTTPS, honouring a
// terminating reverse proxy. Behind Cloudflare Tunnel the origin sees plain
// HTTP (r.TLS is nil) even though the browser leg is HTTPS, so we also trust
// X-Forwarded-Proto — otherwise the session cookie would never carry Secure.
func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// bearerAdmin reports whether the request carries a valid Bearer key whose
// tenant has the admin role — preserving programmatic console access.
func (a *AdminAuth) bearerAdmin(r *http.Request) bool {
	if a.KeyStore == nil {
		return false
	}
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if !strings.HasPrefix(h, p) {
		return false
	}
	t, ok := a.KeyStore.Lookup(auth.HashKey(strings.TrimSpace(h[len(p):])))
	return ok && t.HasRole(auth.RoleAdmin)
}

// authenticate validates a username/password against the DB users first, then
// the break-glass password. Returns (role, ok).
func (a *AdminAuth) authenticate(username, password string) (string, bool) {
	if a.Store != nil {
		if u, ok, _ := a.Store.GetAdminUser(username); ok && !u.Disabled() {
			if VerifyPassword(u.PasswordHash, password) {
				a.Store.TouchAdminUserLogin(username, time.Now())
				role := u.Role
				if role == "" {
					role = "admin"
				}
				return role, true
			}
			return "", false
		}
	}
	// Break-glass: the env dashboard password logs in as the built-in admin.
	if a.BreakGlass != "" && subtle.ConstantTimeCompare([]byte(password), []byte(a.BreakGlass)) == 1 {
		return "admin", true
	}
	return "", false
}

// LoginHandler renders the login form (GET) and processes it (POST).
func (a *AdminAuth) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next := r.URL.Query().Get("next")
		if next == "" || !strings.HasPrefix(next, "/") {
			next = "/router/dashboard"
		}
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			user := strings.TrimSpace(r.FormValue("username"))
			pass := r.FormValue("password")
			role, ok := a.authenticate(user, pass)
			if !ok {
				renderLogin(w, next, "Wrong username or password.")
				return
			}
			actor := user
			if a.Store == nil || func() bool { _, found, _ := a.Store.GetAdminUser(user); return !found }() {
				actor = user + " (" + breakGlassActor + ")"
			}
			if err := a.issueSession(w, r, actor, role); err != nil {
				renderLogin(w, next, "Could not create a session.")
				return
			}
			http.Redirect(w, r, next, http.StatusSeeOther)
			return
		}
		renderLogin(w, next, "")
	}
}

// LogoutHandler clears the session.
func (a *AdminAuth) LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			a.Sessions.delete(c.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
		http.Redirect(w, r, "/router/login", http.StatusSeeOther)
	}
}

func renderLogin(w http.ResponseWriter, next, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var errHTML string
	if errMsg != "" {
		errHTML = `<div class="err">` + template_htmlEscape(errMsg) + `</div>`
	}
	_, _ = w.Write([]byte(strings.NewReplacer(
		"{{NEXT}}", template_htmlEscape(next),
		"{{ERR}}", errHTML,
		"{{FAVICON}}", faviconLinks,
		"{{LOGO}}", brandLogoSVG,
	).Replace(loginHTML)))
}

// template_htmlEscape is a tiny HTML-attribute/text escaper for the login page
// (which is built with a string replacer, not html/template).
func template_htmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;").Replace(s)
}

const loginHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Sign in</title>
{{FAVICON}}
<style>
:root{--bg:#081328;--panel:#101c34;--line:rgba(255,255,255,.09);--ink:#eaeff5;--muted:#9aa5b8;--accent:#fad100}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font-family:"IBM Plex Sans",ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;
  background-image:radial-gradient(60% 50% at 70% 0%,rgba(250,209,0,.10),transparent 70%);
  display:flex;min-height:100vh;align-items:center;justify-content:center}
.box{width:340px;background:var(--panel);border:1px solid var(--line);border-radius:12px;padding:28px 26px}
.brand{display:flex;align-items:center;gap:9px;font-weight:600;margin-bottom:20px}
label{display:block;font-size:12px;color:var(--muted);margin:14px 0 5px;letter-spacing:.02em}
input{width:100%;background:var(--bg);border:1px solid var(--line);border-radius:8px;color:var(--ink);padding:10px 12px;font-size:14px}
input:focus{outline:none;border-color:var(--accent)}
button{width:100%;margin-top:20px;background:var(--accent);color:#081328;border:0;border-radius:6px;padding:11px;font-weight:600;font-size:14px;cursor:pointer}
.err{background:#3a1620;border:1px solid #7a2b39;color:#f8b4bc;border-radius:8px;padding:9px 12px;font-size:13px;margin-bottom:6px}
.sub{color:#6b7891;font-size:12px;margin-top:16px;text-align:center}
</style></head>
<body>
<form class="box" method="post" action="/router/login?next={{NEXT}}">
  <div class="brand">{{LOGO}} Sluss</div>
  {{ERR}}
  <label>Username</label>
  <input name="username" autocomplete="username" autofocus>
  <label>Password</label>
  <input name="password" type="password" autocomplete="current-password">
  <button type="submit">Sign in</button>
  <div class="sub">Named accounts, or the break-glass password.</div>
</form>
</body></html>`
