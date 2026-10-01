package server

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/providercfg"
)

// ModelsOptions configures the Models admin page (ISSUE-072) — the primary
// roster. Models are the routable unit; each carries a tier (the router's USP)
// and price, and references an existing provider connection.
type ModelsOptions struct {
	Engine  *engine.Engine
	Logger  *slog.Logger
	Version string
	// Roster, when set (a data dir is configured), is the SQLite-backed source of
	// truth for the roster (ISSUE-073). It enables add/edit/delete of models and
	// supplies the provider connections for the Add-Model picker. Nil (no data
	// dir) → the Models page is read-only over the live registry.
	Roster providercfg.RosterStore
	// Reloader applies edits live (ISSUE-115); nil → restart to apply.
	Reloader *RosterReloader
	// ProbeClient overrides the HTTP client used by the per-row Test button
	// (ISSUE-098); nil uses a default with modelTestTimeout.
	ProbeClient *http.Client
}

func (o ModelsOptions) crudEnabled() bool { return o.Roster != nil }

type modelView struct {
	ID           string
	ProviderID   string
	ProviderName string
	Slug         string
	Tier         string
	InPerMTok    float64
	OutPerMTok   float64
	Enabled      bool   // admin toggle (from config)
	Routable     bool   // enabled AND provider key present in the live registry
	Synced       bool   // price is auto-synced (openrouter) vs manual
	Editable     bool   // config-backed → can re-tier / re-price / delete
	Pending      bool   // config edit not yet applied to the live registry
	Builtin      bool   // one of the three seeded tier models
	Reasoning    bool   // switchable thinking mode; router controls it per task
	Egress       string // "local" | "cloud" | "" — from the live registry's tags (ISSUE-117)
}

type providerOption struct {
	ID   string
	Name string
}

// providerNames returns id→display-name for config + registry providers.
func (o ModelsOptions) providerNames() (map[string]string, []providerOption) {
	names := map[string]string{}
	var opts []providerOption
	seen := map[string]bool{}
	add := func(id, name string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		if name == "" {
			name = id
		}
		names[id] = name
		opts = append(opts, providerOption{ID: id, Name: name})
	}
	if o.Roster != nil {
		if ps, err := o.Roster.LoadRosterProviders(); err == nil {
			for _, p := range providercfg.SortByID(ps) {
				add(p.ID, p.Name)
			}
		}
	}
	if o.Engine != nil && o.Engine.Registry != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil {
			for _, p := range snap.Providers() {
				add(p.ID, p.Name)
			}
		}
	}
	return names, opts
}

func builtinIDs() map[string]bool {
	out := map[string]bool{}
	for _, m := range providercfg.SeedModels() {
		out[m.ID] = true
	}
	return out
}

// buildModelViews assembles the roster: config-backed models (editable) plus any
// live registry models not represented in config (read-only, e.g. mock dev).
func (o ModelsOptions) buildModelViews() []modelView {
	names, _ := o.providerNames()
	builtins := builtinIDs()

	// Live registry enabled-state, keyed by model ID (presence = in registry).
	liveEnabled := map[string]bool{}
	if o.Engine != nil && o.Engine.Registry != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil {
			for _, p := range snap.Providers() {
				for _, m := range snap.ModelsForProvider(p.ID) {
					liveEnabled[m.ID] = m.Enabled
				}
			}
		}
	}

	var views []modelView
	configIDs := map[string]bool{}
	if o.Roster != nil {
		keyPresent := o.providerKeyPresence()
		if cms, err := o.Roster.LoadRosterModels(); err == nil {
			for _, m := range providercfg.SortModels(cms) {
				configIDs[m.ID] = true
				keySet := keyPresent[m.ProviderID]
				lvEnabled, inReg := liveEnabled[m.ID]
				views = append(views, modelView{
					ID: m.ID, ProviderID: m.ProviderID, ProviderName: names[m.ProviderID],
					Slug: m.ProviderModelID, Tier: m.Tier,
					InPerMTok: m.InputUSDPerMTok, OutPerMTok: m.OutputUSDPerMTok,
					Enabled:   m.Enabled,
					Routable:  m.Enabled && keySet,
					Synced:    m.ProviderID == "openrouter",
					Editable:  true,
					Pending:   !inReg || lvEnabled != (m.Enabled && keySet),
					Builtin:   builtins[m.ID],
					Reasoning: m.ReasoningCapable,
				})
			}
		}
	}

	// Live registry models with no config backing → read-only (mock dev, or a
	// registry built without the config, so nothing is silently hidden).
	if o.Engine != nil && o.Engine.Registry != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil {
			for _, p := range snap.Providers() {
				for _, m := range snap.ModelsForProvider(p.ID) {
					if configIDs[m.ID] {
						continue
					}
					views = append(views, modelView{
						ID: m.ID, ProviderID: m.ProviderID, ProviderName: firstNonEmpty(names[m.ProviderID], p.Name, p.ID),
						Slug: m.ProviderModelID, Tier: string(m.Tier),
						InPerMTok:  float64(m.Cost.InputMicrosPerMillionToken) / 1e6,
						OutPerMTok: float64(m.Cost.OutputMicrosPerMillionToken) / 1e6,
						Enabled:    m.Enabled, Routable: m.Enabled,
						Synced:    m.ProviderID == "openrouter",
						Builtin:   builtins[m.ID],
						Reasoning: m.ReasoningCapable,
					})
				}
			}
		}
	}
	// Where a prompt routed to this model goes (ISSUE-117): the same tag rule
	// as X-Router-Egress, so the roster says "local" exactly when routing does.
	classify := ChatOptions{Engine: o.Engine}
	for i := range views {
		views[i].Egress = classify.legacyEgress(views[i].ID, views[i].ProviderID)
	}
	return views
}

// providerKeyPresence maps each roster provider id to whether its referenced env
// var holds a key (so a model is routable only when its provider's key is set).
func (o ModelsOptions) providerKeyPresence() map[string]bool {
	out := map[string]bool{}
	if o.Roster == nil {
		return out
	}
	ps, err := o.Roster.LoadRosterProviders()
	if err != nil {
		return out
	}
	for _, p := range ps {
		out[p.ID] = p.KeyEnv != "" && os.Getenv(p.KeyEnv) != ""
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ModelsPageHandler renders the Models roster admin page.
func ModelsPageHandler(opts ModelsOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, providerOpts := opts.providerNames()
		data := struct {
			Version   string
			Models    []modelView
			Providers []providerOption
			CRUD      bool
			Live      bool // edits apply without a restart (ISSUE-115)
			Notice    string
			Error     string
		}{opts.liveVersion(), opts.buildModelViews(), providerOpts, opts.crudEnabled(),
			opts.Reloader != nil, r.URL.Query().Get("ok"), r.URL.Query().Get("err")}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := modelsTmpl.Execute(w, data); err != nil {
			if opts.Logger != nil {
				opts.Logger.Error("models page render failed", "err", err)
			}
			http.Error(w, "models render error", http.StatusInternalServerError)
		}
	}
}

// liveVersion is the active registry version (it changes on a live roster
// reload, ISSUE-115), falling back to the startup version.
func (o ModelsOptions) liveVersion() string {
	if o.Engine != nil && o.Engine.Registry != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil && snap.RegistryVersion() != "" {
			return snap.RegistryVersion()
		}
	}
	return o.Version
}

// ModelsAddHandler upserts a model (add / edit / re-tier) and persists it.
func ModelsAddHandler(opts ModelsOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			redirectModels(w, r, "", "invalid form")
			return
		}
		in, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("input_usd_per_mtok")), 64)
		out, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("output_usd_per_mtok")), 64)
		m := providercfg.Model{
			ID:               r.FormValue("id"),
			ProviderID:       strings.TrimSpace(r.FormValue("provider_id")),
			ProviderModelID:  strings.TrimSpace(r.FormValue("provider_model_id")),
			Tier:             strings.ToLower(strings.TrimSpace(r.FormValue("tier"))),
			InputUSDPerMTok:  in,
			OutputUSDPerMTok: out,
			Enabled:          r.FormValue("enabled") != "",
			ReasoningCapable: r.FormValue("reasoning") != "",
		}
		if err := m.Validate(); err != nil {
			redirectModels(w, r, "", err.Error())
			return
		}
		if opts.Roster == nil {
			redirectModels(w, r, "", "roster is read-only (no data dir configured)")
			return
		}
		if err := opts.Roster.UpsertRosterModel(m); err != nil {
			redirectModels(w, r, "", "save: "+err.Error())
			return
		}
		okMsg, errMsg := applyRosterEdit(r.Context(), opts.Reloader, AdminUserFromContext(r.Context()), "Model "+m.ID+" saved")
		redirectModels(w, r, okMsg, errMsg)
	}
}

// ModelsDeleteHandler removes a model by id.
func ModelsDeleteHandler(opts ModelsOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id := r.FormValue("id")
		if opts.Roster == nil {
			redirectModels(w, r, "", "roster is read-only (no data dir configured)")
			return
		}
		if err := opts.Roster.DeleteRosterModel(id); err != nil {
			redirectModels(w, r, "", "save: "+err.Error())
			return
		}
		okMsg, errMsg := applyRosterEdit(r.Context(), opts.Reloader, AdminUserFromContext(r.Context()), "Model "+id+" removed")
		redirectModels(w, r, okMsg, errMsg)
	}
}

func redirectModels(w http.ResponseWriter, r *http.Request, ok, errMsg string) {
	u := "/router/models"
	if ok != "" {
		u += "?ok=" + urlQueryEscape(ok)
	} else if errMsg != "" {
		u += "?err=" + urlQueryEscape(errMsg)
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}

var modelsTmpl = template.Must(template.New("models").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
	"usd":      func(v float64) string { return fmt.Sprintf("$%.2f", v) },
	"tierClass": func(t string) string {
		switch t {
		case "premium":
			return "premium"
		case "balanced":
			return "balanced"
		default:
			return "cheap"
		}
	},
	"tierSelected": func(a, b string) template.HTMLAttr {
		if a == b {
			return template.HTMLAttr(" selected")
		}
		return ""
	},
}).Parse(modelsHTML))

const modelsHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Models</title>
<style>
  {{adminCSS}}
  body{margin:0;background:#0b1220;color:#e8eef7;font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
  header{padding:20px 26px;border-bottom:1px solid #22304d}
  h1{font-size:1.3rem;margin:0}
  .sub{color:#8fa1bf;font-size:.85rem;margin-top:4px}
  .wrap{padding:26px 32px;display:flex;flex-direction:column;gap:18px}
  .card{background:#0e1626;border:1px solid #22304d;border-radius:12px;overflow-x:auto}
  form.addf{max-width:1100px}
  table{border-collapse:collapse;width:100%;font-size:13.5px}
  th,td{text-align:left;padding:10px 11px;border-bottom:1px solid #16223b;white-space:nowrap;vertical-align:top}
  .psrc{font-size:10.5px;color:#64748b;font-family:ui-monospace,Menlo,monospace}
  thead th{color:#8fa1bf;font-size:11px;letter-spacing:.06em;text-transform:uppercase;font-weight:600}
  tbody tr:last-child td{border-bottom:none}
  .pill{display:inline-block;padding:2px 9px;border-radius:999px;font-weight:600;font-size:12px;font-family:ui-monospace,Menlo,monospace}
  .pill.cheap{background:#13351f;color:#4fd08a}.pill.balanced{background:#33290f;color:#f4b740}.pill.premium{background:#361529;color:#f07ab0}
  .num{text-align:right;font-variant-numeric:tabular-nums;font-family:ui-monospace,Menlo,monospace}
  .slug{color:#8fa1bf;font-size:12px;font-family:ui-monospace,Menlo,monospace}
  .muted{color:#64748b;font-size:12px}
  .tag{font-size:11px;font-weight:600;padding:2px 8px;border-radius:999px;font-family:ui-monospace,Menlo,monospace}
  .tag.ok{background:#13351f;color:#4fd08a}.tag.bad{background:#361525;color:#f07ab0}
  .tres{font-size:12px;font-family:ui-monospace,Menlo,monospace;margin-left:6px;white-space:nowrap}
  .tres.ok{color:#4fd08a}.tres.bad{color:#f07ab0;cursor:help}
  .tag.warn{background:#33290f;color:#f4b740}.tag.neutral{background:#16233c;color:#9db4dc}
  .banner{padding:12px 18px;border-radius:10px;font-size:13.5px}
  .banner.ok{background:#13351f;color:#8ff0bf;border:1px solid #1f5c38}
  .banner.err{background:#361525;color:#ffb4d4;border:1px solid #5c1f3a}
  .empty{color:#64748b;padding:1.2rem 16px;text-align:center}
  .note{color:#8fa1bf;font-size:13px;line-height:1.6;background:#0e1626;border:1px dashed #22304d;border-radius:10px;padding:16px 20px}
  .note b{color:#e8eef7}
  form.addf{background:#0e1626;border:1px solid #22304d;border-radius:12px;padding:20px}
  form.addf h2{margin:0 0 14px;font-size:1rem}
  .grid{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}
  label{display:block;font-size:12px;color:#8fa1bf;margin-bottom:5px}
  input,select{width:100%;background:#0b1220;border:1px solid #22304d;color:#e8eef7;border-radius:8px;
    padding:9px 11px;font-size:13.5px;font-family:inherit;outline:none;box-sizing:border-box}
  input:focus,select:focus{border-color:#22c58b}
  .full{grid-column:1 / -1}
  .btn{background:#22c58b;color:#0b1220;border:0;border-radius:8px;padding:9px 16px;font-weight:700;font-size:13.5px;cursor:pointer}
  .btn.sm{padding:5px 10px;font-size:12px}
  .btn.del{background:transparent;color:#f07ab0;border:1px solid #4a1c33;padding:5px 10px;font-size:12px}
  .btn.del:hover{background:#2a1424}
  .rowf{display:inline-flex;gap:6px;align-items:center;margin:0}
  .rowf select,.rowf input{padding:5px 7px;font-size:12px;width:auto}
  .rowf input.price{width:70px}
  .actions{display:flex;gap:8px;align-items:center}
  @media(max-width:820px){.grid{grid-template-columns:1fr 1fr}}
.tag.eg-local{background:#0f3a26;color:#4ade80}.tag.eg-cloud{background:#12264a;color:#7fb2ff}
tr.editrow td{background:#0a1322;padding:12px 16px}
tr.editrow .delf{margin-top:10px}
.editf{display:flex;flex-wrap:wrap;gap:8px;align-items:center}
.editf .fl{font-size:11px;color:#8fa1bf;text-transform:uppercase;letter-spacing:.05em;margin-left:4px}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "models"}}
<div class="tk-main">
<header>
  <h1>Models</h1>
  <div class="sub">The roster — routable models and their tier · {{.Version}}</div>
</header>
<div class="wrap">
  {{if .Notice}}<div class="banner ok">{{.Notice}}</div>{{end}}
  {{if .Error}}<div class="banner err">{{.Error}}</div>{{end}}
  <div class="note">
    <b>Tier is the router's USP.</b> The router picks a model per task automatically (task → tier);
    here you curate the <b>tier per model</b> — not a model per task. A model points at a
    <a href="/router/providers" style="color:#7fd3ff">provider connection</a>. Changes apply <b>{{if .Live}}immediately{{else}}on restart{{end}}</b>.
  </div>
  <div class="card">
    <table>
      <thead><tr>
        <th>Model</th><th>Provider</th><th>Provider model</th><th>Tier</th>
        <th class="num">In $/Mtok</th><th class="num">Out $/Mtok</th><th>Status</th>
        {{if .CRUD}}<th>Actions</th>{{end}}
      </tr></thead>
      <tbody>
      {{range .Models}}
      <tr>
        <td>{{.ID}}{{if .Builtin}}<br><span class="tag neutral">built-in</span>{{end}}</td>
        <td>{{.ProviderName}}{{if eq .Egress "local"}}<br><span class="tag eg-local" title="Carries a local/on-prem tag — prompts routed here stay in the house">local</span>{{else if eq .Egress "cloud"}}<br><span class="tag eg-cloud" title="No local tag — prompts routed here leave the house">cloud</span>{{end}}</td>
        <td class="slug">{{.Slug}}</td>
        <td><span class="pill {{tierClass .Tier}}">{{.Tier}}</span></td>
        <td class="num">{{usd .InPerMTok}}</td>
        <td class="num">{{usd .OutPerMTok}}<div class="psrc" title="{{if .Synced}}Price synced from the provider catalog{{else}}Price set by you{{end}}">{{if .Synced}}synced{{else}}manual{{end}}</div></td>
        <td>
          {{if .Routable}}<span class="tag ok">routable</span>
          {{else if not .Enabled}}<span class="tag warn">disabled</span>
          {{else}}<span class="tag bad">key missing</span>{{end}}
          {{if .Pending}}<span class="tag warn">⏳ restart</span>{{end}}
          <button class="btn sm test" type="button" data-model="{{.ID}}"
            title="Live-test this row from the router: a 1-token completion with this exact slug against its provider. Catches a wrong slug before production does.">Test</button>
          <div class="tres" id="tres-{{.ID}}"></div>
        </td>
        {{if $.CRUD}}
        <td>
          {{if .Editable}}
          <div class="actions">
          <button class="btn sm" type="button" onclick="editRow(this)" aria-expanded="false">Edit</button>

          </div>
          {{else}}<span class="muted">read-only</span>{{end}}
        </td>
        {{end}}
      </tr>
      {{if and $.CRUD .Editable}}
      <tr class="editrow" hidden><td colspan="8">
          <form class="rowf editf" method="post" action="/router/models">
            <input type="hidden" name="id" value="{{.ID}}">
            <input type="hidden" name="provider_id" value="{{.ProviderID}}">
            <span class="fl">Slug</span><input name="provider_model_id" value="{{.Slug}}" aria-label="Provider model slug" style="width:150px;font-family:ui-monospace,Menlo,monospace"
              title="Provider model slug — must be exactly what the provider's own /models returns (e.g. glm-5.2, not zai/glm-5.2). Use the Test button after saving.">
            <span class="fl">Tier</span><select name="tier" aria-label="Tier">
              <option value="cheap"{{tierSelected "cheap" .Tier}}>cheap</option>
              <option value="balanced"{{tierSelected "balanced" .Tier}}>balanced</option>
              <option value="premium"{{tierSelected "premium" .Tier}}>premium</option>
            </select>
            <span class="fl">In $/Mtok</span><input class="price" name="input_usd_per_mtok" aria-label="Input price per million tokens" value="{{printf "%.4f" .InPerMTok}}" title="in $/Mtok">
            <span class="fl">Out $/Mtok</span><input class="price" name="output_usd_per_mtok" aria-label="Output price per million tokens" value="{{printf "%.4f" .OutPerMTok}}" title="out $/Mtok">
            <label style="margin:0;display:inline-flex;gap:4px;align-items:center;color:#b7c4dc"><input type="checkbox" name="enabled" style="width:auto"{{if .Enabled}} checked{{end}}>on</label>
            <label style="margin:0;display:inline-flex;gap:4px;align-items:center;color:#b7c4dc" title="The model has a switchable thinking mode (Qwen3-style). The router enables thinking for hard tasks and disables it for simple ones — per request."><input type="checkbox" name="reasoning" style="width:auto"{{if .Reasoning}} checked{{end}}>thinking</label>
            <button class="btn sm" type="submit">Save</button>
          </form>
          <form class="rowf delf" method="post" action="/router/models/delete" onsubmit="return confirm('Remove {{.ID}}?')">
            <input type="hidden" name="id" value="{{.ID}}">
            <button class="btn del" type="submit">Remove</button>
          </form>
      </td></tr>
      {{end}}
      {{else}}<tr><td colspan="{{if .CRUD}}8{{else}}7{{end}}" class="empty">No models.</td></tr>
      {{end}}
      </tbody>
    </table>
  </div>

  {{if .CRUD}}
  <form class="addf" method="post" action="/router/models">
    <h2>Add model</h2>
    <div class="grid">
      <div><label>ID (short name, a–z0–9)</label><input name="id" placeholder="glm-balanced" required></div>
      <div><label>Provider connection</label>
        <select name="provider_id" required>
          {{range .Providers}}<option value="{{.ID}}">{{.Name}} ({{.ID}})</option>{{else}}<option value="">— add a provider first —</option>{{end}}
        </select></div>
      <div><label>Provider model (slug)</label><input name="provider_model_id" placeholder="z-ai/glm-4.6" required></div>
      <div><label>Tier</label>
        <select name="tier">
          <option value="cheap">cheap</option>
          <option value="balanced" selected>balanced</option>
          <option value="premium">premium</option>
        </select></div>
      <div><label>Input $/Mtok</label><input name="input_usd_per_mtok" placeholder="0.6"></div>
      <div><label>Output $/Mtok</label><input name="output_usd_per_mtok" placeholder="2.2"></div>
      <div><label>Enabled</label><label style="display:inline-flex;gap:6px;align-items:center;color:#b7c4dc;margin-top:6px"><input type="checkbox" name="enabled" style="width:auto" checked> routable</label></div>
      <div><label>Thinking model</label><label style="display:inline-flex;gap:6px;align-items:center;color:#b7c4dc;margin-top:6px" title="Qwen3-style switchable thinking: the router enables thinking for hard tasks (requires_reasoning) and disables it for simple ones — same model, fast when possible."><input type="checkbox" name="reasoning" style="width:auto"> control thinking per task</label></div>
    </div>
    <div style="margin-top:14px"><button class="btn" type="submit">Save model</button></div>
  </form>
  {{end}}
</div>
</div>
</div>
<script>
// Per-row live test (ISSUE-098): a 1-token completion with the row's exact slug,
// made from the router's own network. Renders ✅ + latency or ❌ + the upstream
// error inline, so a wrong slug is caught at save time, not in production.
document.querySelectorAll('button.test').forEach(function(btn){
  btn.addEventListener('click', async function(){
    var id=btn.getAttribute('data-model'), out=document.getElementById('tres-'+id);
    btn.disabled=true; out.className='tres'; out.textContent='testing…';
    try{
      var res=await fetch('/router/models/test',{method:'POST',credentials:'same-origin',
        headers:{'Content-Type':'application/x-www-form-urlencoded'},
        body:'id='+encodeURIComponent(id)});
      var j=await res.json();
      if(j.ok){
        out.className='tres ok'; out.textContent='✅ '+j.latency_ms+' ms';
        out.title='HTTP '+j.status+' from '+(j.url||'');
      }else{
        out.className='tres bad';
        out.textContent='❌ '+(j.status?('HTTP '+j.status):'unreachable');
        out.title=((j.diagnosis||'')+'\n'+(j.error||'')).trim();
      }
    }catch(e){out.className='tres bad';out.textContent='❌ test failed';out.title=e.message;}
    btn.disabled=false;
  });
});
</script>
<script>
function editRow(b){var r=b.closest('tr').nextElementSibling;if(!r||!r.classList.contains('editrow'))return;r.hidden=!r.hidden;b.setAttribute('aria-expanded',String(!r.hidden));b.textContent=r.hidden?'Edit':'Close';}
</script>
</body></html>`
