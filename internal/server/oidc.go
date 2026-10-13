package server

// Sign in with the company account (ISSUE-130). Sluss is an OpenID Connect
// relying party against the customer's identity provider — Microsoft Entra ID,
// Okta, Google Workspace, Keycloak — and keeps no accounts of its own: no
// passwords, no user table for employees, nothing to offboard. Who may sign in,
// MFA, device compliance (Conditional Access) and leaving the company are all
// the IdP's job. What Sluss adds is attribution: the session carries the
// person's email and the department their group maps to, and every chat
// request they send is logged under that department with the user on the key
// id — the identified-user trail a CISO asks for.
//
// Standard library only: discovery, authorization code + PKCE, the token
// endpoint, and RS256 ID-token verification against the issuer's JWKS. The
// flow fails closed: any doubt about the token is a refused login, never a
// guest session.

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
)

// OIDCConfig is read from ROUTER_OIDC_* in main. Issuer and ClientID enable it.
type OIDCConfig struct {
	Issuer       string // e.g. https://login.microsoftonline.com/<tenant>/v2.0
	ClientID     string
	ClientSecret string // empty = public client (PKCE only)
	// Scopes requested; default "openid profile email". Okta needs "groups"
	// added; Entra sends groups as an optional claim configured in the app.
	Scopes string
	// GroupsClaim is the ID-token claim listing the user's groups ("groups").
	GroupsClaim string
	// AdminGroups: members get the admin console. Everyone else is a user
	// (chat only).
	AdminGroups []string
	// Departments maps a group to the department (tenant) chat is attributed
	// to, e.g. {"grp-finance": "finance"}. First match in claim order wins.
	Departments map[string]string
	// DefaultDepartment is used when no group maps; "sso" when empty, so an
	// identified user is never confused with the demo tenant.
	DefaultDepartment string
	// AllowedDomains, when set, restricts sign-in to these email domains
	// (useful with Google Workspace, whose ID tokens carry no groups).
	AllowedDomains []string
	// Label is what the login button says: "Sign in with <Label>".
	Label string
}

// Enabled reports whether SSO is configured.
func (c *OIDCConfig) Enabled() bool {
	return c != nil && strings.TrimSpace(c.Issuer) != "" && strings.TrimSpace(c.ClientID) != ""
}

// OIDC is the relying party. One per process.
type OIDC struct {
	cfg       OIDCConfig
	auth      *AdminAuth
	auditor   audit.Sink
	publicURL string
	logger    *slog.Logger
	client    *http.Client

	mu      sync.Mutex
	disc    *oidcDiscovery
	discAt  time.Time
	keys    map[string]*rsa.PublicKey
	keysAt  time.Time
	pending map[string]oidcPending // state → login in progress
	nowFunc func() time.Time
}

type oidcDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type oidcPending struct {
	nonce, verifier, next string
	expires               time.Time
}

const (
	oidcStartPath    = "/router/oidc/start"
	oidcCallbackPath = "/router/oidc/callback"
	oidcPendingTTL   = 10 * time.Minute
	oidcDiscoveryTTL = 24 * time.Hour
	oidcJWKSTTL      = 6 * time.Hour
)

// NewOIDC builds the relying party. Discovery and keys are fetched lazily on
// the first login, so a boot with an unreachable IdP still starts the router;
// the login then fails closed with a clear message.
func NewOIDC(cfg OIDCConfig, a *AdminAuth, auditor audit.Sink, publicURL string, logger *slog.Logger) *OIDC {
	if strings.TrimSpace(cfg.Scopes) == "" {
		cfg.Scopes = "openid profile email"
	}
	if strings.TrimSpace(cfg.GroupsClaim) == "" {
		cfg.GroupsClaim = "groups"
	}
	if strings.TrimSpace(cfg.DefaultDepartment) == "" {
		cfg.DefaultDepartment = "sso"
	}
	if strings.TrimSpace(cfg.Label) == "" {
		cfg.Label = "company account"
	}
	cfg.Issuer = strings.TrimRight(strings.TrimSpace(cfg.Issuer), "/")
	if logger == nil {
		logger = slog.Default()
	}
	return &OIDC{
		cfg: cfg, auth: a, auditor: auditor, publicURL: strings.TrimRight(strings.TrimSpace(publicURL), "/"),
		logger: logger, client: &http.Client{Timeout: 10 * time.Second},
		keys: map[string]*rsa.PublicKey{}, pending: map[string]oidcPending{}, nowFunc: time.Now,
	}
}

// Button is what the login page shows.
func (o *OIDC) Button() *SSOButton { return &SSOButton{Label: o.cfg.Label, StartPath: oidcStartPath} }

// redirectURI is where the IdP sends the browser back. ROUTER_PUBLIC_URL when
// set (the IdP registration must match it exactly), else derived from the
// request behind the proxy.
func (o *OIDC) redirectURI(r *http.Request) string {
	if o.publicURL != "" {
		return o.publicURL + oidcCallbackPath
	}
	scheme := "http"
	if requestIsHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host + oidcCallbackPath
}

func (o *OIDC) discovery(ctx context.Context) (*oidcDiscovery, error) {
	o.mu.Lock()
	if o.disc != nil && o.nowFunc().Before(o.discAt.Add(oidcDiscoveryTTL)) {
		d := o.disc
		o.mu.Unlock()
		return d, nil
	}
	o.mu.Unlock()
	var d oidcDiscovery
	if err := o.getJSON(ctx, o.cfg.Issuer+"/.well-known/openid-configuration", &d); err != nil {
		return nil, fmt.Errorf("discovery: %w", err)
	}
	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" || d.JWKSURI == "" {
		return nil, errors.New("discovery: document lacks authorization, token or jwks endpoint")
	}
	o.mu.Lock()
	o.disc, o.discAt = &d, o.nowFunc()
	o.mu.Unlock()
	return &d, nil
}

func (o *OIDC) getJSON(ctx context.Context, u string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", u, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(into)
}

// key returns the RSA public key for kid, refreshing the JWKS once when the
// kid is unknown (key rotation) or the cache is stale.
func (o *OIDC) key(ctx context.Context, jwksURI, kid string) (*rsa.PublicKey, error) {
	o.mu.Lock()
	k, ok := o.keys[kid]
	fresh := o.nowFunc().Before(o.keysAt.Add(oidcJWKSTTL))
	o.mu.Unlock()
	if ok && fresh {
		return k, nil
	}
	var set struct {
		Keys []struct {
			Kty, Kid, Use, N, E string
		} `json:"keys"`
	}
	if err := o.getJSON(ctx, jwksURI, &set); err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, jk := range set.Keys {
		if jk.Kty != "RSA" || (jk.Use != "" && jk.Use != "sig") {
			continue
		}
		nb, err1 := base64.RawURLEncoding.DecodeString(jk.N)
		eb, err2 := base64.RawURLEncoding.DecodeString(jk.E)
		if err1 != nil || err2 != nil || len(nb) == 0 || len(eb) == 0 {
			continue
		}
		e := int(new(big.Int).SetBytes(eb).Int64())
		keys[jk.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}
	}
	o.mu.Lock()
	o.keys, o.keysAt = keys, o.nowFunc()
	k, ok = keys[kid]
	o.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("jwks: no RSA key with kid %q", kid)
	}
	return k, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// StartHandler begins the authorization-code flow: state, nonce and a PKCE
// verifier are kept server-side for ten minutes, then the browser is sent to
// the IdP.
func (o *OIDC) StartHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next := r.URL.Query().Get("next")
		if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
			next = "/chat"
		}
		d, err := o.discovery(r.Context())
		if err != nil {
			o.logger.Warn("oidc: discovery failed", "err", err)
			o.auth.renderLogin(w, next, "The identity provider could not be reached. Try again, or sign in as an operator.")
			return
		}
		state, err1 := randomToken(24)
		nonce, err2 := randomToken(24)
		verifier, err3 := randomToken(48)
		if err1 != nil || err2 != nil || err3 != nil {
			http.Error(w, "could not start sign-in", http.StatusInternalServerError)
			return
		}
		now := o.nowFunc()
		o.mu.Lock()
		for k, p := range o.pending { // sweep stale attempts
			if now.After(p.expires) {
				delete(o.pending, k)
			}
		}
		o.pending[state] = oidcPending{nonce: nonce, verifier: verifier, next: next, expires: now.Add(oidcPendingTTL)}
		o.mu.Unlock()

		sum := sha256.Sum256([]byte(verifier))
		q := url.Values{
			"response_type":         {"code"},
			"client_id":             {o.cfg.ClientID},
			"redirect_uri":          {o.redirectURI(r)},
			"scope":                 {o.cfg.Scopes},
			"state":                 {state},
			"nonce":                 {nonce},
			"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
			"code_challenge_method": {"S256"},
		}
		sep := "?"
		if strings.Contains(d.AuthorizationEndpoint, "?") {
			sep = "&"
		}
		http.Redirect(w, r, d.AuthorizationEndpoint+sep+q.Encode(), http.StatusFound)
	}
}

// CallbackHandler finishes the flow: code → tokens, verify the ID token,
// map groups to role and department, issue the session, audit the login.
func (o *OIDC) CallbackHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			o.logger.Warn("oidc: provider returned error", "error", e, "description", q.Get("error_description"))
			o.auth.renderLogin(w, "/chat", "Sign-in was refused by the identity provider ("+e+").")
			return
		}
		state, code := q.Get("state"), q.Get("code")
		o.mu.Lock()
		p, ok := o.pending[state]
		delete(o.pending, state)
		o.mu.Unlock()
		if !ok || code == "" || o.nowFunc().After(p.expires) {
			o.auth.renderLogin(w, "/chat", "Sign-in expired or was not started here. Please try again.")
			return
		}
		id, err := o.exchange(r, code, p.verifier)
		if err == nil {
			err = o.verify(r.Context(), id, p.nonce)
		}
		if err != nil {
			o.logger.Warn("oidc: token refused", "err", err)
			audit.Record(r.Context(), o.auditor, audit.Entry{
				Action: ActionConsoleLogin, Actor: "unknown", Outcome: audit.OutcomeFailure,
				Reason: err.Error(), Detail: map[string]string{"method": "oidc"},
			})
			o.auth.renderLogin(w, "/chat", "The sign-in token could not be verified. Please try again.")
			return
		}
		ident, err := o.identity(id.claims)
		if err != nil {
			audit.Record(r.Context(), o.auditor, audit.Entry{
				Action: ActionConsoleLogin, Actor: ident.email, Outcome: audit.OutcomeBlocked,
				Reason: err.Error(), Detail: map[string]string{"method": "oidc"},
			})
			o.auth.renderLogin(w, "/chat", err.Error())
			return
		}
		if err := o.auth.issueSession(w, r, ident.email, ident.role, ident.department); err != nil {
			http.Error(w, "could not create a session", http.StatusInternalServerError)
			return
		}
		audit.Record(r.Context(), o.auditor, audit.Entry{
			Action: ActionConsoleLogin, Actor: ident.email, TenantID: ident.department, Target: o.cfg.Issuer,
			Detail: map[string]string{"method": "oidc", "role": ident.role, "department": ident.department, "name": ident.name},
		})
		next := p.next
		if ident.role != roleAdmin && strings.HasPrefix(next, "/router/") && next != "/router/logout" {
			next = "/chat" // an employee asked for a console page: chat is what they get
		}
		http.Redirect(w, r, next, http.StatusSeeOther)
	}
}

type idToken struct {
	raw    string
	header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	claims map[string]any
	signed []byte // header.payload
	sig    []byte
}

// exchange posts the code to the token endpoint and parses the ID token.
func (o *OIDC) exchange(r *http.Request, code, verifier string) (*idToken, error) {
	d, err := o.discovery(r.Context())
	if err != nil {
		return nil, err
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {o.redirectURI(r)},
		"client_id":     {o.cfg.ClientID},
		"code_verifier": {verifier},
	}
	if o.cfg.ClientSecret != "" {
		form.Set("client_secret", o.cfg.ClientSecret)
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, d.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token endpoint: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint: HTTP %d", resp.StatusCode)
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.IDToken == "" {
		return nil, errors.New("token endpoint: no id_token in response")
	}
	return parseIDToken(tok.IDToken)
}

func parseIDToken(raw string) (*idToken, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("id_token: not a compact JWS")
	}
	t := &idToken{raw: raw, signed: []byte(parts[0] + "." + parts[1])}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("id_token: bad header encoding")
	}
	if err := json.Unmarshal(hb, &t.header); err != nil {
		return nil, errors.New("id_token: bad header")
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("id_token: bad payload encoding")
	}
	if err := json.Unmarshal(pb, &t.claims); err != nil {
		return nil, errors.New("id_token: bad payload")
	}
	if t.sig, err = base64.RawURLEncoding.DecodeString(parts[2]); err != nil {
		return nil, errors.New("id_token: bad signature encoding")
	}
	return t, nil
}

// verify checks signature (RS256 against the issuer's JWKS), issuer,
// audience, expiry and nonce. Anything else is a refusal.
func (o *OIDC) verify(ctx context.Context, t *idToken, nonce string) error {
	if t.header.Alg != "RS256" {
		return fmt.Errorf("id_token: unsupported alg %q (RS256 required)", t.header.Alg)
	}
	d, err := o.discovery(ctx)
	if err != nil {
		return err
	}
	pub, err := o.key(ctx, d.JWKSURI, t.header.Kid)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(t.signed)
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], t.sig); err != nil {
		return errors.New("id_token: signature does not verify")
	}
	if iss, _ := t.claims["iss"].(string); strings.TrimRight(iss, "/") != o.cfg.Issuer {
		return fmt.Errorf("id_token: issuer %q is not %q", iss, o.cfg.Issuer)
	}
	if !audienceContains(t.claims["aud"], o.cfg.ClientID) {
		return errors.New("id_token: audience is not this client")
	}
	exp, _ := t.claims["exp"].(float64)
	if exp == 0 || o.nowFunc().After(time.Unix(int64(exp), 0).Add(60*time.Second)) {
		return errors.New("id_token: expired")
	}
	if n, _ := t.claims["nonce"].(string); n == "" || n != nonce {
		return errors.New("id_token: nonce mismatch")
	}
	return nil
}

func audienceContains(aud any, clientID string) bool {
	switch v := aud.(type) {
	case string:
		return v == clientID
	case []any:
		for _, a := range v {
			if s, _ := a.(string); s == clientID {
				return true
			}
		}
	}
	return false
}

type oidcIdentity struct {
	email, name, role, department string
	groups                        []string
}

// identity turns verified claims into who the session is: email as the actor,
// groups → role and department, optional domain allow-list.
func (o *OIDC) identity(claims map[string]any) (oidcIdentity, error) {
	var id oidcIdentity
	for _, k := range []string{"email", "preferred_username", "upn"} {
		if v, _ := claims[k].(string); strings.Contains(v, "@") {
			id.email = strings.ToLower(strings.TrimSpace(v))
			break
		}
	}
	if id.email == "" {
		if sub, _ := claims["sub"].(string); sub != "" {
			id.email = "sub:" + sub
		} else {
			return id, errors.New("The identity provider sent no email or subject for this account.")
		}
	}
	id.name, _ = claims["name"].(string)
	if raw, ok := claims[o.cfg.GroupsClaim].([]any); ok {
		for _, g := range raw {
			if s, _ := g.(string); s != "" {
				id.groups = append(id.groups, s)
			}
		}
	}
	if len(o.cfg.AllowedDomains) > 0 {
		at := strings.LastIndex(id.email, "@")
		dom := ""
		if at >= 0 {
			dom = id.email[at+1:]
		}
		allowed := false
		for _, d := range o.cfg.AllowedDomains {
			if strings.EqualFold(strings.TrimSpace(d), dom) {
				allowed = true
				break
			}
		}
		if !allowed {
			return id, errors.New("This account's email domain is not allowed to sign in here.")
		}
	}
	id.role = roleUser
	for _, g := range id.groups {
		for _, a := range o.cfg.AdminGroups {
			if strings.EqualFold(strings.TrimSpace(a), g) {
				id.role = roleAdmin
			}
		}
	}
	id.department = o.cfg.DefaultDepartment
	for _, g := range id.groups {
		if d, ok := o.cfg.Departments[g]; ok && strings.TrimSpace(d) != "" {
			id.department = strings.TrimSpace(d)
			break
		}
	}
	return id, nil
}

// ParseDepartmentMap parses "group=dept,group2=dept2" from the environment.
func ParseDepartmentMap(s string) map[string]string {
	out := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if ok && strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

// SplitList splits a comma-separated env value, dropping blanks.
func SplitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
