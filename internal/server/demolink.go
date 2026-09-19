package server

// Shareable CISO demo link. An admin generates a secret token; the link
//   <public-url>/demo?token=<token>
// grants a least-privilege "demo" session (chat only, never an admin page) with
// no password prompt. A CISO clicks it, clicks through the quick prompts, and
// sees the value live — then you call them. The token lives in the SQLite KV so
// it can be rotated/disabled without a redeploy, and every access is audited.

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/history"
)

// demoShareTokenKey is the KV key holding the current demo share token ("" = off).
const demoShareTokenKey = "demo_share_token"

// KV keys for demo-link usage (the "did the prospect try it?" signal).
const (
	demoOpensKey    = "demo_link_opens"
	demoLastOpenKey = "demo_link_last_open"
)

// actionDemoAccess audits someone opening the shared demo link.
const actionDemoAccess = audit.Action("demo.link.access")

// LoadDemoShareToken returns the current token, or "" when the demo link is off.
func LoadDemoShareToken(h *history.Store) string {
	if h == nil {
		return ""
	}
	if v, ok := h.KVGet(demoShareTokenKey); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// SetDemoShareToken stores (or, with "", disables) the demo share token.
func SetDemoShareToken(h *history.Store, token string) error {
	if h == nil {
		return nil
	}
	return h.KVSet(demoShareTokenKey, strings.TrimSpace(token))
}

// DemoLinkUsage is the "did the prospect open the demo?" signal.
type DemoLinkUsage struct {
	Enabled  bool   `json:"enabled"`
	Opens    int    `json:"opens"`
	LastOpen string `json:"last_open,omitempty"` // RFC3339, empty = never
}

// incrementDemoOpens bumps the open counter and stamps the last-open time. A nil
// store is a gentle no-op (in-memory demo).
func incrementDemoOpens(h *history.Store) {
	if h == nil {
		return
	}
	n := 0
	if v, ok := h.KVGet(demoOpensKey); ok {
		n, _ = strconv.Atoi(strings.TrimSpace(v))
	}
	_ = h.KVSet(demoOpensKey, strconv.Itoa(n+1))
	_ = h.KVSet(demoLastOpenKey, time.Now().UTC().Format(time.RFC3339))
}

// LoadDemoLinkUsage reports whether the link is enabled and how often it was opened.
func LoadDemoLinkUsage(h *history.Store) DemoLinkUsage {
	u := DemoLinkUsage{Enabled: LoadDemoShareToken(h) != ""}
	if h == nil {
		return u
	}
	if v, ok := h.KVGet(demoOpensKey); ok {
		u.Opens, _ = strconv.Atoi(strings.TrimSpace(v))
	}
	if v, ok := h.KVGet(demoLastOpenKey); ok {
		u.LastOpen = strings.TrimSpace(v)
	}
	return u
}

// GenerateDemoShareToken returns a new URL-safe random token.
func GenerateDemoShareToken() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// DemoShareURL builds the full shareable link from the public base URL. When no
// public URL is configured it returns a relative link (fine for copy-paste in
// the same browser, and a clear hint to set ROUTER_PUBLIC_URL for a real link).
func DemoShareURL(publicURL, token string) string {
	if token == "" {
		return ""
	}
	base := strings.TrimRight(strings.TrimSpace(publicURL), "/")
	return base + "/demo?token=" + token
}

// DemoEntryHandler validates ?token= and, on a match, starts a least-privilege
// demo session and sends the visitor to /chat. Named links (ISSUE-089) are
// checked first — the session actor carries the link's label so the audit trail
// says WHICH prospect opened the demo; the legacy single share token still works.
func DemoEntryHandler(a *AdminAuth, h *history.Store, auditor audit.Sink) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimSpace(r.URL.Query().Get("token"))
		if got == "" {
			http.Redirect(w, r, "/router/login", http.StatusFound)
			return
		}
		// Named per-prospect link: validate, count the open, attribute by label.
		if label, ok := h.TouchDemoLink(got, time.Now()); ok {
			actor := "demo:" + label
			if err := a.issueSession(w, r, actor, roleDemo); err != nil {
				http.Error(w, "could not start the demo", http.StatusInternalServerError)
				return
			}
			audit.Record(r.Context(), auditor, audit.Entry{
				Action: actionDemoAccess, Actor: actor, Target: "named-link",
			})
			http.Redirect(w, r, "/chat", http.StatusSeeOther)
			return
		}
		// Legacy single shared token.
		want := LoadDemoShareToken(h)
		if want == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			// Fail closed — wrong/disabled links go to the normal login.
			http.Redirect(w, r, "/router/login", http.StatusFound)
			return
		}
		if err := a.issueSession(w, r, "demo-guest", roleDemo); err != nil {
			http.Error(w, "could not start the demo", http.StatusInternalServerError)
			return
		}
		incrementDemoOpens(h)
		audit.Record(r.Context(), auditor, audit.Entry{
			Action: actionDemoAccess, Actor: "demo-guest", Target: "shared-link",
		})
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
	}
}

// DemoLinksCreateHandler creates a named per-prospect link from the admin form.
func DemoLinksCreateHandler(h *history.Store, auditor audit.Sink) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil {
			redirectPrompts(w, r, "", "named links require ROUTER_DATA_DIR")
			return
		}
		_ = r.ParseForm()
		label := strings.TrimSpace(r.FormValue("label"))
		if label == "" {
			redirectPrompts(w, r, "", "a label is required (e.g. the prospect's company)")
			return
		}
		tok, err := GenerateDemoShareToken()
		if err != nil {
			redirectPrompts(w, r, "", "could not generate a token")
			return
		}
		if err := h.CreateDemoLink(tok, label, time.Now()); err != nil {
			redirectPrompts(w, r, "", "save: "+err.Error())
			return
		}
		audit.Record(r.Context(), auditor, audit.Entry{
			Action: actionDemoAccess, Actor: AdminUserFromContext(r.Context()),
			Target: "named-link", Reason: "created: " + label,
		})
		redirectPrompts(w, r, "Named demo link created for "+label+".", "")
	}
}

// DemoLinksDisableHandler revokes a named link by token.
func DemoLinksDisableHandler(h *history.Store, auditor audit.Sink) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil {
			redirectPrompts(w, r, "", "named links require ROUTER_DATA_DIR")
			return
		}
		_ = r.ParseForm()
		tok := strings.TrimSpace(r.FormValue("token"))
		if ok, err := h.DisableDemoLink(tok, time.Now()); err != nil || !ok {
			redirectPrompts(w, r, "", "could not disable the link")
			return
		}
		audit.Record(r.Context(), auditor, audit.Entry{
			Action: actionDemoAccess, Actor: AdminUserFromContext(r.Context()),
			Target: "named-link", Reason: "disabled",
		})
		redirectPrompts(w, r, "Demo link disabled.", "")
	}
}

// PromptsShareLinkHandler generates/rotates or disables the demo share token from
// the Demo-prompts admin page (action=generate|disable).
func PromptsShareLinkHandler(h *history.Store, auditor audit.Sink) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil {
			redirectPrompts(w, r, "", "the shareable link requires ROUTER_DATA_DIR")
			return
		}
		_ = r.ParseForm()
		switch r.FormValue("action") {
		case "disable":
			if err := SetDemoShareToken(h, ""); err != nil {
				redirectPrompts(w, r, "", "save: "+err.Error())
				return
			}
			audit.Record(r.Context(), auditor, audit.Entry{
				Action: actionDemoAccess, Actor: AdminUserFromContext(r.Context()),
				Target: "shared-link", Reason: "disabled",
			})
			redirectPrompts(w, r, "Shared demo link disabled.", "")
		default: // generate / rotate
			tok, err := GenerateDemoShareToken()
			if err != nil {
				redirectPrompts(w, r, "", "could not generate a token")
				return
			}
			if err := SetDemoShareToken(h, tok); err != nil {
				redirectPrompts(w, r, "", "save: "+err.Error())
				return
			}
			audit.Record(r.Context(), auditor, audit.Entry{
				Action: actionDemoAccess, Actor: AdminUserFromContext(r.Context()),
				Target: "shared-link", Reason: "rotated",
			})
			redirectPrompts(w, r, "New shared demo link created.", "")
		}
	}
}
