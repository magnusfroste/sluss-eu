package server

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// fakeIdP is a minimal OpenID provider: discovery, a token endpoint that
// returns an ID token signed with its RSA key, and a JWKS. The authorization
// endpoint is never visited (tests call the callback directly with a code),
// but the token endpoint records what it received so PKCE can be asserted.
type fakeIdP struct {
	srv         *httptest.Server
	key         *rsa.PrivateKey
	kid         string
	claims      func() map[string]any // claims to put in the next ID token
	signer      *rsa.PrivateKey       // key to sign with (defaults to key)
	gotVerifier string
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIdP{key: key, kid: "k1"}
	f.signer = key
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 f.srv.URL,
			"authorization_endpoint": f.srv.URL + "/authorize",
			"token_endpoint":         f.srv.URL + "/token",
			"jwks_uri":               f.srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		pub := f.key.PublicKey
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": f.kid, "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.gotVerifier = r.FormValue("code_verifier")
		if r.FormValue("grant_type") != "authorization_code" || r.FormValue("code") == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": f.sign(f.claims()), "token_type": "Bearer"})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIdP) sign(claims map[string]any) string {
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	signed := enc(map[string]string{"alg": "RS256", "kid": f.kid, "typ": "JWT"}) + "." + enc(claims)
	sum := sha256.Sum256([]byte(signed))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, f.signer, crypto.SHA256, sum[:])
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// baseClaims is a valid token for client "sluss" with the given nonce.
func (f *fakeIdP) baseClaims(nonce string) map[string]any {
	return map[string]any{
		"iss": f.srv.URL, "aud": "sluss", "sub": "u1", "nonce": nonce,
		"exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Unix(),
		"email": "Anna@Example.eu", "name": "Anna Andersson",
	}
}

type oidcRig struct {
	idp   *fakeIdP
	oidc  *OIDC
	auth  *AdminAuth
	audit *audit.MemorySink
}

func newOIDCRig(t *testing.T, cfg OIDCConfig) *oidcRig {
	t.Helper()
	idp := newFakeIdP(t)
	cfg.Issuer, cfg.ClientID = idp.srv.URL, "sluss"
	sink := audit.NewMemorySink(0)
	a := &AdminAuth{Sessions: NewSessionStore(nil), Store: newUserStore(t), Auditor: sink}
	o := NewOIDC(cfg, a, sink, "", nil)
	a.SSO = o.Button()
	return &oidcRig{idp: idp, oidc: o, auth: a, audit: sink}
}

// start runs /router/oidc/start and returns the state and nonce it minted,
// extracted from the redirect to the IdP.
func (rg *oidcRig) start(t *testing.T, next string) (state, nonce string) {
	t.Helper()
	rec := httptest.NewRecorder()
	rg.oidc.StartHandler()(rec, httptest.NewRequest(http.MethodGet, oidcStartPath+"?next="+url.QueryEscape(next), nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("start: status %d body %s", rec.Code, rec.Body.String())
	}
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil || !strings.HasPrefix(u.String(), rg.idp.srv.URL+"/authorize") {
		t.Fatalf("start: redirect %q, want the IdP's authorize endpoint", rec.Header().Get("Location"))
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("client_id") != "sluss" {
		t.Fatalf("start: PKCE/client missing in %q", u.String())
	}
	return q.Get("state"), q.Get("nonce")
}

// callback runs /router/oidc/callback with a code for the given state.
func (rg *oidcRig) callback(t *testing.T, state string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	rg.oidc.CallbackHandler()(rec, httptest.NewRequest(http.MethodGet, oidcCallbackPath+"?code=c0de&state="+url.QueryEscape(state), nil))
	return rec
}

func sessionCookieOf(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c
		}
	}
	t.Fatalf("no session cookie set; status %d body %s", rec.Code, rec.Body.String())
	return nil
}

func TestOIDCAdminGroupGetsConsoleAndDepartment(t *testing.T) {
	rg := newOIDCRig(t, OIDCConfig{
		AdminGroups: []string{"sec-admins"},
		Departments: map[string]string{"grp-security": "security"},
	})
	state, nonce := rg.start(t, "/router/dashboard")
	rg.idp.claims = func() map[string]any {
		c := rg.idp.baseClaims(nonce)
		c["groups"] = []any{"grp-security", "sec-admins"}
		return c
	}
	rec := rg.callback(t, state)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/router/dashboard" {
		t.Fatalf("callback: status %d location %q body %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	if rg.idp.gotVerifier == "" {
		t.Fatal("token endpoint did not receive a PKCE code_verifier")
	}
	cookie := sessionCookieOf(t, rec)

	var user, dept, role string
	guarded := rg.auth.Require(func(w http.ResponseWriter, r *http.Request) {
		user, dept, role = AdminUserFromContext(r.Context()), DepartmentFromContext(r.Context()), RoleFromContext(r.Context())
	})
	req := httptest.NewRequest(http.MethodGet, "/router/dashboard", nil)
	req.AddCookie(cookie)
	grec := httptest.NewRecorder()
	guarded.ServeHTTP(grec, req)
	if grec.Code != http.StatusOK {
		t.Fatalf("admin-group user should reach the console, got %d", grec.Code)
	}
	if user != "anna@example.eu" || dept != "security" || role != roleAdmin {
		t.Fatalf("session = %q/%q/%q, want anna@example.eu/security/admin", user, dept, role)
	}

	// The login is evidence: one console.login entry with method oidc.
	var found bool
	for _, e := range rg.audit.Entries() {
		if e.Action == ActionConsoleLogin && e.Actor == "anna@example.eu" && e.Detail["method"] == "oidc" && e.Detail["department"] == "security" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a console.login audit entry, got %+v", rg.audit.Entries())
	}
}

func TestOIDCEmployeeIsChatOnlyAndAttributed(t *testing.T) {
	rg := newOIDCRig(t, OIDCConfig{
		AdminGroups: []string{"sec-admins"},
		Departments: map[string]string{"grp-finance": "finance"},
	})
	state, nonce := rg.start(t, "/router/dashboard") // asks for the console...
	rg.idp.claims = func() map[string]any {
		c := rg.idp.baseClaims(nonce)
		c["groups"] = []any{"grp-finance"}
		return c
	}
	rec := rg.callback(t, state)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/chat" {
		t.Fatalf("employee should land in /chat, got %d → %q", rec.Code, rec.Header().Get("Location"))
	}
	cookie := sessionCookieOf(t, rec)

	// ...but the console refuses an employee session.
	req := httptest.NewRequest(http.MethodGet, "/router/dashboard", nil)
	req.AddCookie(cookie)
	grec := httptest.NewRecorder()
	rg.auth.Require(func(w http.ResponseWriter, r *http.Request) { t.Fatal("employee must not reach the console") }).ServeHTTP(grec, req)
	if grec.Code != http.StatusFound || grec.Header().Get("Location") != "/chat" {
		t.Fatalf("console should bounce an employee to /chat, got %d → %q", grec.Code, grec.Header().Get("Location"))
	}

	// The chat admits them, and the send endpoint attributes traffic to the
	// department with the person on the key id — never the demo tenant.
	var got *tenant.Tenant
	var key string
	chat := rg.auth.RequireDemo(demoTenantInjector(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = tenant.FromContext(r.Context())
		key = chatSessionsKey(r)
	})).ServeHTTP)
	creq := httptest.NewRequest(http.MethodPost, "/chat/send", nil)
	creq.AddCookie(cookie)
	crec := httptest.NewRecorder()
	chat.ServeHTTP(crec, creq)
	if crec.Code != http.StatusOK || got == nil {
		t.Fatalf("chat should admit an employee, got %d", crec.Code)
	}
	if got.ID != "finance" || got.KeyID != "user:anna@example.eu" {
		t.Fatalf("tenant = %+v, want finance / user:anna@example.eu", got)
	}
	if key != demoSessionsKey+":anna@example.eu" {
		t.Fatalf("chat sessions key = %q, want a per-user key", key)
	}
}

func TestOIDCDefaultDepartmentWhenNoGroupMaps(t *testing.T) {
	rg := newOIDCRig(t, OIDCConfig{})
	state, nonce := rg.start(t, "/chat")
	rg.idp.claims = func() map[string]any { return rg.idp.baseClaims(nonce) }
	cookie := sessionCookieOf(t, rg.callback(t, state))
	var dept string
	req := httptest.NewRequest(http.MethodGet, "/chat", nil)
	req.AddCookie(cookie)
	rg.auth.RequireDemo(func(w http.ResponseWriter, r *http.Request) { dept = DepartmentFromContext(r.Context()) }).ServeHTTP(httptest.NewRecorder(), req)
	if dept != "sso" {
		t.Fatalf("department = %q, want the default \"sso\"", dept)
	}
}

func TestOIDCRefusesBadSignature(t *testing.T) {
	rg := newOIDCRig(t, OIDCConfig{})
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	rg.idp.signer = other // signed by a key the JWKS does not publish
	state, nonce := rg.start(t, "/chat")
	rg.idp.claims = func() map[string]any { return rg.idp.baseClaims(nonce) }
	rec := rg.callback(t, state)
	assertRefused(t, rec, rg)
}

func TestOIDCRefusesNonceMismatch(t *testing.T) {
	rg := newOIDCRig(t, OIDCConfig{})
	state, _ := rg.start(t, "/chat")
	rg.idp.claims = func() map[string]any { return rg.idp.baseClaims("replayed-nonce") }
	assertRefused(t, rg.callback(t, state), rg)
}

func TestOIDCRefusesWrongAudienceAndExpired(t *testing.T) {
	rg := newOIDCRig(t, OIDCConfig{})
	state, nonce := rg.start(t, "/chat")
	rg.idp.claims = func() map[string]any {
		c := rg.idp.baseClaims(nonce)
		c["aud"] = "someone-else"
		return c
	}
	assertRefused(t, rg.callback(t, state), rg)

	state, nonce = rg.start(t, "/chat")
	rg.idp.claims = func() map[string]any {
		c := rg.idp.baseClaims(nonce)
		c["exp"] = time.Now().Add(-10 * time.Minute).Unix()
		return c
	}
	assertRefused(t, rg.callback(t, state), rg)
}

func TestOIDCRefusesDomainNotAllowed(t *testing.T) {
	rg := newOIDCRig(t, OIDCConfig{AllowedDomains: []string{"corp.se"}})
	state, nonce := rg.start(t, "/chat")
	rg.idp.claims = func() map[string]any { return rg.idp.baseClaims(nonce) } // example.eu
	rec := rg.callback(t, state)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "not allowed") {
		t.Fatalf("expected the login page with a domain refusal, got %d %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			t.Fatal("a refused login must not set a session")
		}
	}
}

func TestOIDCRefusesUnknownOrReusedState(t *testing.T) {
	rg := newOIDCRig(t, OIDCConfig{})
	rec := rg.callback(t, "never-issued")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "expired or was not started here") {
		t.Fatalf("unknown state should re-render the login with an error, got %d", rec.Code)
	}
	state, nonce := rg.start(t, "/chat")
	rg.idp.claims = func() map[string]any { return rg.idp.baseClaims(nonce) }
	sessionCookieOf(t, rg.callback(t, state))
	// The same state a second time is a replay.
	if rec := rg.callback(t, state); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "expired or was not started here") {
		t.Fatalf("reused state must be refused, got %d", rec.Code)
	}
}

func TestLoginPageShowsSSOButtonOnlyWhenConfigured(t *testing.T) {
	a := &AdminAuth{Sessions: NewSessionStore(nil), Store: newUserStore(t)}
	rec := httptest.NewRecorder()
	a.LoginHandler()(rec, httptest.NewRequest(http.MethodGet, "/router/login", nil))
	if strings.Contains(rec.Body.String(), "Sign in with") {
		t.Fatal("no SSO configured: the button must not render")
	}
	a.SSO = &SSOButton{Label: "Entra ID", StartPath: oidcStartPath}
	rec = httptest.NewRecorder()
	a.LoginHandler()(rec, httptest.NewRequest(http.MethodGet, "/router/login?next=/chat", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "Sign in with Entra ID") || !strings.Contains(body, oidcStartPath+"?next=") {
		t.Fatalf("SSO button missing from login page:\n%s", body)
	}
}

func assertRefused(t *testing.T, rec *httptest.ResponseRecorder, rg *oidcRig) {
	t.Helper()
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "could not be verified") {
		t.Fatalf("expected the login page with a verification error, got %d %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			t.Fatal("a refused token must not set a session")
		}
	}
	var failed bool
	for _, e := range rg.audit.Entries() {
		if e.Action == ActionConsoleLogin && e.Outcome == audit.OutcomeFailure {
			failed = true
		}
	}
	if !failed {
		t.Fatal("a refused login should leave a failed console.login audit entry")
	}
}

func TestParseDepartmentMapAndSplitList(t *testing.T) {
	m := ParseDepartmentMap(" grp-a = finance , grp-b=hr,bad, =x,y= ")
	if len(m) != 2 || m["grp-a"] != "finance" || m["grp-b"] != "hr" {
		t.Fatalf("ParseDepartmentMap = %v", m)
	}
	if l := SplitList(" a, ,b "); len(l) != 2 || l[0] != "a" || l[1] != "b" {
		t.Fatalf("SplitList = %v", l)
	}
}
