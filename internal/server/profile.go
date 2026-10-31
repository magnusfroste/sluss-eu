package server

// The signed-in person's own page (ISSUE-155): who you are and how you signed
// in, a new password for a local account, your chat history (download or
// delete), and sign out. Admins see it inside the console; employees and
// demo guests see it on its own with a way back to the chat. It only ever
// touches the caller's own account and history.

import (
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/history"
)

type profileData struct {
	Me      whoami
	Method  string // how this session signed in, in plain words
	Local   bool   // a local account: the password can be changed here
	Console bool   // render inside the admin console
	History bool   // a server-side chat history exists for this person
	Chats   int
	Notice  string
	Error   string
}

// signInMethod says how the current session signed in.
func signInMethod(me whoami, local bool) string {
	switch {
	case me.BreakGlass:
		return "The break-glass password — not a named account"
	case me.Role == "api":
		return "An admin API key"
	case me.Role == roleDemo:
		return "A demo link"
	case local:
		return "Username and password (a local Sluss account)"
	default:
		return "Your company account (single sign-on)"
	}
}

func isLocalAccount(h *history.Store, me whoami) bool {
	if h == nil || me.BreakGlass || me.User == "" {
		return false
	}
	_, ok, _ := h.GetAdminUser(me.User)
	return ok
}

// ProfilePageHandler serves GET /me.
func ProfilePageHandler(h *history.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me := currentWho(r)
		local := isLocalAccount(h, me)
		d := profileData{
			Me: me, Local: local, Method: signInMethod(me, local),
			Console: me.Role != roleUser && me.Role != roleDemo,
			Notice:  r.URL.Query().Get("ok"), Error: r.URL.Query().Get("err"),
		}
		if key := chatSessionsKey(r); key != "" && h != nil {
			if v, ok := h.KVGet(key); ok && strings.TrimSpace(v) != "" && strings.TrimSpace(v) != "[]" {
				d.History = true
				d.Chats = strings.Count(v, `"messages"`)
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := profileTmpl.Execute(w, d); err != nil && logger != nil {
			logger.Error("profile render failed", "err", err)
		}
	}
}

// ProfilePasswordHandler serves POST /me/password: a local account changes
// its own password after confirming the current one.
func ProfilePasswordHandler(h *history.Store, auditor audit.Sink) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		me := currentWho(r)
		back := func(ok, errMsg string) {
			u := "/me"
			if ok != "" {
				u += "?ok=" + urlQueryEscape(ok)
			} else if errMsg != "" {
				u += "?err=" + urlQueryEscape(errMsg)
			}
			http.Redirect(w, r, u, http.StatusSeeOther)
		}
		if !isLocalAccount(h, me) {
			back("", "This sign-in has no Sluss password. Change it where you signed in.")
			return
		}
		u, _, _ := h.GetAdminUser(me.User)
		current, next, again := r.FormValue("current"), r.FormValue("new"), r.FormValue("again")
		switch {
		case !VerifyPassword(u.PasswordHash, current):
			back("", "The current password is not right.")
			return
		case len(next) < 8:
			back("", "The new password must be at least 8 characters.")
			return
		case next != again:
			back("", "The two new passwords do not match.")
			return
		case next == current:
			back("", "The new password is the same as the current one.")
			return
		}
		hash, err := HashPassword(next)
		if err != nil {
			back("", "Could not save the password.")
			return
		}
		u.PasswordHash = hash
		if err := h.UpsertAdminUser(u); err != nil {
			back("", "Could not save the password.")
			return
		}
		auditUserChange(r.Context(), auditor, audit.Action("admin_user.password_changed"), me.User)
		back("Your password is changed.", "")
	}
}

// ProfileHistoryHandler serves GET /me/history (download) and POST
// /me/history/delete (delete everything) for the caller's own chats.
func ProfileHistoryHandler(h *history.Store, auditor audit.Sink) (download, del http.HandlerFunc) {
	download = func(w http.ResponseWriter, r *http.Request) {
		v := "[]"
		if key := chatSessionsKey(r); key != "" && h != nil {
			if s, ok := h.KVGet(key); ok && strings.TrimSpace(s) != "" {
				v = s
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="sluss-chats-`+time.Now().UTC().Format("2006-01-02")+`.json"`)
		_, _ = w.Write([]byte(v))
	}
	del = func(w http.ResponseWriter, r *http.Request) {
		if key := chatSessionsKey(r); key != "" && h != nil {
			_ = h.KVSet(key, "[]")
		}
		audit.Record(r.Context(), auditor, audit.Entry{Action: audit.Action("chat.history_deleted"), Actor: AdminUserFromContext(r.Context())})
		http.Redirect(w, r, "/me?ok="+urlQueryEscape("Your chat history is deleted."), http.StatusSeeOther)
	}
	return download, del
}

var profileTmpl = template.Must(template.New("profile").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
	"dsHead":   dsHeadFunc,
}).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Your profile</title>
<style>
{{adminCSS}}
*{box-sizing:border-box}
body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.mono{font-family:"IBM Plex Mono",ui-monospace,Menlo,monospace}
.solo{max-width:720px;margin:0 auto;padding:28px 20px 50px}
.solo .back{display:inline-block;margin-bottom:18px;color:#7fd3ff;text-decoration:none;font-size:14px}
.kv{display:grid;grid-template-columns:150px 1fr;gap:10px;padding:8px 0;border-bottom:1px solid #17253f;font-size:14px}
.kv:last-child{border-bottom:0}
.kv .k{color:#8fa1bf}
.pf label{display:block;font-size:13px;color:#8fa1bf;margin:12px 0 5px}
.pf input{width:100%;max-width:360px;background:#0b1528;border:1px solid #22324f;color:#e8eef7;border-radius:8px;padding:9px 11px;font:inherit}
.pf input:focus{outline:none;border-color:#fad100}
.row{display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:14px}
.hint{color:#8fa1bf;font-size:13px;line-height:1.55;margin:6px 0 0}
.row .ds-btn{margin-left:0;font-size:.85rem;font-weight:600;font-family:inherit;line-height:1.2}
.ds-btn.danger{background:transparent;color:#f07ab0;border:1px solid #4a1c33}
.ds-btn.danger:hover{background:#2a1424}
.ds-btn.ghost{background:transparent;color:#cfe0f7;border:1px solid #2a3b5c}
</style></head>
<body>{{if .Console}}<div class="tk-shell">
{{adminNav ""}}
<div class="tk-main"><div class="ds-content">
{{dsHead "" "Your profile" "Who you are signed in as, your password and your chat history."}}
{{else}}<div class="solo">
<a class="back" href="/chat">← Back to the chat</a>
<h1 style="margin:0 0 4px;font-size:1.5rem">Your profile</h1>
<p class="hint" style="margin:0 0 18px">Who you are signed in as, your password and your chat history.</p>
{{end}}
{{if .Notice}}<div class="ds-alert lvl-ok" style="margin-bottom:14px"><span class="ds-dot"></span><div>{{.Notice}}</div></div>{{end}}
{{if .Error}}<div class="ds-alert lvl-bad" style="margin-bottom:14px"><span class="ds-dot"></span><div>{{.Error}}</div></div>{{end}}

<div class="ds-card">
  <h2>You</h2>
  <div class="kv"><span class="k">Signed in as</span><span class="mono">{{.Me.User}}</span></div>
  <div class="kv"><span class="k">Role</span><span>{{if eq .Me.Role "admin"}}Administrator — the console and the chat{{else if eq .Me.Role "user"}}Employee — the chat{{else if eq .Me.Role "demo"}}Demo guest — the chat{{else}}{{.Me.Role}}{{end}}</span></div>
  {{if .Me.Department}}<div class="kv"><span class="k">Department</span><span>{{.Me.Department}}</span></div>{{end}}
  <div class="kv"><span class="k">Signed in with</span><span>{{.Method}}</span></div>
</div>

<div class="ds-section"><div class="ds-card">
  <h2>Password</h2>
  {{if .Local}}
  <form class="pf" method="post" action="/me/password" autocomplete="off">
    <label for="cur">Current password</label><input id="cur" name="current" type="password" autocomplete="current-password" required>
    <label for="new">New password (at least 8 characters)</label><input id="new" name="new" type="password" autocomplete="new-password" minlength="8" required>
    <label for="again">New password again</label><input id="again" name="again" type="password" autocomplete="new-password" minlength="8" required>
    <div class="row"><button class="ds-btn primary" type="submit">Change password</button></div>
  </form>
  {{else if .Me.BreakGlass}}
  <p class="hint">You signed in with the break-glass password, which is set in the server's environment (<span class="mono">ROUTER_DASHBOARD_PASSWORD</span>), not here. For everyday use, create a named account under Access → Console users.</p>
  {{else}}
  <p class="hint">Your password belongs to the account you signed in with{{if eq .Me.Role "demo"}} (a demo link has none){{else}} — change it there, for example in your company's Microsoft or Google account{{end}}.</p>
  {{end}}
</div></div>

<div class="ds-section"><div class="ds-card">
  <h2>Your chat history</h2>
  <p class="hint">{{if .History}}About {{.Chats}} chat{{if ne .Chats 1}}s{{end}} are kept for you on this server. Only you can see them.{{else}}No chats are kept for you on this server yet.{{end}}{{if eq .Me.Role "demo"}} Everyone using the same demo link shares this history.{{end}}</p>
  <div class="row">
    <a class="ds-btn ghost" href="/me/history" download>Download as JSON</a>
    {{if .History}}<form method="post" action="/me/history/delete" onsubmit="return confirm('Delete all your chats? This cannot be undone.')" style="margin:0"><button class="ds-btn danger" type="submit">Delete all my chats</button></form>{{end}}
  </div>
  <p class="hint">Deleting removes the chats, not the request log: that keeps what kind of question went where — never its text — for the audit trail.</p>
</div></div>

<div class="ds-section"><div class="row" style="margin-top:0"><a class="ds-btn ghost" href="/router/logout">Sign out</a></div></div>
{{if .Console}}</div></div></div>{{else}}</div>{{end}}
</body></html>`))
