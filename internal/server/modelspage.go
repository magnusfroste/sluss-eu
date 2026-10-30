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
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
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
	// Recent traffic and failures for the model panel (ISSUE-142): durable
	// history when there is a data dir, the in-memory ring otherwise.
	History    *history.Store
	RequestLog *eventlog.RequestLogTracker
	ErrorRing  *eventlog.ErrorRing
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
	// A save from the model panel (ISSUE-142) reopens the same panel.
	if id := strings.TrimSpace(r.FormValue("panel")); id != "" && (ok != "" || errMsg != "") {
		u += "&m=" + urlQueryEscape(id)
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}

var modelsTmpl = template.Must(template.New("models").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
	"dsHead":   dsHeadFunc,
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
  body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
  header{padding:20px 26px;border-bottom:1px solid #22324f}
  h1{font-size:1.3rem;margin:0}
  .sub{color:#8fa1bf;font-size:.85rem;margin-top:4px}
  .wrap{padding:26px 32px;display:flex;flex-direction:column;gap:18px}
  .card{background:#101c34;border:1px solid #22324f;border-radius:12px;overflow-x:auto}
  form.addf{max-width:1100px}
  table{border-collapse:collapse;width:100%;font-size:13.5px}
  th,td{text-align:left;padding:10px 11px;border-bottom:1px solid #17253f;white-space:nowrap;vertical-align:top}
  .psrc{font-size:10.5px;color:#64748b;font-family:ui-monospace,Menlo,monospace}
  thead th{color:#8fa1bf;font-size:11px;letter-spacing:.06em;text-transform:uppercase;font-weight:600}
  tbody tr:last-child td{border-bottom:none}
  .pill{display:inline-block;padding:2px 9px;border-radius:999px;font-weight:600;font-size:12px;font-family:ui-monospace,Menlo,monospace}
  .pill.cheap{background:#13351f;color:#4fd08a}.pill.balanced{background:#33290f;color:#f4b740}.pill.premium{background:#361529;color:#f07ab0}
  .num{text-align:right;font-variant-numeric:tabular-nums;font-family:ui-monospace,Menlo,monospace}
  .slug{color:#8fa1bf;font-size:12px;font-family:ui-monospace,Menlo,monospace}
  .muted{color:#64748b;font-size:12px}
  .mono{font-family:ui-monospace,Menlo,monospace}
  .tag{font-size:11px;font-weight:600;padding:2px 8px;border-radius:999px;font-family:ui-monospace,Menlo,monospace}
  .tag.ok{background:#13351f;color:#4fd08a}.tag.bad{background:#361525;color:#f07ab0}
  .tres{font-size:12px;font-family:ui-monospace,Menlo,monospace;margin-left:6px;white-space:nowrap}
  .tres.ok{color:#4fd08a}.tres.bad{color:#f07ab0;cursor:help}
  .tag.warn{background:#33290f;color:#f4b740}.tag.neutral{background:#16233c;color:#9db4dc}
  .banner{padding:12px 18px;border-radius:10px;font-size:13.5px}
  .banner.ok{background:#13351f;color:#8ff0bf;border:1px solid #1f5c38}
  .banner.err{background:#361525;color:#ffb4d4;border:1px solid #5c1f3a}
  .empty{color:#64748b;padding:1.2rem 16px;text-align:center}
  .note{color:#8fa1bf;font-size:13px;line-height:1.6;background:#101c34;border:1px dashed #22324f;border-radius:10px;padding:16px 20px}
  .note b{color:#e8eef7}
  form.addf{background:#101c34;border:1px solid #22324f;border-radius:12px;padding:20px}
  form.addf h2{margin:0 0 14px;font-size:1rem}
  .grid{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}
  label{display:block;font-size:12px;color:#8fa1bf;margin-bottom:5px}
  input,select{width:100%;background:#081328;border:1px solid #22324f;color:#e8eef7;border-radius:8px;
    padding:9px 11px;font-size:13.5px;font-family:inherit;outline:none;box-sizing:border-box}
  input:focus,select:focus{border-color:#fad100}
  .full{grid-column:1 / -1}
  .btn{background:#fad100;color:#081328;border:0;border-radius:8px;padding:9px 16px;font-weight:700;font-size:13.5px;cursor:pointer}
  .btn.sm{padding:5px 10px;font-size:12px}
  .btn.del{background:transparent;color:#f07ab0;border:1px solid #4a1c33;padding:5px 10px;font-size:12px}
  .btn.del:hover{background:#2a1424}
  .rowf{display:inline-flex;gap:6px;align-items:center;margin:0}
  .rowf select,.rowf input{padding:5px 7px;font-size:12px;width:auto}
  .rowf input.price{width:70px}
  .actions{display:flex;gap:8px;align-items:center}
  @media(max-width:820px){.grid{grid-template-columns:1fr 1fr}}
.tag.eg-local{background:#0f3a26;color:#4ade80}.tag.eg-cloud{background:#12264a;color:#7fb2ff}
/* Model panel (ISSUE-142) */
.sr{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0)}
tr.mrow{cursor:pointer}
tr.mrow:hover td,tr.mrow:focus td,tr.mrow.sel td{background:#13213d}
tr.mrow:focus{outline:none}tr.mrow:focus-visible td:first-child{box-shadow:inset 3px 0 0 #fad100}
.mid{font-weight:600}
.btn.ghost{background:transparent;color:#cfe0f7;border:1px solid #2a3b5c}
.btn.ghost:hover{background:#16233c}
.mp-back{position:fixed;inset:0;background:rgba(3,8,18,.55);z-index:40}
.mp{position:fixed;top:0;right:0;bottom:0;width:min(520px,100vw);background:#0d1830;border-left:1px solid #22324f;
  z-index:41;display:flex;flex-direction:column;box-shadow:-24px 0 48px rgba(0,0,0,.35);animation:mpin .16s ease-out}
@keyframes mpin{from{transform:translateX(24px);opacity:.4}to{transform:none;opacity:1}}
@media (prefers-reduced-motion:reduce){.mp{animation:none}}
.mp[hidden],.mp-back[hidden],.mp-res[hidden]{display:none}
.mp-h{padding:18px 20px 14px;border-bottom:1px solid #22324f;display:flex;gap:12px;align-items:flex-start}
.mp-h h2{margin:0;font-size:1.15rem;word-break:break-all}
.mp-h .meta{margin-top:6px;display:flex;flex-wrap:wrap;gap:6px;align-items:center;color:#8fa1bf;font-size:12.5px}
.mp-x{margin-left:auto;background:transparent;border:1px solid #2a3b5c;color:#cfe0f7;border-radius:8px;width:32px;height:32px;font-size:18px;cursor:pointer;flex:none}
.mp-b{overflow-y:auto;padding:4px 20px 20px;flex:1}
.mp-s{padding:16px 0;border-bottom:1px solid #17253f}
.mp-s:last-child{border-bottom:0}
.mp-s h3{margin:0 0 4px;font-size:.78rem;letter-spacing:.07em;text-transform:uppercase;color:#8fa1bf}
.mp-s .hint{color:#8fa1bf;font-size:12.5px;line-height:1.5;margin:4px 0 10px}
.mp-res{margin-top:10px;font-size:13px;line-height:1.5;border-radius:8px;padding:10px 12px;background:#0a1322;border:1px solid #1d2c48}
.mp-res.ok{border-color:#1f5c38;color:#8ff0bf}.mp-res.bad{border-color:#5c1f3a;color:#ffb4d4}
.mp-res .why{color:#b7c4dc;margin-top:4px;font-size:12.5px}
.served{list-style:none;margin:10px 0 0;padding:0;display:flex;flex-direction:column;gap:6px}
.served li{display:flex;align-items:center;gap:8px;font-family:ui-monospace,Menlo,monospace;font-size:12.5px;padding:6px 8px;border:1px solid #1d2c48;border-radius:8px}
.served li.cur{border-color:#1f5c38}
.served li[hidden]{display:none}
.mp .sfilter{margin-top:4px}
.served li span{flex:1;word-break:break-all}
.mp form.set{display:flex;flex-direction:column;gap:12px}
.mp .f2{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.mp .chk{display:flex;gap:8px;align-items:flex-start;color:#cfe0f7;font-size:13px;margin:0}
.mp .chk input{width:auto;margin-top:2px}
.mp .chk small{display:block;color:#8fa1bf;font-size:12px}
.mp .fh{color:#64748b;font-size:11.5px;margin-top:4px;line-height:1.4}
.act-n{display:flex;gap:18px;margin:6px 0 10px}
.act-n div{font-size:12px;color:#8fa1bf}.act-n b{display:block;font-size:1.25rem;color:#e8eef7;font-variant-numeric:tabular-nums}
.act-l{list-style:none;margin:0;padding:0;font-size:12.5px}
.act-l li{display:flex;gap:10px;padding:6px 0;border-bottom:1px solid #17253f}
.act-l li:last-child{border-bottom:0}
.act-l .t{color:#8fa1bf;min-width:44px;font-variant-numeric:tabular-nums}
.act-l .c{margin-left:auto;color:#8fa1bf;font-family:ui-monospace,Menlo,monospace}
.act-l.fail .r{color:#ffb4d4}
.mp a.lnk{color:#7fd3ff;font-size:13px}
.mp-danger{display:flex;gap:10px;align-items:center;justify-content:space-between}
.mp-danger p{margin:0;color:#8fa1bf;font-size:12.5px}
@media(max-width:560px){.mp .f2{grid-template-columns:1fr}}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "models"}}
<div class="tk-main">
{{dsHead "models" "Models" "Which models Sluss may use, where they run, and what they cost."}}
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
        <th><span class="sr">Details</span></th>
      </tr></thead>
      <tbody>
      {{range .Models}}
      <tr class="mrow" data-model="{{.ID}}" tabindex="0" aria-label="Open {{.ID}}">
        <td><b class="mid">{{.ID}}</b>{{if .Builtin}}<br><span class="tag neutral">built-in</span>{{end}}</td>
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
          <button class="btn sm test" type="button" data-model="{{.ID}}" data-out="tres-{{.ID}}"
            title="Live-test this row from the router: a 1-token completion with this exact slug against its provider. Catches a wrong slug before production does.">Test</button>
          <div class="tres" id="tres-{{.ID}}"></div>
        </td>
        <td><button class="btn sm ghost open" type="button" data-model="{{.ID}}">{{if and $.CRUD .Editable}}Open{{else}}Details{{end}} ›</button></td>
      </tr>
      {{else}}<tr><td colspan="8" class="empty">No models.</td></tr>
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
{{range .Models}}
<div class="mp" id="mp-{{.ID}}" role="dialog" aria-modal="true" aria-labelledby="mph-{{.ID}}" hidden>
  <div class="mp-h">
    <div>
      <h2 id="mph-{{.ID}}">{{.ID}}</h2>
      <div class="meta">
        <span>{{.ProviderName}}</span>
        {{if eq .Egress "local"}}<span class="tag eg-local">local</span>{{else if eq .Egress "cloud"}}<span class="tag eg-cloud">cloud</span>{{end}}
        <span class="pill {{tierClass .Tier}}">{{.Tier}}</span>
        {{if .Routable}}<span class="tag ok">routable</span>{{else if not .Enabled}}<span class="tag warn">disabled</span>{{else}}<span class="tag bad">key missing</span>{{end}}
        {{if .Pending}}<span class="tag warn">⏳ restart</span>{{end}}
      </div>
    </div>
    <button class="mp-x" type="button" data-close aria-label="Close">×</button>
  </div>
  <div class="mp-b">
    <section class="mp-s">
      <h3>Does it answer?</h3>
      <p class="hint">Sends a one-word question as <span class="mono">{{.Slug}}</span> to {{.ProviderName}}, from this server. A lab box that renamed or unloaded the model shows up here before your users notice.</p>
      <button class="btn sm test" type="button" data-model="{{.ID}}" data-out="mpt-{{.ID}}" data-served="1">Test now</button>
      <button class="btn sm ghost served-btn" type="button" data-model="{{.ID}}">What does {{.ProviderName}} serve?</button>
      <div class="mp-res" id="mpt-{{.ID}}" hidden></div>
      <div id="mps-{{.ID}}"></div>
    </section>

    <section class="mp-s">
      <h3>Settings</h3>
      {{if and $.CRUD .Editable}}
      <form class="set" method="post" action="/router/models">
        <input type="hidden" name="id" value="{{.ID}}">
        <input type="hidden" name="provider_id" value="{{.ProviderID}}">
        <input type="hidden" name="panel" value="{{.ID}}">
        <div><label for="slug-{{.ID}}">Model name at the provider</label>
          <input id="slug-{{.ID}}" name="provider_model_id" value="{{.Slug}}" class="mono" required>
          <div class="fh">Exactly what {{.ProviderName}} calls it in its own model list — e.g. <span class="mono">glm-5.2</span>, not <span class="mono">zai/glm-5.2</span>. Test after saving.</div></div>
        <div><label for="tier-{{.ID}}">Tier</label>
          <select id="tier-{{.ID}}" name="tier">
            <option value="cheap"{{tierSelected "cheap" .Tier}}>cheap — simple, high-volume tasks</option>
            <option value="balanced"{{tierSelected "balanced" .Tier}}>balanced — everyday work</option>
            <option value="premium"{{tierSelected "premium" .Tier}}>premium — the hardest tasks</option>
          </select>
          <div class="fh">Sluss picks a tier per task and a model within the tier. Policy can still force personal data to local models.</div></div>
        <div class="f2">
          <div><label for="in-{{.ID}}">Input, $ per million tokens</label><input id="in-{{.ID}}" name="input_usd_per_mtok" inputmode="decimal" value="{{printf "%.4f" .InPerMTok}}"></div>
          <div><label for="out-{{.ID}}">Output, $ per million tokens</label><input id="out-{{.ID}}" name="output_usd_per_mtok" inputmode="decimal" value="{{printf "%.4f" .OutPerMTok}}"></div>
        </div>
        <div class="fh" style="margin-top:-6px">{{if .Synced}}Synced from the provider's price list; a value you save here is used until the next sync.{{else}}Set by you. Use 0 for your own hardware if you do not charge internally.{{end}}</div>
        <label class="chk"><input type="checkbox" name="enabled"{{if .Enabled}} checked{{end}}><span>Sluss may route to this model<small>Off keeps the row but sends no traffic here.</small></span></label>
        <label class="chk"><input type="checkbox" name="reasoning"{{if .Reasoning}} checked{{end}}><span>Switchable thinking mode<small>For Qwen3-style models: thinking on for hard tasks, off for simple ones.</small></span></label>
        <div><button class="btn" type="submit">Save</button></div>
      </form>
      {{else if not $.CRUD}}
      <p class="hint">Read-only: set <span class="mono">ROUTER_DATA_DIR</span> to edit models in the console.</p>
      {{else}}
      <p class="hint">This model comes from the live registry and cannot be edited here.</p>
      {{end}}
    </section>

    <section class="mp-s">
      <h3>Last 24 hours</h3>
      <div class="act" data-model="{{.ID}}"><p class="hint">Loading…</p></div>
      <a class="lnk" href="/router/log?model={{.ID}}">All requests for {{.ID}} in the request log ›</a>
    </section>

    {{if and $.CRUD .Editable}}
    <section class="mp-s mp-danger">
      <p>Remove the model from the roster. Traffic moves to the other models in its tier.</p>
      <form method="post" action="/router/models/delete" onsubmit="return confirm('Remove {{.ID}}?')">
        <input type="hidden" name="id" value="{{.ID}}">
        <button class="btn del" type="submit">Remove</button>
      </form>
    </section>
    {{end}}
  </div>
</div>
{{end}}
<div class="mp-back" id="mp-back" hidden></div>
<script>
var CAN_EDIT={{if .CRUD}}true{{else}}false{{end}};
function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c];});}
function post(url,body){return fetch(url,{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:body}).then(function(r){return r.json();});}

// Live test (ISSUE-098), in the table and in the panel: a 1-token completion
// with the row's exact slug, made from the router's own network.
document.querySelectorAll('button.test').forEach(function(btn){
  btn.addEventListener('click', async function(ev){
    ev.stopPropagation();
    var id=btn.getAttribute('data-model'), out=document.getElementById(btn.getAttribute('data-out')), inPanel=out.classList.contains('mp-res');
    btn.disabled=true; out.hidden=false; out.className=inPanel?'mp-res':'tres'; out.textContent='testing…';
    try{
      var j=await post('/router/models/test','id='+encodeURIComponent(id));
      if(inPanel){
        out.className='mp-res '+(j.ok?'ok':'bad');
        out.innerHTML=j.ok?('✅ Answered in '+j.latency_ms+' ms.')
          :('❌ '+(j.status?('The provider answered HTTP '+j.status+'.'):'Could not reach the provider.')
            +(j.diagnosis?'<div class="why">'+esc(j.diagnosis)+'</div>':'')
            +(j.error?'<div class="why mono">'+esc(j.error).slice(0,300)+'</div>':''));
        // A wrong or renamed model answers 400/404: show what is served instead.
        if(!j.ok&&(j.status===400||j.status===404)&&btn.getAttribute('data-served'))showServed(id);
      }else if(j.ok){
        out.className='tres ok'; out.textContent='✅ '+j.latency_ms+' ms'; out.title='HTTP '+j.status+' from '+(j.url||'');
      }else{
        out.className='tres bad'; out.textContent='❌ '+(j.status?('HTTP '+j.status):'unreachable');
        out.title=((j.diagnosis||'')+'\n'+(j.error||'')).trim();
      }
    }catch(e){out.className=inPanel?'mp-res bad':'tres bad';out.textContent='❌ test failed: '+e.message;}
    btn.disabled=false;
  });
});

// What the provider serves, with "Use this" (ISSUE-134 / 142).
async function showServed(id){
  var box=document.getElementById('mps-'+id);
  box.innerHTML='<p class="hint">Asking the provider for its model list…</p>';
  try{
    var j=await post('/router/models/served','id='+encodeURIComponent(id));
    if(j.error){box.innerHTML='<div class="mp-res bad">'+esc(j.error)+'</div>';return;}
    if(!j.served.length){box.innerHTML='<p class="hint">The provider lists no models.</p>';return;}
    var has=j.served.indexOf(j.slug)>=0, big=j.served.length>12;
    var h='<p class="hint" style="margin-top:12px">'+(has?'The provider serves <b class="mono">'+esc(j.slug)+'</b>'+(big?' among '+j.served.length+' models':'')+'.':'The provider does <b>not</b> list <b class="mono">'+esc(j.slug)+'</b>. Pick the one it serves now:')+'</p>';
    if(big)h+='<input class="sfilter" type="search" placeholder="Filter '+j.served.length+' models…" aria-label="Filter the provider\'s models">';
    h+='<ul class="served">';
    j.served.forEach(function(s){
      var cur=s===j.slug;
      h+='<li'+(cur?' class="cur"':'')+' data-s="'+esc(s.toLowerCase())+'"><span>'+esc(s)+'</span>'+(cur?'<span class="tag ok" style="flex:none">in use</span>'
        :(CAN_EDIT?'<button class="btn sm use" type="button" data-model="'+esc(id)+'" data-slug="'+esc(s)+'">Use this</button>':''))+'</li>';
    });
    box.innerHTML=h+'</ul><p class="hint more" hidden></p>';
    // Long lists (OpenRouter has hundreds): show the current model and the
    // first 12 matches of the filter.
    var items=[].slice.call(box.querySelectorAll('.served li')), more=box.querySelector('.more');
    function filt(q){var n=0,hid=0;q=(q||'').toLowerCase().trim();
      items.forEach(function(li){var m=li.classList.contains('cur')||(li.dataset.s.indexOf(q)>=0&&n<12);if(m&&!li.classList.contains('cur'))n++;
        if(!m&&li.dataset.s.indexOf(q)>=0)hid++;li.hidden=!m;});
      more.hidden=!hid;more.textContent=hid+' more match — narrow the filter.';}
    filt('');var sf=box.querySelector('.sfilter');if(sf)sf.addEventListener('input',function(){filt(sf.value);});
    box.querySelectorAll('button.use').forEach(function(b){
      b.addEventListener('click',async function(){
        b.disabled=true;b.textContent='saving…';
        var r=await post('/router/models/slug','id='+encodeURIComponent(b.dataset.model)+'&slug='+encodeURIComponent(b.dataset.slug));
        if(r.error){b.textContent='failed';b.title=r.error;alert(r.error);b.disabled=false;return;}
        location.href='/router/models?ok='+encodeURIComponent(r.ok||'Saved')+'&m='+encodeURIComponent(b.dataset.model);
      });
    });
  }catch(e){box.innerHTML='<div class="mp-res bad">'+esc(e.message)+'</div>';}
}
document.querySelectorAll('button.served-btn').forEach(function(b){b.addEventListener('click',function(){showServed(b.dataset.model);});});

// Recent traffic for one model.
function ago(t){var s=(Date.now()-new Date(t).getTime())/1000;if(s<60)return 'now';if(s<3600)return Math.floor(s/60)+'m';return Math.floor(s/3600)+'h';}
async function loadActivity(id){
  var box=document.querySelector('.act[data-model="'+id+'"]');
  if(!box||box.dataset.loaded)return;
  try{
    var r=await fetch('/router/models/activity?id='+encodeURIComponent(id),{credentials:'same-origin'});
    var a=await r.json();box.dataset.loaded='1';
    var h='<div class="act-n"><div><b>'+a.count+'</b>requests</div><div><b>'+a.failed+'</b>failed calls</div><div><b>$'+a.cost_usd.toFixed(a.cost_usd<1?4:2)+'</b>cost</div></div>';
    if(a.failures.length){h+='<ul class="act-l fail">';a.failures.forEach(function(f){h+='<li><span class="t">'+ago(f.time)+'</span><span class="r">'+esc(f.reason)+'</span><span class="c">'+f.ms+' ms</span></li>';});h+='</ul>';}
    if(a.recent.length){h+='<ul class="act-l">';a.recent.forEach(function(q){h+='<li><span class="t">'+ago(q.time)+'</span><span>'+esc(q.task)+' · '+esc(q.data)+'</span><span class="c">'+q.tokens+' tok</span></li>';});h+='</ul>';}
    else if(!a.failures.length){h+='<p class="hint">No traffic to this model in the last 24 hours.</p>';}
    if(!a.durable)h+='<p class="hint">In-memory log: set ROUTER_DATA_DIR to keep history across restarts.</p>';
    box.innerHTML=h;
  }catch(e){box.innerHTML='<p class="hint">Could not load traffic: '+esc(e.message)+'</p>';}
}

// Open / close (ISSUE-142). ?m=<id> opens a panel directly, so Overview and
// links can point at one model.
var openId=null,lastFocus=null,back=document.getElementById('mp-back');
function openPanel(id,push){
  var p=document.getElementById('mp-'+id);if(!p)return;
  if(openId)closePanel(false);
  lastFocus=document.activeElement;openId=id;
  p.hidden=false;back.hidden=false;document.body.style.overflow='hidden';
  document.querySelectorAll('tr.mrow').forEach(function(r){r.classList.toggle('sel',r.dataset.model===id);});
  p.querySelector('[data-close]').focus();
  loadActivity(id);
  if(push!==false){var u=new URL(location.href);u.searchParams.set('m',id);u.searchParams.delete('ok');u.searchParams.delete('err');history.replaceState(null,'',u);}
}
function closePanel(restore){
  if(!openId)return;
  document.getElementById('mp-'+openId).hidden=true;back.hidden=true;document.body.style.overflow='';
  var u=new URL(location.href);u.searchParams.delete('m');history.replaceState(null,'',u);
  openId=null;if(restore!==false&&lastFocus)lastFocus.focus();
}
document.querySelectorAll('tr.mrow').forEach(function(r){
  r.addEventListener('click',function(e){if(e.target.closest('button.test,a'))return;openPanel(r.dataset.model);});
  r.addEventListener('keydown',function(e){if(e.key==='Enter'||e.key===' '){e.preventDefault();openPanel(r.dataset.model);}});
});
document.querySelectorAll('[data-close]').forEach(function(b){b.addEventListener('click',function(){closePanel();});});
back.addEventListener('click',function(){closePanel();});
document.addEventListener('keydown',function(e){
  if(!openId)return;
  if(e.key==='Escape'){closePanel();return;}
  if(e.key==='Tab'){ // keep focus inside the open panel
    var f=document.getElementById('mp-'+openId).querySelectorAll('button:not([disabled]),input,select,a[href]');
    if(!f.length)return;var first=f[0],last=f[f.length-1];
    if(e.shiftKey&&document.activeElement===first){e.preventDefault();last.focus();}
    else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first.focus();}
  }
});
(function(){var m=new URLSearchParams(location.search).get('m');if(m)openPanel(m,false);})();
</script>
</body></html>`
