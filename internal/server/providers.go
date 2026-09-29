package server

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/health"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/providercfg"
)

// ProvidersOptions configures the providers admin page and its CRUD endpoints.
// After ISSUE-072 the providers page is a slim connection layer (like LiteLLM's
// "LLM Credentials"): base_url + key_env only. Models live on the Models page.
type ProvidersOptions struct {
	Engine  *engine.Engine // for the active registry snapshot
	Health  *health.Tracker
	Logger  *slog.Logger
	Version string
	// Roster, when set (a data dir is configured), is the SQLite-backed source of
	// truth for provider connections (ISSUE-073). It enables add/delete of custom
	// providers. Nil (no data dir) → the providers page is read-only.
	Roster providercfg.RosterStore
	// Cache supplies the active policy so the risk register (ISSUE-093) can show
	// which tags the policy requires — and warn when no provider carries one.
	Cache *policy.Cache
	// Auditor records tag changes (supply-chain facts policy routes on).
	Auditor audit.Sink
}

// riskRegisterTag is one entry of the curated supply-chain vocabulary
// (ISSUE-093 pt 4). Tags are facts the OPERATOR asserts about a provider —
// the router never claims them on its own — and policy rules require/deny them.
type riskRegisterTag struct{ Tag, Desc string }

var riskRegisterTags = []riskRegisterTag{
	{"local", "Stays in the house — on-prem/VPC endpoint, no external egress."},
	{"air-gapped", "No internet path at all — the strictest class."},
	{"eu-resident", "Processing inside the EU (residency / data-transfer posture)."},
	{"dpa-signed", "A signed data-processing agreement is on file."},
	{"iso27001", "Provider holds a current ISO 27001 certification."},
	{"soc2", "Provider holds a current SOC 2 report."},
	{"no-train", "Contract states your data is never used for model training."},
	{"subprocessors-vetted", "The provider's sub-processors are reviewed and listed."},
}

func isCuratedTag(tag string) bool {
	for _, t := range riskRegisterTags {
		if t.Tag == tag {
			return true
		}
	}
	return false
}

type providerView struct {
	ID        string
	Name      string
	BaseURL   string
	KeyEnv    string
	KeySet    bool
	Status    string
	Health    float64
	HasHealth bool
	Deletable bool
	Pending   bool
	Synced    bool     // prices for this connection's models are synced (openrouter)
	ModelN    int      // number of routable models referencing this connection
	Tags      []string // compliance/residency tags (ISSUE-078)
}

func (o ProvidersOptions) crudEnabled() bool { return o.Roster != nil }

// buildProviderViews assembles the live registry providers plus any config
// providers not yet applied (pending restart).
func (o ProvidersOptions) buildProviderViews() []providerView {
	custom := map[string]bool{}
	if o.Roster != nil {
		if cps, err := o.Roster.LoadRosterProviders(); err == nil {
			for _, c := range cps {
				custom[c.ID] = true
			}
		}
	}
	var healthMap map[string]float64
	if o.Health != nil {
		healthMap = o.Health.Providers()
	}
	inRegistry := map[string]bool{}
	var views []providerView
	if o.Engine != nil && o.Engine.Registry != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil {
			for _, p := range snap.Providers() {
				inRegistry[p.ID] = true
				pv := providerView{
					ID: p.ID, Name: p.Name, BaseURL: p.BaseURL, KeyEnv: p.AuthSecretRef,
					Status: string(p.Status), Deletable: custom[p.ID], Synced: p.ID == "openrouter",
					ModelN: len(snap.ModelsForProvider(p.ID)),
					Tags:   p.ComplianceTags,
				}
				if p.AuthSecretRef != "" {
					pv.KeySet = os.Getenv(p.AuthSecretRef) != ""
				}
				if h, ok := healthMap[p.ID]; ok {
					pv.Health, pv.HasHealth = h, true
				}
				views = append(views, pv)
			}
		}
	}
	// Config providers not yet in the registry → pending restart.
	if o.Roster != nil {
		if cps, err := o.Roster.LoadRosterProviders(); err == nil {
			for _, c := range providercfg.SortByID(cps) {
				if inRegistry[c.ID] {
					continue
				}
				views = append(views, providerView{ID: c.ID, Name: c.Name, BaseURL: c.BaseURL, KeyEnv: c.KeyEnv,
					KeySet: os.Getenv(c.KeyEnv) != "", Status: "pending", Deletable: true, Pending: true, Synced: c.ID == "openrouter",
					Tags: c.ComplianceTags})
			}
		}
	}
	return views
}

// registerRow is one provider row in the risk-register matrix.
type registerRow struct {
	ID, Name string
	Has      map[string]bool // curated tag → asserted
	Extra    string          // non-curated tags, comma-joined (kept verbatim)
	Editable bool
}

// buildRiskRegister assembles the matrix (roster is the editable source of
// truth; registry-only providers show read-only), the set of tags the active
// policy requires, and the required tags no provider carries (fail-closed gap).
func (o ProvidersOptions) buildRiskRegister(views []providerView) (rows []registerRow, required map[string]bool, missing []string) {
	// The register edits the ROSTER (source of truth) — a just-saved tag shows
	// immediately even though routing picks it up on restart (the card says so).
	rosterTags := map[string][]string{}
	if o.Roster != nil {
		if ps, err := o.Roster.LoadRosterProviders(); err == nil {
			for _, p := range ps {
				rosterTags[p.ID] = p.ComplianceTags
			}
		}
	}
	covered := map[string]bool{}
	for _, v := range views {
		tags, inRoster := rosterTags[v.ID]
		if !inRoster {
			tags = v.Tags // registry-only provider: read-only live tags
		}
		row := registerRow{ID: v.ID, Name: v.Name, Has: map[string]bool{}, Editable: inRoster && o.crudEnabled()}
		var extra []string
		for _, t := range tags {
			covered[t] = true
			if isCuratedTag(t) {
				row.Has[t] = true
			} else {
				extra = append(extra, t)
			}
		}
		row.Extra = strings.Join(extra, ", ")
		rows = append(rows, row)
	}
	required = map[string]bool{}
	if o.Cache != nil {
		if p, ok := o.Cache.Active(policy.Scope{}); ok {
			for _, t := range p.RequiredProviderTags() {
				required[t] = true
				if !covered[t] {
					missing = append(missing, t)
				}
			}
		}
	}
	sort.Strings(missing)
	return rows, required, missing
}

// ProvidersPageHandler renders the providers admin page.
func ProvidersPageHandler(opts ProvidersOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		views := opts.buildProviderViews()
		rows, required, missing := opts.buildRiskRegister(views)
		data := struct {
			Version   string
			Providers []providerView
			CRUD      bool
			Notice    string
			Error     string
			RiskTags  []riskRegisterTag
			Register  []registerRow
			Required  map[string]bool
			Missing   []string
		}{opts.Version, views, opts.crudEnabled(),
			r.URL.Query().Get("ok"), r.URL.Query().Get("err"),
			riskRegisterTags, rows, required, missing}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := providersTmpl.Execute(w, data); err != nil {
			if opts.Logger != nil {
				opts.Logger.Error("providers page render failed", "err", err)
			}
			http.Error(w, "providers render error", http.StatusInternalServerError)
		}
	}
}

// ProvidersTagsHandler updates a provider's compliance tags from the risk
// register (curated checkboxes + free-text extras). Audited: the tags are
// supply-chain assertions policy routes on.
func ProvidersTagsHandler(opts ProvidersOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.Roster == nil {
			redirectProviders(w, r, "", "roster is read-only (no data dir configured)")
			return
		}
		if err := r.ParseForm(); err != nil {
			redirectProviders(w, r, "", "invalid form")
			return
		}
		id := strings.TrimSpace(r.FormValue("id"))
		ps, err := opts.Roster.LoadRosterProviders()
		if err != nil {
			redirectProviders(w, r, "", "load roster: "+err.Error())
			return
		}
		var target *providercfg.Provider
		for i := range ps {
			if ps[i].ID == id {
				target = &ps[i]
				break
			}
		}
		if target == nil {
			redirectProviders(w, r, "", "unknown provider "+id)
			return
		}
		var tags []string
		for _, t := range r.Form["tag"] {
			if isCuratedTag(t) {
				tags = append(tags, t)
			}
		}
		tags = append(tags, providercfg.ParseTags(r.FormValue("extra_tags"))...)
		target.ComplianceTags = tags
		if err := opts.Roster.UpsertRosterProvider(*target); err != nil {
			redirectProviders(w, r, "", "save: "+err.Error())
			return
		}
		audit.Record(r.Context(), opts.Auditor, audit.Entry{
			Action: audit.ActionProviderTags,
			Actor:  AdminUserFromContext(r.Context()),
			Target: id,
			Reason: "compliance tags set to [" + strings.Join(tags, ", ") + "]",
		})
		redirectProviders(w, r, "Tags updated for "+id+" — restart (redeploy) to apply to routing.", "")
	}
}

// ProvidersAddHandler upserts a provider connection from the add form.
func ProvidersAddHandler(opts ProvidersOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			redirectProviders(w, r, "", "invalid form")
			return
		}
		p := providercfg.Provider{
			ID:             r.FormValue("id"),
			Name:           r.FormValue("name"),
			BaseURL:        strings.TrimSpace(r.FormValue("base_url")),
			KeyEnv:         strings.TrimSpace(r.FormValue("key_env")),
			ComplianceTags: providercfg.ParseTags(r.FormValue("compliance_tags")),
		}
		if err := p.Validate(); err != nil {
			redirectProviders(w, r, "", err.Error())
			return
		}
		if opts.Roster == nil {
			redirectProviders(w, r, "", "roster is read-only (no data dir configured)")
			return
		}
		if err := opts.Roster.UpsertRosterProvider(p); err != nil {
			redirectProviders(w, r, "", "save: "+err.Error())
			return
		}
		redirectProviders(w, r, "Provider "+p.ID+" saved — restart (redeploy) to activate it.", "")
	}
}

// ProvidersDeleteHandler removes a provider connection by id.
func ProvidersDeleteHandler(opts ProvidersOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id := r.FormValue("id")
		if opts.Roster == nil {
			redirectProviders(w, r, "", "roster is read-only (no data dir configured)")
			return
		}
		if err := opts.Roster.DeleteRosterProvider(id); err != nil {
			redirectProviders(w, r, "", "save: "+err.Error())
			return
		}
		redirectProviders(w, r, "Provider "+id+" removed — restart (redeploy) to apply.", "")
	}
}

func redirectProviders(w http.ResponseWriter, r *http.Request, ok, errMsg string) {
	u := "/router/providers"
	if ok != "" {
		u += "?ok=" + urlQueryEscape(ok)
	} else if errMsg != "" {
		u += "?err=" + urlQueryEscape(errMsg)
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}

func urlQueryEscape(s string) string {
	return strings.NewReplacer(" ", "%20", "&", "%26", "?", "%3F", "\n", " ", "\"", "%22", "<", "", ">", "").Replace(s)
}

var providersTmpl = template.Must(template.New("providers").Funcs(template.FuncMap{
	"adminCSS":  adminCSSFunc,
	"adminNav":  adminNavFunc,
	"healthPct": func(v float64) string { return fmt.Sprintf("%.0f%%", v*100) },
}).Parse(providersHTML))

const providersHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Providers</title>
<style>
  {{adminCSS}}
  body{margin:0;background:#0b1220;color:#e8eef7;font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
  header{padding:20px 26px;border-bottom:1px solid #22304d}
  h1{font-size:1.3rem;margin:0}
  .sub{color:#8fa1bf;font-size:.85rem;margin-top:4px}
  .wrap{padding:26px 32px;display:flex;flex-direction:column;gap:18px}
  .card{background:#0e1626;border:1px solid #22304d;border-radius:12px;overflow:hidden}
  .card.pending{border-color:#4a3a12}
  .chead{display:flex;align-items:center;gap:12px;flex-wrap:wrap;padding:16px 20px}
  .pname{font-weight:700;font-size:1.05rem}
  .purl{font-family:ui-monospace,Menlo,monospace;color:#8fa1bf;font-size:12.5px}
  .spacer{flex:1}
  .tag{font-size:12px;font-weight:600;padding:3px 10px;border-radius:999px;font-family:ui-monospace,Menlo,monospace}
  .tag.ok{background:#13351f;color:#4fd08a}.tag.bad{background:#361525;color:#f07ab0}
  .tag.warn{background:#33290f;color:#f4b740}.tag.neutral{background:#16233c;color:#9db4dc}
  a.tag.neutral{text-decoration:none}
  .note{color:#8fa1bf;font-size:13px;line-height:1.6;background:#0e1626;border:1px dashed #22304d;border-radius:10px;padding:16px 20px}
  .note b{color:#e8eef7}
  .banner{padding:12px 18px;border-radius:10px;font-size:13.5px}
  .banner.ok{background:#13351f;color:#8ff0bf;border:1px solid #1f5c38}
  .banner.err{background:#361525;color:#ffb4d4;border:1px solid #5c1f3a}
  .empty{color:#64748b;padding:1.2rem 20px;text-align:center}
  form.addf{background:#0e1626;border:1px solid #22304d;border-radius:12px;padding:20px;max-width:1100px}
  form.addf h2{margin:0 0 14px;font-size:1rem}
  .grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}
  label{display:block;font-size:12px;color:#8fa1bf;margin-bottom:5px}
  input{width:100%;background:#0b1220;border:1px solid #22304d;color:#e8eef7;border-radius:8px;
    padding:10px 12px;font-size:13.5px;font-family:inherit;outline:none}
  input:focus{border-color:#22c58b}
  .full{grid-column:1 / -1}
  .btn{background:#22c58b;color:#0b1220;border:0;border-radius:8px;padding:10px 18px;font-weight:700;font-size:14px;cursor:pointer}
  .btn.del{background:transparent;color:#f07ab0;border:1px solid #4a1c33;padding:5px 12px;font-size:12px}
  .btn.del:hover{background:#2a1424}
  .rcard{background:#0e1626;border:1px solid #22304d;border-radius:12px;padding:18px 20px}
  .rnote{color:#8fa1bf;font-size:13px;line-height:1.6;margin-bottom:10px}
  .rnote b{color:#e8eef7}
  .rwarn{background:#33270e;border:1px solid #7a5a1f;color:#f4d38a;border-radius:10px;padding:11px 14px;font-size:13px;margin-bottom:12px}
  .rtab{border-collapse:collapse;font-size:13px;min-width:100%}
  .rtab th{color:#8fa1bf;font-size:11px;letter-spacing:.04em;text-transform:uppercase;font-weight:600;
    padding:8px 10px;text-align:center;border-bottom:1px solid #22304d;cursor:help}
  .rtab td{padding:8px 10px;text-align:center;border-bottom:1px solid #16223b}
  .rtab tbody tr:last-child td{border-bottom:none}
  .rtab .req{color:#f4b740;font-size:10px;letter-spacing:.03em;margin-top:2px}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "providers"}}
<div class="tk-main">
<header>
  <h1>Providers</h1>
  <div class="sub">Reusable connections (endpoint + key env var) · {{.Version}}</div>
</header>
<div class="wrap">
  {{if .Notice}}<div class="banner ok">{{.Notice}}</div>{{end}}
  {{if .Error}}<div class="banner err">{{.Error}}</div>{{end}}
  <div class="note">
    <b>A provider is just a connection</b> — an OpenAI-compatible endpoint and the name of
    the env var holding its key. Models (and their tier/price) are curated on
    <a href="/router/models" style="color:#7fd3ff">Models</a>.
    <b>Keys are set in the environment, never here</b> — changes therefore apply on
    <b>restart/redeploy</b> (the same moment you add the key). Secrets never leave the
    app or the database.
    <br><br>
    <b>Your own private AI model (on-prem, DGX, air-gapped)?</b> Add its endpoint like any
    provider and give it the compliance tag <b><code>local</code></b>
    (<code>private</code>, <code>on-prem</code>, <code>self-hosted</code> and
    <code>air-gapped</code> also count as "stays in the house"). Policy can then route
    flagged content (e.g. personal IDs) there instead of the cloud — and the demo chat
    shows "the data never left the house". Then add the model on
    <a href="/router/models" style="color:#7fd3ff">Models</a> and give it a tier so it
    becomes routable.
  </div>
  {{range .Providers}}
  <div class="card{{if .Pending}} pending{{end}}">
    <div class="chead">
      <span class="pname">{{.Name}}</span>
      <span class="purl">{{.BaseURL}}</span>
      <span class="spacer"></span>
      {{if .Pending}}<span class="tag warn">⏳ awaiting restart</span>{{end}}
      {{if .KeyEnv}}
        {{if .KeySet}}<span class="tag ok">key set · {{.KeyEnv}}</span>
        {{else}}<span class="tag bad">key missing · {{.KeyEnv}}</span>{{end}}
      {{else}}<span class="tag neutral">no key</span>{{end}}
      {{if .HasHealth}}<span class="tag {{if ge .Health 0.9}}ok{{else if ge .Health 0.5}}warn{{else}}bad{{end}}">health {{healthPct .Health}}</span>{{end}}
      {{if .Synced}}<span class="tag neutral">prices synced</span>{{else}}<span class="tag neutral">manual prices</span>{{end}}
      {{range .Tags}}<span class="tag neutral">{{.}}</span>{{end}}
      <a class="tag neutral" href="/router/models">{{.ModelN}} models</a>
      <span class="tag neutral">{{.Status}}</span>
      {{if and .Deletable $.CRUD}}
      <button class="btn" type="button" style="margin:0" onclick="editProvider(this)" data-id="{{.ID}}" data-name="{{.Name}}" data-url="{{.BaseURL}}" data-key="{{.KeyEnv}}" data-tags="{{range $i, $t := .Tags}}{{if $i}}, {{end}}{{$t}}{{end}}">Edit</button>
      <form method="post" action="/router/providers/delete" onsubmit="return confirm('Remove {{.ID}}?')" style="margin:0">
        <input type="hidden" name="id" value="{{.ID}}">
        <button class="btn del" type="submit">Remove</button>
      </form>
      {{end}}
    </div>
  </div>
  {{else}}
  <div class="empty">No providers configured.</div>
  {{end}}

  <div class="rcard">
    <h2 style="margin:0 0 6px;font-size:1rem">Risk register — supply chain</h2>
    <div class="rnote">Curated compliance facts per provider — what <b>you</b> assert (DPA on file, certifications, residency), never what the router assumes. Policy rules <b>require</b> these tags; a required tag with no provider means those prompts <b>fail closed</b>. Tag changes are audited and apply on restart.</div>
    {{if .Missing}}<div class="rwarn">⚠ The active policy requires {{range $i, $t := .Missing}}{{if $i}}, {{end}}<b>{{$t}}</b>{{end}} — no provider carries {{if eq (len .Missing) 1}}it{{else}}them{{end}}. Prompts matching those rules are blocked (fail-closed) until a provider is tagged.</div>{{end}}
    <div style="overflow-x:auto">
    <table class="rtab">
      <thead><tr><th style="text-align:left">Provider</th>
      {{range .RiskTags}}<th title="{{.Desc}}">{{.Tag}}{{if index $.Required .Tag}}<div class="req">required</div>{{end}}</th>{{end}}
      <th style="text-align:left">other tags</th><th></th></tr></thead>
      <tbody>
      {{range .Register}}
      <tr>
        {{if .Editable}}
        <td style="text-align:left;white-space:nowrap">{{.Name}}</td>
        {{$row := .}}{{range $.RiskTags}}<td><input form="tags-{{$row.ID}}" type="checkbox" name="tag" value="{{.Tag}}"{{if index $row.Has .Tag}} checked{{end}}></td>{{end}}
        <td><input form="tags-{{.ID}}" name="extra_tags" value="{{.Extra}}" placeholder="e.g. dev-only" style="width:130px;background:#0b1220;border:1px solid #22304d;color:#e8eef7;border-radius:6px;padding:4px 7px;font-size:12px"></td>
        <td><button form="tags-{{.ID}}" class="btn" style="padding:4px 12px;font-size:12px" type="submit">Save</button></td>
        {{else}}
        <td style="text-align:left;white-space:nowrap">{{.Name}} <span class="tag neutral">read-only</span></td>
        {{$row := .}}{{range $.RiskTags}}<td>{{if index $row.Has .Tag}}✓{{else}}—{{end}}</td>{{end}}
        <td>{{.Extra}}</td><td></td>
        {{end}}
      </tr>
      {{end}}
      </tbody>
    </table>
    </div>
    {{range .Register}}{{if .Editable}}<form id="tags-{{.ID}}" method="post" action="/router/providers/tags"><input type="hidden" name="id" value="{{.ID}}"></form>{{end}}{{end}}
  </div>

  {{if .CRUD}}
  <form class="addf" method="post" action="/router/providers" id="provform">
    <h2 id="provh">Add provider connection</h2>
    <p id="provhint" style="display:none;font-size:.82rem;color:#8fa1bf;margin:-6px 0 10px">Editing an existing connection — saving replaces its endpoint, key env var and tags (the ID stays). Changes apply on restart.</p>
    <div class="grid">
      <div><label>ID (short name, a–z0–9)</label><input name="id" id="prov_id" placeholder="zai" required></div>
      <div><label>Display name</label><input name="name" id="prov_name" placeholder="Z.ai"></div>
      <div class="full"><label>Base URL (OpenAI-compatible)</label><input name="base_url" id="prov_url" placeholder="https://api.z.ai/api/coding/paas/v4" required></div>
      <div class="full"><label>Key env var (the value is set in the environment, not here)</label><input name="key_env" id="prov_key" placeholder="ZAI_API_KEY" required></div>
      <div class="full"><label>Compliance tags (comma-separated — policy can require/deny them). Residency: eu-resident, dpa-signed. Private/on-prem model: <b>local</b> (stays in the house)</label><input name="compliance_tags" id="prov_tags" placeholder="local, on-prem, eu-resident"></div>
    </div>
    <div style="margin-top:14px"><button class="btn" type="submit" id="provsave">Save connection</button> <button class="btn" type="button" id="provcancel" style="display:none" onclick="resetProvider()">Cancel</button></div>
  </form>
  <script>
  function editProvider(b){var d=b.dataset;
    document.getElementById('prov_id').value=d.id;document.getElementById('prov_id').readOnly=true;
    document.getElementById('prov_name').value=d.name;document.getElementById('prov_url').value=d.url;
    document.getElementById('prov_key').value=d.key;document.getElementById('prov_tags').value=d.tags;
    document.getElementById('provh').textContent='Edit connection: '+d.id;document.getElementById('provsave').textContent='Save changes';
    document.getElementById('provhint').style.display='';document.getElementById('provcancel').style.display='';
    document.getElementById('provform').scrollIntoView({behavior:'smooth'});document.getElementById('prov_url').focus();}
  function resetProvider(){var f=document.getElementById('provform');f.reset();document.getElementById('prov_id').readOnly=false;
    document.getElementById('provh').textContent='Add provider connection';document.getElementById('provsave').textContent='Save connection';
    document.getElementById('provhint').style.display='none';document.getElementById('provcancel').style.display='none';}
  </script>
  {{end}}
</div>
</div>
</div>
</body></html>`
