package server

// Users admin page: create named console accounts, reset passwords, and
// disable/enable them — so console actions are attributable to a person. Every
// mutation is audited with the acting user as the actor.

import (
	"context"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/history"
)

type userView struct {
	Username  string
	Role      string
	Created   string
	LastLogin string
	Disabled  bool
}

type usersPageData struct {
	Users  []userView
	Notice string
	Error  string
}

// UsersPageHandler renders the users admin page.
func UsersPageHandler(h *history.Store, auditor audit.Sink) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, _ := h.ListAdminUsers()
		views := make([]userView, 0, len(users))
		for _, u := range users {
			views = append(views, userView{
				Username: u.Username, Role: u.Role,
				Created: fmtTime(u.CreatedAt), LastLogin: fmtTime(u.LastLoginAt), Disabled: u.Disabled(),
			})
		}
		data := usersPageData{Users: views, Notice: r.URL.Query().Get("ok"), Error: r.URL.Query().Get("err")}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := usersTmpl.Execute(w, data); err != nil {
			http.Error(w, "users render error", http.StatusInternalServerError)
		}
	}
}

// UsersAddHandler creates or resets a user from the form.
func UsersAddHandler(h *history.Store, auditor audit.Sink) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")
		role := strings.TrimSpace(r.FormValue("role"))
		if role == "" {
			role = "admin"
		}
		if username == "" || len(password) < 8 {
			redirectUsers(w, r, "", "a username is required and the password must be at least 8 characters")
			return
		}
		hash, err := HashPassword(password)
		if err != nil {
			redirectUsers(w, r, "", "could not hash the password")
			return
		}
		if err := h.UpsertAdminUser(history.AdminUser{Username: username, PasswordHash: hash, Role: role, CreatedAt: time.Now()}); err != nil {
			redirectUsers(w, r, "", "spara: "+err.Error())
			return
		}
		auditUserChange(r.Context(), auditor, "admin_user.upsert", username)
		redirectUsers(w, r, "User "+username+" saved.", "")
	}
}

// UsersDisableHandler disables or re-enables a user.
func UsersDisableHandler(h *history.Store, auditor audit.Sink) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		username := strings.TrimSpace(r.FormValue("username"))
		if username == "" {
			redirectUsers(w, r, "", "username saknas")
			return
		}
		enable := r.FormValue("enable") == "1"
		at := time.Now()
		action := "admin_user.disable"
		msg := "User " + username + " disabled."
		if enable {
			at = time.Time{}
			action = "admin_user.enable"
			msg = "User " + username + " enabled."
		}
		if err := h.SetAdminUserDisabled(username, at); err != nil {
			redirectUsers(w, r, "", "spara: "+err.Error())
			return
		}
		auditUserChange(r.Context(), auditor, audit.Action(action), username)
		redirectUsers(w, r, msg, "")
	}
}

func auditUserChange(ctx context.Context, auditor audit.Sink, action audit.Action, target string) {
	audit.Record(ctx, auditor, audit.Entry{
		Action: action,
		Actor:  AdminUserFromContext(ctx),
		Target: target,
	})
}

func redirectUsers(w http.ResponseWriter, r *http.Request, ok, errMsg string) {
	u := "/router/users"
	if ok != "" {
		u += "?ok=" + urlQueryEscape(ok)
	} else if errMsg != "" {
		u += "?err=" + urlQueryEscape(errMsg)
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}

var usersTmpl = template.Must(template.New("users").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
}).Parse(usersHTML))

const usersHTML = `<!doctype html>
<html lang="sv"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Users</title>
<style>
{{adminCSS}}
*{box-sizing:border-box}
body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.hd{padding:20px 26px;border-bottom:1px solid #22324f}
.hd h1{font-size:1.3rem;margin:0}
.hd .sub{color:#8fa1bf;font-size:.85rem;margin-top:4px}
.wrap{padding:22px 30px;display:flex;flex-direction:column;gap:18px}
.card{background:#101c34;border:1px solid #22324f;border-radius:12px;padding:18px 20px;overflow-x:auto}
table{width:100%;border-collapse:collapse}
th{text-align:left;font-size:.72rem;color:#64748b;text-transform:uppercase;letter-spacing:.05em;padding:8px 10px;border-bottom:1px solid #22324f}
td{padding:9px 10px;border-bottom:1px solid #17253f;font-size:.86rem}
.tag{display:inline-block;padding:2px 9px;border-radius:20px;font-size:.72rem;background:#17253f;color:#b7c4dc}
.tag.ok{background:#0f2f22;color:#4ade80}.tag.off{background:#3a1620;color:#f87171}
.mono{font-family:ui-monospace,monospace;font-size:.82rem}
.btn{background:#17253f;color:#e8eef7;border:1px solid #2c4066;border-radius:8px;padding:8px 14px;font-size:.85rem;cursor:pointer}
.btn.del{background:#3a1620;border-color:#5b2330;color:#f8b4bc}
.btn.primary{background:#fad100;color:#081328;border-color:#fad100;font-weight:600}.btn.primary:hover{background:#fddc5b}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(170px,1fr));gap:12px}
label{display:block;font-size:.75rem;color:#8fa1bf;margin-bottom:4px}
input{width:100%;background:#081328;border:1px solid #2c4066;border-radius:8px;color:#e8eef7;padding:8px 10px;font-size:.85rem}
.notice{background:#0f2f22;border:1px solid #1f6b45;color:#a7f3d0;border-radius:8px;padding:10px 14px;font-size:.85rem}
.err{background:#3a1620;border:1px solid #7a2b39;color:#f8b4bc;border-radius:8px;padding:10px 14px;font-size:.85rem}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "users"}}
<div class="tk-main">
<div class="hd"><h1>Users</h1>
<div class="sub">Named console accounts. Every console action is attributed to the signed-in user in the audit log.</div></div>
<div class="wrap">

{{if .Notice}}<div class="notice">{{.Notice}}</div>{{end}}
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}

<div class="card">
<table>
<thead><tr><th>Username</th><th>Role</th><th>Created</th><th>Last login</th><th>Status</th><th></th></tr></thead>
<tbody>
{{range .Users}}
<tr>
  <td class="mono">{{.Username}}</td>
  <td>{{.Role}}</td>
  <td class="mono">{{.Created}}</td>
  <td class="mono">{{if .LastLogin}}{{.LastLogin}}{{else}}—{{end}}</td>
  <td>{{if .Disabled}}<span class="tag off">disabled</span>{{else}}<span class="tag ok">active</span>{{end}}</td>
  <td>
    <form method="post" action="/router/users/disable" style="margin:0">
      <input type="hidden" name="username" value="{{.Username}}">
      <input type="hidden" name="enable" value="{{if .Disabled}}1{{else}}0{{end}}">
      <button class="btn{{if not .Disabled}} del{{end}}" type="submit">{{if .Disabled}}Aktivera{{else}}Inaktivera{{end}}</button>
    </form>
  </td>
</tr>
{{else}}
<tr><td colspan="6" style="color:#64748b;text-align:center;padding:20px"><b style="color:#e8eef7">No named users yet.</b><br>Create one account per person — every console action (policy changes, key creation, roster edits) is then attributed to a name in the audit trail instead of to the shared password. (The break-glass env password keeps working for emergencies.)</td></tr>
{{end}}
</tbody>
</table>
</div>

<div class="card">
<h2 style="margin:0 0 12px;font-size:1rem">Create / reset user</h2>
<form method="post" action="/router/users">
  <div class="grid">
    <div><label>Username</label><input name="username" placeholder="anna.svensson" required></div>
    <div><label>Role</label><input name="role" placeholder="admin" value="admin"></div>
    <div><label>Password (min 8 characters)</label><input name="password" type="password" required></div>
  </div>
  <div style="margin-top:14px"><button class="btn primary" type="submit">Save user</button></div>
</form>
</div>

</div></div></div>
</body></html>`
