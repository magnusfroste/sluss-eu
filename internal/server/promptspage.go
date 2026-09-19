package server

// Admin page for the demo quick prompts (ISSUE: admin-managed demo). Same admin
// shell as Models/Providers/Keys. Add, edit-by-readd, delete, and reset to the
// curated defaults — so a demo link is a living, editable showcase.

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/magnusfroste/sluss/internal/history"
)

type namedLinkView struct {
	Label    string
	URL      string
	Created  string
	Opens    int64
	LastOpen string
	Active   bool
	Token    string
}

type promptsPageData struct {
	Prompts    []QuickPrompt
	ReadOnly   bool // no data dir → prompts are the built-in defaults, not editable
	ShareURL   string
	ShareOn    bool
	NeedsBase  bool // link is enabled but ROUTER_PUBLIC_URL is unset (relative link)
	NamedLinks []namedLinkView
	Notice     string
	Error      string
}

// PromptsPageHandler renders the quick-prompts admin page. publicURL is the
// site's public base (ROUTER_PUBLIC_URL) used to build the shareable demo link.
func PromptsPageHandler(h *history.Store, publicURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := LoadDemoShareToken(h)
		var links []namedLinkView
		if h != nil {
			if ls, err := h.ListDemoLinks(); err == nil {
				for _, l := range ls {
					v := namedLinkView{Label: l.Label, URL: DemoShareURL(publicURL, l.Token),
						Created: l.CreatedAt.Format("2006-01-02"), Opens: l.Opens,
						Active: l.Active(), Token: l.Token}
					if !l.LastOpenAt.IsZero() {
						v.LastOpen = l.LastOpenAt.Format("2006-01-02 15:04")
					}
					links = append(links, v)
				}
			}
		}
		data := promptsPageData{
			Prompts:    LoadQuickPrompts(h),
			ReadOnly:   h == nil,
			ShareURL:   DemoShareURL(publicURL, token),
			ShareOn:    token != "",
			NeedsBase:  token != "" && strings.TrimSpace(publicURL) == "",
			NamedLinks: links,
			Notice:     r.URL.Query().Get("ok"),
			Error:      r.URL.Query().Get("err"),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := promptsTmpl.Execute(w, data); err != nil {
			http.Error(w, "prompts render error", http.StatusInternalServerError)
		}
	}
}

// PromptsAddHandler appends a prompt from the form.
func PromptsAddHandler(h *history.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil {
			redirectPrompts(w, r, "", "prompts are read-only (no data dir)")
			return
		}
		_ = r.ParseForm()
		p := QuickPrompt{
			Label: strings.TrimSpace(r.FormValue("label")),
			Tier:  strings.TrimSpace(r.FormValue("tier")),
			Text:  strings.TrimSpace(r.FormValue("text")),
			Note:  strings.TrimSpace(r.FormValue("note")),
		}
		if p.Label == "" || p.Text == "" {
			redirectPrompts(w, r, "", "label and text are required")
			return
		}
		prompts := append(LoadQuickPrompts(h), p)
		if err := SaveQuickPrompts(h, prompts); err != nil {
			redirectPrompts(w, r, "", "save: "+err.Error())
			return
		}
		redirectPrompts(w, r, "Prompt added.", "")
	}
}

// PromptsDeleteHandler removes the prompt at the given index.
func PromptsDeleteHandler(h *history.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil {
			redirectPrompts(w, r, "", "prompts are read-only (no data dir)")
			return
		}
		_ = r.ParseForm()
		idx, err := strconv.Atoi(r.FormValue("index"))
		prompts := LoadQuickPrompts(h)
		if err != nil || idx < 0 || idx >= len(prompts) {
			redirectPrompts(w, r, "", "invalid index")
			return
		}
		prompts = append(prompts[:idx], prompts[idx+1:]...)
		if err := SaveQuickPrompts(h, prompts); err != nil {
			redirectPrompts(w, r, "", "save: "+err.Error())
			return
		}
		redirectPrompts(w, r, "Prompt removed.", "")
	}
}

// PromptsResetHandler restores the curated default set.
func PromptsResetHandler(h *history.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil {
			redirectPrompts(w, r, "", "prompts are read-only (no data dir)")
			return
		}
		if err := SaveQuickPrompts(h, defaultQuickPrompts()); err != nil {
			redirectPrompts(w, r, "", "save: "+err.Error())
			return
		}
		redirectPrompts(w, r, "Restored to the default set.", "")
	}
}

func redirectPrompts(w http.ResponseWriter, r *http.Request, ok, errMsg string) {
	u := "/router/prompts"
	if ok != "" {
		u += "?ok=" + urlQueryEscape(ok)
	} else if errMsg != "" {
		u += "?err=" + urlQueryEscape(errMsg)
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}

var promptsTmpl = template.Must(template.New("prompts").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
}).Parse(promptsHTML))

const promptsHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Demo prompts</title>
<style>
{{adminCSS}}
*{box-sizing:border-box}
body{margin:0;background:#0b1220;color:#e8eef7;font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.hd{padding:20px 26px;border-bottom:1px solid #22304d}
.hd h1{font-size:1.3rem;margin:0}
.hd .sub{color:#8fa1bf;font-size:.85rem;margin-top:4px}
.wrap{padding:22px 30px;display:flex;flex-direction:column;gap:18px}
.card{background:#0e1626;border:1px solid #22304d;border-radius:12px;padding:18px 20px;overflow-x:auto}
table{width:100%;border-collapse:collapse}
th{text-align:left;font-size:.72rem;color:#5f6e87;text-transform:uppercase;letter-spacing:.05em;padding:8px 10px;border-bottom:1px solid #22304d}
td{padding:9px 10px;border-bottom:1px solid #16223b;font-size:.86rem;vertical-align:top}
.tag{display:inline-block;padding:2px 9px;border-radius:20px;font-size:.72rem;background:#12203a;color:#b7c4dc;white-space:nowrap}
.note{color:#8fa1bf;font-size:.8rem}
.mono{font-family:ui-monospace,monospace;font-size:.82rem}
.btn{background:#1a2a49;color:#e8eef7;border:1px solid #2c4066;border-radius:8px;padding:8px 14px;font-size:.85rem;cursor:pointer}
.btn.del{background:#3a1620;border-color:#5b2330;color:#f8b4bc}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px}
label{display:block;font-size:.75rem;color:#8fa1bf;margin-bottom:4px}
input,textarea{width:100%;background:#0b1220;border:1px solid #2c4066;border-radius:8px;color:#e8eef7;padding:8px 10px;font-size:.85rem;font-family:inherit}
.notice{background:#0f2f22;border:1px solid #1f6b45;color:#a7f3d0;border-radius:8px;padding:10px 14px;font-size:.85rem}
.err{background:#3a1620;border:1px solid #7a2b39;color:#f8b4bc;border-radius:8px;padding:10px 14px;font-size:.85rem}
.ro{background:#33290f;border:1px solid #7a5a1f;color:#f4d38a;border-radius:8px;padding:10px 14px;font-size:.85rem}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "prompts"}}
<div class="tk-main">
<div class="hd"><h1>Demo prompts</h1>
<div class="sub">The quick-prompt chips in the live demo (<a href="/chat" style="color:#7fd3ff">/chat</a>). Send a link to a CISO — they click through the value, then you call.</div></div>
<div class="wrap">

{{if .Notice}}<div class="notice">{{.Notice}}</div>{{end}}
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
{{if .ReadOnly}}<div class="ro">Read-only mode (no ROUTER_DATA_DIR) — showing the built-in default set. Set a data dir to edit and save.</div>{{end}}

{{if not .ReadOnly}}
<div class="card">
<h2 style="margin:0 0 6px;font-size:1rem">Named demo links (per prospect)</h2>
<div class="note" style="margin-bottom:12px">One link per recipient. Every open is counted and attributed — the audit trail and the table below show <b>who</b> has tried the demo and when. Disable a link to revoke that recipient only.</div>
<form method="post" action="/router/demolinks" style="display:flex;gap:10px;align-items:flex-end;flex-wrap:wrap;margin-bottom:12px">
  <div style="flex:1;min-width:220px"><label>Label (prospect / company)</label><input name="label" placeholder="Acme AB — CISO" required></div>
  <button class="btn" type="submit">Create link</button>
</form>
{{if .NamedLinks}}
<table>
<thead><tr><th>Label</th><th>Link</th><th>Opens</th><th>Last opened</th><th>Status</th><th></th></tr></thead>
<tbody>
{{range .NamedLinks}}
<tr>
  <td><b>{{.Label}}</b><div class="note">{{.Created}}</div></td>
  <td class="mono" style="max-width:340px;overflow-wrap:anywhere">{{if .Active}}<span id="nl-{{.Token}}">{{.URL}}</span> <button class="btn" style="padding:3px 9px;font-size:11px" type="button" onclick="navigator.clipboard&&navigator.clipboard.writeText(document.getElementById('nl-{{.Token}}').textContent)">Copy</button>{{else}}<span class="note">revoked</span>{{end}}</td>
  <td>{{.Opens}}</td>
  <td class="note">{{if .LastOpen}}{{.LastOpen}}{{else}}never{{end}}</td>
  <td>{{if .Active}}<span class="tag" style="background:#13351f;color:#4fd08a">active</span>{{else}}<span class="tag">disabled</span>{{end}}</td>
  <td>{{if .Active}}<form method="post" action="/router/demolinks/disable" onsubmit="return confirm('Disable this link? {{.Label}} will no longer be able to open the demo.')" style="margin:0"><input type="hidden" name="token" value="{{.Token}}"><button class="btn del" type="submit">Disable</button></form>{{end}}</td>
</tr>
{{end}}
</tbody>
</table>
{{else}}<div class="note">No named links yet — create one per prospect above.</div>{{end}}
</div>

<div class="card">
<h2 style="margin:0 0 6px;font-size:1rem">Legacy shared demo link</h2>
<div class="note" style="margin-bottom:12px">One secret link that opens the live demo <b>without a password</b> — chat only, never admin. Every open is audited. Prefer named links above for per-prospect attribution; rotate this one to revoke it everywhere.</div>
{{if .ShareOn}}
  <div style="display:flex;gap:10px;align-items:center;flex-wrap:wrap">
    <input id="shareurl" class="mono" style="flex:1;min-width:280px" readonly value="{{.ShareURL}}">
    <button class="btn" type="button" onclick="navigator.clipboard&&navigator.clipboard.writeText(document.getElementById('shareurl').value)">Copy</button>
    <form method="post" action="/router/prompts/sharelink" style="margin:0"><input type="hidden" name="action" value="generate"><button class="btn" type="submit" onclick="return confirm('Create a new link? The old one stops working.')">Rotate</button></form>
    <form method="post" action="/router/prompts/sharelink" style="margin:0"><input type="hidden" name="action" value="disable"><button class="btn del" type="submit" onclick="return confirm('Disable the shared link?')">Disable</button></form>
  </div>
  {{if .NeedsBase}}<div class="ro" style="margin-top:10px">Set <span class="mono">ROUTER_PUBLIC_URL</span> to make links fully qualified (currently relative).</div>{{end}}
{{else}}
  <form method="post" action="/router/prompts/sharelink" style="margin:0"><input type="hidden" name="action" value="generate"><button class="btn" type="submit">Create shared link</button></form>
{{end}}
</div>
{{end}}

<div class="card">
<table>
<thead><tr><th>Label</th><th>Hint</th><th>Prompt</th><th>What to watch for</th><th></th></tr></thead>
<tbody>
{{range $i, $p := .Prompts}}
<tr>
  <td>{{$p.Label}}</td>
  <td><span class="tag">{{$p.Tier}}</span></td>
  <td class="mono">{{$p.Text}}</td>
  <td class="note">{{$p.Note}}</td>
  <td>{{if not $.ReadOnly}}
    <form method="post" action="/router/prompts/delete" onsubmit="return confirm('Remove?')" style="margin:0">
      <input type="hidden" name="index" value="{{$i}}">
      <button class="btn del" type="submit">Remove</button>
    </form>
  {{end}}</td>
</tr>
{{else}}
<tr><td colspan="5" style="color:#5f6e87;text-align:center;padding:20px">No prompts.</td></tr>
{{end}}
</tbody>
</table>
</div>

{{if not .ReadOnly}}
<div class="card">
<h2 style="margin:0 0 12px;font-size:1rem">Add prompt</h2>
<form method="post" action="/router/prompts">
  <div class="grid">
    <div><label>Label (chip text)</label><input name="label" placeholder="Summarise a customer case" required></div>
    <div><label>Hint (short)</label><input name="tier" placeholder="PII → local"></div>
  </div>
  <div style="margin-top:12px"><label>Prompt (sent to the router)</label><textarea name="text" rows="2" placeholder="Summarise the case for customer …" required></textarea></div>
  <div style="margin-top:12px"><label>What to watch for (shown during the demo)</label><input name="note" placeholder="Personal data detected → local model"></div>
  <div style="margin-top:14px;display:flex;gap:10px">
    <button class="btn" type="submit">Save prompt</button>
  </div>
</form>
</div>
<form method="post" action="/router/prompts/reset" onsubmit="return confirm('Restore the default set?')">
  <button class="btn" type="submit">Restore defaults</button>
</form>
{{end}}

</div></div></div>
</body></html>`
