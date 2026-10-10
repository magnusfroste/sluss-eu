package server

// Keys admin page (ISSUE-080): a browser UI over the DB-backed key provisioning
// API (ISSUE-079). Admins mint a key for a department, see the list, and revoke
// — without curl. The minted secret is rendered exactly ONCE, inline on the
// page after creation (never in a URL, log, or the list), then is unrecoverable.

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/magnusfroste/sluss/internal/apikeys"
)

type keyView struct {
	KeyID     string
	TenantID  string
	ProjectID string
	Role      string
	Scopes    string
	Created   string
	LastUsed  string
	Revoked   bool
}

// mintedKey is the one-time plaintext reveal shown right after minting.
type mintedKey struct {
	KeyID     string
	Plaintext string
	TenantID  string
	ProjectID string
}

type keysPageData struct {
	Keys   []keyView
	Minted *mintedKey
	Notice string
	Error  string
}

func keyViews(mgr *apikeys.Manager) ([]keyView, error) {
	recs, err := mgr.List()
	if err != nil {
		return nil, err
	}
	views := make([]keyView, 0, len(recs))
	for _, k := range recs {
		views = append(views, keyView{
			KeyID: k.KeyID, TenantID: k.TenantID, ProjectID: k.ProjectID,
			Role: k.Role, Scopes: strings.Join(k.Scopes, ", "),
			Created: fmtTime(k.CreatedAt), LastUsed: fmtTime(k.LastUsedAt),
			Revoked: k.Revoked(),
		})
	}
	return views, nil
}

func renderKeysPage(w http.ResponseWriter, mgr *apikeys.Manager, data keysPageData) {
	views, err := keyViews(mgr)
	if err != nil && data.Error == "" {
		data.Error = "could not read keys: " + err.Error()
	}
	data.Keys = views
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := keysTmpl.Execute(w, data); err != nil {
		http.Error(w, "keys render error", http.StatusInternalServerError)
	}
}

// KeysPageHandler renders the keys admin page (GET).
func KeysPageHandler(mgr *apikeys.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderKeysPage(w, mgr, keysPageData{
			Notice: r.URL.Query().Get("ok"),
			Error:  r.URL.Query().Get("err"),
		})
	}
}

// KeysMintFormHandler mints a key from the HTML form and re-renders the page
// with the plaintext shown once. It never redirects with the secret in the URL.
func KeysMintFormHandler(mgr *apikeys.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			renderKeysPage(w, mgr, keysPageData{Error: "invalid form"})
			return
		}
		var scopes []string
		if s := strings.TrimSpace(r.FormValue("scopes")); s != "" {
			for _, part := range strings.Split(s, ",") {
				if p := strings.TrimSpace(part); p != "" {
					scopes = append(scopes, p)
				}
			}
		}
		plaintext, rec, err := mgr.Mint(apikeys.MintRequest{
			TenantID:  strings.TrimSpace(r.FormValue("tenant_id")),
			ProjectID: strings.TrimSpace(r.FormValue("project_id")),
			Role:      strings.TrimSpace(r.FormValue("role")),
			Scopes:    scopes,
			Actor:     AdminUserFromContext(r.Context()),
		})
		if err != nil {
			renderKeysPage(w, mgr, keysPageData{Error: err.Error()})
			return
		}
		renderKeysPage(w, mgr, keysPageData{Minted: &mintedKey{
			KeyID: rec.KeyID, Plaintext: plaintext, TenantID: rec.TenantID, ProjectID: rec.ProjectID,
		}})
	}
}

// KeysRevokeFormHandler revokes a key from the HTML form and redirects back.
func KeysRevokeFormHandler(mgr *apikeys.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		keyID := strings.TrimSpace(r.FormValue("key_id"))
		if keyID == "" {
			redirectKeys(w, r, "", "key_id saknas")
			return
		}
		ok, err := mgr.Revoke(keyID, AdminUserFromContext(r.Context()))
		switch {
		case err != nil:
			redirectKeys(w, r, "", "revoke misslyckades: "+err.Error())
		case !ok:
			redirectKeys(w, r, "", "ingen aktiv nyckel med id "+keyID)
		default:
			redirectKeys(w, r, "Key "+keyID+" revoked.", "")
		}
	}
}

func redirectKeys(w http.ResponseWriter, r *http.Request, ok, errMsg string) {
	u := "/router/keys"
	if ok != "" {
		u += "?ok=" + urlQueryEscape(ok)
	} else if errMsg != "" {
		u += "?err=" + urlQueryEscape(errMsg)
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}

var keysTmpl = template.Must(template.New("keys").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
}).Parse(keysHTML))

const keysHTML = `<!doctype html>
<html lang="sv"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Keys</title>
<style>
{{adminCSS}}
*{box-sizing:border-box}
body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.tk-main{padding:0}
.hd{padding:20px 26px;border-bottom:1px solid #22324f}
.hd h1{font-size:1.3rem;margin:0}
.hd .sub{color:#8fa1bf;font-size:.85rem;margin-top:4px}
.wrap{padding:22px 30px;display:flex;flex-direction:column;gap:18px}
.card{background:#101c34;border:1px solid #22324f;border-radius:12px;padding:18px 20px;overflow-x:auto}
table{width:100%;border-collapse:collapse}
th{text-align:left;font-size:.72rem;color:#5f6e87;text-transform:uppercase;letter-spacing:.05em;padding:8px 10px;border-bottom:1px solid #22324f}
td{padding:9px 10px;border-bottom:1px solid #17253f;font-size:.86rem}
.mono{font-family:ui-monospace,monospace;font-size:.82rem}
.tag{display:inline-block;padding:2px 8px;border-radius:20px;font-size:.72rem;background:#182a4a;color:#b7c4dc}
.tag.ok{background:#0f3323;color:#4ade80}.tag.bad{background:#3a1620;color:#f87171}
.btn{background:#1a2a49;color:#e8eef7;border:1px solid #2c4066;border-radius:8px;padding:8px 14px;font-size:.85rem;cursor:pointer}
.btn.del{background:#3a1620;border-color:#5b2330;color:#f8b4bc}
.btn.primary{background:#fad100;color:#081328;border-color:#fad100;font-weight:600}.btn.primary:hover{background:#fddc5b}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(170px,1fr));gap:12px}
label{display:block;font-size:.75rem;color:#8fa1bf;margin-bottom:4px}
input{width:100%;background:#081328;border:1px solid #2c4066;border-radius:8px;color:#e8eef7;padding:8px 10px;font-size:.85rem}
.reveal{background:#0f2f22;border:1px solid #22c58b;border-radius:12px;padding:16px 18px}
.reveal .key{font-family:ui-monospace,monospace;font-size:1rem;background:#06170f;border:1px dashed #22c58b;border-radius:8px;padding:12px;margin:10px 0;word-break:break-all;color:#7dffc0}
.warn{color:#ffcf8f;font-size:.82rem}
.notice{background:#0f2f22;border:1px solid #1f6b45;color:#a7f3d0;border-radius:8px;padding:10px 14px;font-size:.85rem}
.err{background:#3a1620;border:1px solid #7a2b39;color:#f8b4bc;border-radius:8px;padding:10px 14px;font-size:.85rem}
.empty{color:#5f6e87;text-align:center;padding:22px}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "keys"}}
<div class="tk-main">
<div class="hd"><h1>API keys</h1>
<div class="sub">DB-backed keys per department (tenant/project). The secret is shown <b>once</b>.</div></div>
<div class="wrap">

{{if .Notice}}<div class="notice">{{.Notice}}</div>{{end}}
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}

{{if .Minted}}
<div class="reveal">
  <div>Key <span class="mono">{{.Minted.KeyID}}</span> created for <b>{{.Minted.TenantID}}</b>{{if .Minted.ProjectID}} / {{.Minted.ProjectID}}{{end}}.</div>
  <div class="key">{{.Minted.Plaintext}}</div>
  <div class="warn">Copy the key now — it is never shown again and cannot be recovered.</div>
</div>
{{end}}

<div class="card">
<table>
<thead><tr><th>Key ID</th><th>Tenant</th><th>Project</th><th>Role</th><th>Scopes</th><th>Created</th><th>Last used</th><th>Status</th><th></th></tr></thead>
<tbody>
{{range .Keys}}
<tr>
  <td class="mono">{{.KeyID}}</td>
  <td>{{.TenantID}}</td>
  <td>{{if .ProjectID}}{{.ProjectID}}{{else}}—{{end}}</td>
  <td>{{if .Role}}{{.Role}}{{else}}user{{end}}</td>
  <td class="mono">{{if .Scopes}}{{.Scopes}}{{else}}*{{end}}</td>
  <td class="mono">{{.Created}}</td>
  <td class="mono">{{if .LastUsed}}{{.LastUsed}}{{else}}—{{end}}</td>
  <td>{{if .Revoked}}<span class="tag bad">revoked</span>{{else}}<span class="tag ok">active</span>{{end}}</td>
  <td>{{if not .Revoked}}
    <form method="post" action="/router/keys/revoke" onsubmit="return confirm('Revoke {{.KeyID}}?')" style="margin:0">
      <input type="hidden" name="key_id" value="{{.KeyID}}">
      <button class="btn del" type="submit">Revoke</button>
    </form>
  {{end}}</td>
</tr>
{{else}}
<tr><td colspan="9" class="empty"><b style="color:#e8eef7">No department keys yet.</b><br>Create one key per department or team below — every request is then attributed to it in the log, the dashboard and the audit trail, and you can revoke one team without touching the others. The secret is shown once. (Until then the env key <span class="mono">LOCAL_API_KEY</span> works as bootstrap.)</td></tr>
{{end}}
</tbody>
</table>
</div>

<div class="card">
<h2 style="margin:0 0 12px;font-size:1rem">Create a key for a department</h2>
<form method="post" action="/router/keys">
  <div class="grid">
    <div><label>Tenant ID (department)</label><input name="tenant_id" placeholder="finance" required></div>
    <div><label>Project (optional)</label><input name="project_id" placeholder="prod"></div>
    <div><label>Role (user/admin)</label><input name="role" placeholder="user"></div>
    <div><label>Scopes (comma-separated, empty = all)</label><input name="scopes" placeholder="chat:completions"></div>
  </div>
  <div style="margin-top:14px"><button class="btn primary" type="submit">Create key</button></div>
</form>
</div>

</div></div></div>
</body></html>`
