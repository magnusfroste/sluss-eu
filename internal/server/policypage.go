package server

// Policy console (ISSUE-093): the CISO-facing "firewall for data egress". This
// first increment is READ + DRY-RUN — see the active policy (version, rule
// count) and, crucially, paste a prompt to see exactly how the rules route it
// (model, tier, egress, or a fail-closed block) before a single provider call.
// Rule authoring without YAML is the follow-up; the engine + templates already
// make the decision.

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/policy/builtin"
	"github.com/magnusfroste/sluss/internal/retention"
)

// packDesc is a one-line description per built-in compliance pack (module). A
// market/regime enables the pack it needs via ROUTER_POLICY_PATH=builtin:<name>.
var packDesc = map[string]string{
	"pii-local":      "Personal data → local model (baseline protection).",
	"nis2-baseline":  "NIS2: PII + possible secrets stay local, security_review fail-closed.",
	"dora-baseline":  "DORA (finance): + requires a dpa-signed provider (ICT third-party control).",
	"gdpr-sovereign": "GDPR sovereignty: PII → eu-resident (data stays in the EU).",
}

type policyPack struct {
	Name, Desc, Version string
	Rules               int
	Active              bool
}

// builtinPacks lists the embedded compliance packs with a description and marks
// the one whose version matches the active policy.
func builtinPacks(activeVersion string) []policyPack {
	var out []policyPack
	for _, name := range builtin.Names() {
		p := policyPack{Name: name, Desc: packDesc[name]}
		if b, ok := builtin.Load(name); ok {
			if parsed, err := policy.Parse(b); err == nil {
				p.Version = parsed.Version
				p.Rules = len(parsed.Rules)
				p.Active = parsed.Version != "" && parsed.Version == activeVersion
			}
		}
		out = append(out, p)
	}
	return out
}

// modelEgress reports "local" when the selected model carries a stays-in-the-house
// tag, else "cloud" — the same rule the demo chat uses.
func modelEgress(eng *engine.Engine, modelID string) (string, []string) {
	if eng == nil || eng.Registry == nil || modelID == "" {
		return "cloud", nil
	}
	snap, err := eng.Registry.Active()
	if err != nil {
		return "cloud", nil
	}
	m, ok := snap.Model(modelID)
	if !ok {
		return "cloud", nil
	}
	for _, t := range m.ComplianceTags {
		if localTags[t] {
			return "local", m.ComplianceTags
		}
	}
	return "cloud", m.ComplianceTags
}

func modelTier(eng *engine.Engine, modelID string) string {
	if eng == nil || eng.Registry == nil {
		return ""
	}
	if snap, err := eng.Registry.Active(); err == nil {
		if m, ok := snap.Model(modelID); ok {
			return string(m.Tier)
		}
	}
	return ""
}

// PolicyDryRunHandler runs a prompt through the routing engine (no provider call)
// and returns the decision: classification signals, selected model/tier/egress,
// or a fail-closed block with its reason.
func PolicyDryRunHandler(eng *engine.Engine, cache *policy.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if eng == nil {
			http.Error(w, "routing engine not configured", http.StatusServiceUnavailable)
			return
		}
		var body struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Prompt) == "" {
			http.Error(w, "provide a prompt", http.StatusBadRequest)
			return
		}
		dec, job, err := explainRoute(eng, cache, explainInput{
			RequestID: "policy-dry-run",
			Request:   &openai.ChatRequest{Messages: []openai.Message{{Role: "user", Content: body.Prompt}}},
		})
		out := map[string]any{
			"task":            string(job.TaskType),
			"task_confidence": job.TaskConfidence,
			"risk":            string(job.RiskLevel),
			"sensitivity":     string(job.Sensitivity),
			"pii_types":       job.PIITypes,
			"policy_version":  dec.PolicyVersion,
			"reasons":         dec.DecisionReasons,
		}
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "block") || dec.Blocked {
			out["blocked"] = true
			out["block_code"] = dec.BlockCode
			out["block_reason"] = firstNonEmpty(dec.BlockReason, "blocked — no compliant provider (fail-closed)")
		} else if err != nil {
			out["error"] = err.Error()
		} else {
			egress, tags := modelEgress(eng, dec.SelectedModel)
			out["selected_model"] = dec.SelectedModel
			out["provider"] = dec.SelectedProvider
			out["tier"] = modelTier(eng, dec.SelectedModel)
			out["egress"] = egress
			out["model_tags"] = tags
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

type policyPageData struct {
	Version       string
	PolicyVersion string
	RegistryVer   string
	RuleCount     int
	HasPolicy     bool
	Packs         []policyPack
	// Console rules (no-YAML rule editing, ISSUE-093).
	RulesEnabled  bool
	ConsoleRules  []ConsoleRule
	ConsoleActive bool
	Sensitivities []string
	TaskTypes     []string
	RiskLevels    []string
	// Data & retention card (ISSUE-096).
	RetentionDays  int
	PromptLogging  bool
	ResetAvailable bool
	Notice         string
	Error          string
}

// PolicyPageOptions configures the policy console page.
type PolicyPageOptions struct {
	Engine    *engine.Engine
	Cache     *policy.Cache
	Version   string
	Retention *retention.Settings
	// History enables no-YAML rule editing (rules persist in SQLite).
	History *history.Store
	// ResetAvailable shows the audited demo-data reset button (requires a data dir).
	ResetAvailable bool
}

// PolicyPageHandler renders the policy console (active policy + rules + dry-run).
func PolicyPageHandler(o PolicyPageOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d := policyPageData{Version: o.Version,
			Notice: r.URL.Query().Get("ok"), Error: r.URL.Query().Get("err")}
		if o.Cache != nil {
			if p, ok := o.Cache.Active(policy.Scope{}); ok {
				d.HasPolicy = true
				d.PolicyVersion = p.Version()
				d.RegistryVer = p.RegistryVersion()
				d.RuleCount = p.RuleCount()
			}
		}
		d.Packs = builtinPacks(d.PolicyVersion)
		d.RulesEnabled = o.History != nil
		d.ConsoleRules = LoadConsoleRules(o.History)
		d.ConsoleActive = ConsolePolicyActive(o.History)
		d.Sensitivities = consoleSensitivities
		d.TaskTypes = consoleTaskTypes
		d.RiskLevels = consoleRiskLevels
		if o.Retention != nil {
			d.RetentionDays = o.Retention.RetentionDays("")
			d.PromptLogging = o.Retention.PromptLoggingEnabled("")
		}
		d.ResetAvailable = o.ResetAvailable
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := policyTmpl.Execute(w, d); err != nil {
			http.Error(w, "policy render error", http.StatusInternalServerError)
		}
	}
}

var policyTmpl = template.Must(template.New("policy").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
}).Parse(policyHTML))

const policyHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Policy</title>
<style>
{{adminCSS}}
*{box-sizing:border-box}
body{margin:0;background:#0b1220;color:#e8eef7;font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.hd{padding:20px 26px;border-bottom:1px solid #22304d}
.hd h1{font-size:1.3rem;margin:0}.hd .sub{color:#8fa1bf;font-size:.85rem;margin-top:4px}
.wrap{padding:22px 30px;display:flex;flex-direction:column;gap:18px;max-width:900px}
.card{background:#0e1626;border:1px solid #22304d;border-radius:12px;padding:18px 20px;overflow-x:auto}
.card h2{margin:0 0 10px;font-size:1rem}
.kv{display:flex;gap:26px;flex-wrap:wrap;font-size:.9rem}
.kv b{color:#8fa1bf;font-weight:500}
.note{color:#8fa1bf;font-size:.85rem;line-height:1.6}
textarea{width:100%;background:#0b1220;border:1px solid #2c4066;border-radius:8px;color:#e8eef7;padding:11px 13px;font-size:.9rem;font-family:inherit;min-height:70px}
.btn{background:#22c58b;color:#0b1220;border:0;border-radius:8px;padding:10px 18px;font-weight:700;font-size:14px;cursor:pointer;margin-top:10px}
.res{margin-top:14px;display:none}
.badge{display:inline-block;padding:3px 10px;border-radius:20px;font-size:.78rem;font-weight:600;margin-right:6px}
.b-local{background:#13351f;color:#4fd08a}.b-cloud{background:#33290f;color:#f4b740}.b-block{background:#361525;color:#f07ab0}
.b-neutral{background:#12203a;color:#b7c4dc}
.res .row{margin:6px 0;font-size:.9rem}.res .k{color:#8fa1bf;display:inline-block;min-width:130px}
.mono{font-family:ui-monospace,monospace}
.chip{display:inline-block;padding:2px 8px;border-radius:6px;background:#12203a;color:#b7c4dc;font-size:.78rem;margin:2px}
.eg{color:#5f6e87;font-size:.82rem;margin-top:6px}
.eg a{color:#7fd3ff;cursor:pointer}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "policy"}}
<div class="tk-main">
<div class="hd"><h1>Policy</h1>
<div class="sub">The firewall for data egress. See the active policy and test how a prompt routes — before a single provider call.</div></div>
<div class="wrap">

{{if .Notice}}<div class="card" style="border-color:#1f6b45;color:#a7f3d0;background:#0f2f22">{{.Notice}}</div>{{end}}
{{if .Error}}<div class="card" style="border-color:#5b2330;color:#f8b4bc;background:#2a1118">{{.Error}}</div>{{end}}

<div class="card">
  <h2>Active policy</h2>
  {{if .HasPolicy}}
  <div class="kv">
    <span><b>version</b> <span class="mono">{{.PolicyVersion}}</span></span>
    <span><b>rules</b> {{.RuleCount}}</span>
    <span><b>registry</b> <span class="mono">{{.RegistryVer}}</span></span>
  </div>
  {{else}}<div class="note">No active policy (built-in default). Set <span class="mono">ROUTER_POLICY_PATH=builtin:nis2-baseline</span> for the NIS2 starter ruleset.</div>{{end}}
  <div class="note" style="margin-top:10px">Evaluation per rule: <b>block → force → constraints → hints → defaults</b>. Constraints accumulate and are <b>fail-closed</b> — with no compliant provider the request is blocked (never a silent cloud fallback). Rules change without a code deploy.</div>
  <div class="note" style="margin-top:8px">Evidence: <a href="/router/compliance/report" style="color:#7fd3ff">control report</a> · <a href="/router/gap-report" style="color:#7fd3ff">shadow-AI gap report</a> · <a href="/router/incident-report?window=24h" style="color:#7fd3ff">incident evidence 24h</a> · <a href="/router/incident-report" style="color:#7fd3ff">72h</a> · <a href="/router/audit/export" style="color:#7fd3ff">audit chain</a></div>
</div>

<div class="card">
  <h2>Rules — no YAML {{if .ConsoleActive}}<span class="badge b-local">LIVE</span>{{else if .ConsoleRules}}<span class="badge b-neutral">draft</span>{{end}}</h2>
  {{if not .RulesEnabled}}
  <div class="note">Rule editing needs a data dir (<span class="mono">ROUTER_DATA_DIR</span>).</div>
  {{else}}
  <div class="note" style="margin-bottom:10px">Build the egress firewall like firewall rules: <b>condition → action</b>, evaluated top-down, fail-closed. Activate applies instantly (no restart) and is recorded in the audit chain; rollback returns to the baseline policy (<span class="mono">ROUTER_POLICY_PATH</span>). Test with the dry-run below before and after.</div>
  {{if .ConsoleRules}}
  <table style="width:100%;border-collapse:collapse;margin-bottom:12px">
    <tr style="color:#8fa1bf;font-size:.78rem;text-transform:uppercase;letter-spacing:.05em"><td style="padding:4px 0">#</td><td>When</td><td>Then</td><td></td></tr>
    {{range $i, $r := .ConsoleRules}}
    <tr style="border-top:1px solid #16223b">
      <td style="padding:8px 8px 8px 0;color:#8fa1bf">{{$i}}</td>
      <td style="padding:8px 8px 8px 0"><span class="chip">{{$r.WhenSummary}}</span>{{if $r.Note}}<div class="note">{{$r.Note}}</div>{{end}}</td>
      <td style="padding:8px 8px 8px 0;font-size:.86rem">{{$r.ActionSummary}}</td>
      <td style="padding:8px 0;text-align:right">
        <form method="post" action="/router/policy/rules/delete" style="margin:0">
          <input type="hidden" name="index" value="{{$i}}">
          <button class="btn" style="background:transparent;color:#f07ab0;border:1px solid #4a1c33;padding:4px 10px;font-size:12px;margin:0" type="submit">Remove</button>
        </form>
      </td>
    </tr>
    {{end}}
  </table>
  <div style="display:flex;gap:10px">
    {{if not .ConsoleActive}}
    <form method="post" action="/router/policy/activate" style="margin:0"><button class="btn" style="margin:0" type="submit">Activate ruleset</button></form>
    {{end}}
    {{if .ConsoleActive}}
    <form method="post" action="/router/policy/rollback" style="margin:0" onsubmit="return confirm('Roll back to the baseline policy? The console rules stay saved but stop applying.')"><button class="btn" style="margin:0;background:#33290f;color:#f4d38a;border:1px solid #7a5a1f" type="submit">Roll back to baseline</button></form>
    {{end}}
  </div>
  {{end}}
  <form method="post" action="/router/policy/rules" style="margin-top:14px;border-top:1px solid #16223b;padding-top:12px">
    <div style="font-size:.82rem;color:#8fa1bf;margin-bottom:6px">WHEN (at least one condition)</div>
    <div style="display:flex;flex-wrap:wrap;gap:8px;margin-bottom:10px">
      {{range .Sensitivities}}<label style="display:inline-flex;gap:5px;align-items:center;font-size:.85rem;background:#0b1220;border:1px solid #22304d;border-radius:8px;padding:6px 10px;cursor:pointer"><input type="checkbox" name="sensitivity" value="{{.}}">{{.}}</label>{{end}}
    </div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;margin-bottom:10px">
      <label style="font-size:.85rem;color:#8fa1bf">task type
        <select name="task_type" style="display:block;background:#0b1220;border:1px solid #22304d;color:#e8eef7;border-radius:8px;padding:7px 9px;margin-top:4px">
          <option value="">any</option>
          {{range .TaskTypes}}<option value="{{.}}">{{.}}</option>{{end}}
        </select></label>
      <label style="font-size:.85rem;color:#8fa1bf">risk level
        <select name="risk_level" style="display:block;background:#0b1220;border:1px solid #22304d;color:#e8eef7;border-radius:8px;padding:7px 9px;margin-top:4px">
          <option value="">any</option>
          {{range .RiskLevels}}<option value="{{.}}">{{.}}</option>{{end}}
        </select></label>
    </div>
    <div style="font-size:.82rem;color:#8fa1bf;margin-bottom:6px">THEN</div>
    <div style="display:flex;gap:10px;flex-wrap:wrap;align-items:end;margin-bottom:10px">
      <label style="font-size:.85rem;color:#8fa1bf">action
        <select name="action" id="ruleaction" style="display:block;background:#0b1220;border:1px solid #22304d;color:#e8eef7;border-radius:8px;padding:7px 9px;margin-top:4px">
          <option value="require_tags">require provider tags (stay in the house / EU)</option>
          <option value="block">block (fail-closed)</option>
        </select></label>
      <label style="font-size:.85rem;color:#8fa1bf" id="tagslabel">provider tags (comma-separated)
        <input name="tags" value="local" style="display:block;background:#0b1220;border:1px solid #22304d;color:#e8eef7;border-radius:8px;padding:7px 9px;margin-top:4px;min-width:220px"></label>
      <label style="font-size:.85rem;color:#8fa1bf">note (optional)
        <input name="note" placeholder="why this rule exists" style="display:block;background:#0b1220;border:1px solid #22304d;color:#e8eef7;border-radius:8px;padding:7px 9px;margin-top:4px;min-width:220px"></label>
    </div>
    <button class="btn" style="margin:0" type="submit">Add rule</button>
  </form>
  {{end}}
</div>

<div class="card">
  <h2>Compliance-paket (moduler)</h2>
  <div class="note" style="margin-bottom:10px">Embedded rulesets — each market/regime enables the pack it needs. Activate with <span class="mono">ROUTER_POLICY_PATH=builtin:&lt;name&gt;</span> (restart). Same engine; the pack is market-specific.</div>
  <table style="width:100%;border-collapse:collapse">
  {{range .Packs}}
    <tr style="border-top:1px solid #16223b">
      <td style="padding:8px 8px 8px 0;vertical-align:top;white-space:nowrap">
        <span class="mono">builtin:{{.Name}}</span>{{if .Active}} <span class="badge b-local">aktiv</span>{{end}}
      </td>
      <td style="padding:8px 0;color:#8fa1bf;font-size:.86rem">{{.Desc}} <span class="chip">{{.Rules}} regler</span></td>
    </tr>
  {{end}}
  </table>
</div>

<div class="card">
  <h2>Data &amp; retention</h2>
  <div class="kv">
    <span><b>retention</b> {{if .RetentionDays}}{{.RetentionDays}} days (automatic sweep){{else}}not set{{end}}</span>
    <span><b>prompt logging</b> {{if .PromptLogging}}on{{else}}off (default){{end}}</span>
  </div>
  <div class="note" style="margin-top:10px">Deletion by policy, not by button: the retention sweeper removes request
  history past the window automatically (GDPR storage limitation). Prompt text is never stored unless prompt logging
  is explicitly enabled.</div>
  {{if .ResetAvailable}}
  <div class="note" style="margin-top:10px"><b>Reset demo data</b> clears operational data only — request history,
  spend/savings, demo chat sessions and in-memory counters. It never touches the audit chain, API keys, users or the
  roster — and the reset itself is <b>recorded in the tamper-evident audit chain</b>: you can delete data, but never
  the fact that you deleted it.</div>
  <form method="post" action="/router/data/reset" onsubmit="return confirm('Reset demo data? Request history, spend and demo sessions will be cleared. The reset is recorded in the audit chain.')" style="margin:10px 0 0">
    <button class="btn" style="background:#3a1620;color:#f8b4bc;border:1px solid #5b2330" type="submit">Reset demo data</button>
  </form>
  <div class="note" style="margin-top:14px"><b>Tenant erasure (GDPR art. 17)</b>: delete ONE tenant's request history and
  spend attribution, optionally revoking its API keys. The audit chain is never selectively deleted — the erasure itself
  is recorded in it.</div>
  <form method="post" action="/router/data/erase-tenant" onsubmit="return confirm('Erase all request data for this tenant? The erasure is recorded in the audit chain.')" style="margin:8px 0 0;display:flex;gap:10px;flex-wrap:wrap;align-items:center">
    <input name="tenant_id" placeholder="tenant id (e.g. prospect_acme)" required style="background:#0b1220;border:1px solid #2c4066;border-radius:8px;color:#e8eef7;padding:9px 12px;font-size:.88rem;min-width:240px">
    <label style="display:inline-flex;gap:6px;align-items:center;font-size:.85rem;color:#8fa1bf"><input type="checkbox" name="revoke_keys" checked style="width:auto">also revoke the tenant's API keys</label>
    <button class="btn" style="margin:0;background:#3a1620;color:#f8b4bc;border:1px solid #5b2330" type="submit">Erase tenant data</button>
  </form>
  {{end}}
</div>

<div class="card">
  <h2>Dry-run — test a prompt</h2>
  <div class="note" style="margin-bottom:8px">Paste a prompt → see the classification, selected model, tier and egress (or a fail-closed block). No data leaves the router; no provider is called.</div>
  <textarea id="p" placeholder="e.g. Summarise the case for customer 811218-9876 ..."></textarea>
  <div class="eg">Examples:
    <a onclick="ex('Summarise the case for customer 811218-9876 who complained about an invoice.')">personal ID</a> ·
    <a onclick="ex('security review our SSO login flow for auth bypass and secret leakage')">security review</a> ·
    <a onclick="ex('write a concise git commit message for a bugfix')">trivial</a>
  </div>
  <button class="btn" onclick="run()">Run dry-run</button>
  <div class="res" id="res"></div>
</div>

</div></div></div>
<script>
const ra=document.getElementById('ruleaction');
if(ra){ra.onchange=function(){document.getElementById('tagslabel').style.display=ra.value==='block'?'none':'';};}
function ex(t){document.getElementById('p').value=t;run();}
async function run(){
  const p=document.getElementById('p').value.trim();if(!p)return;
  const res=document.getElementById('res');res.style.display='block';res.innerHTML='<span class="note">running…</span>';
  try{
    const r=await fetch('/router/policy/dryrun',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json'},body:JSON.stringify({prompt:p})});
    if(!r.ok){res.innerHTML='<span class="badge b-block">fel</span> '+(await r.text());return;}
    const j=await r.json();
    let head='';
    if(j.blocked){head='<span class="badge b-block">BLOCKED (fail-closed)</span>';}
    else if(j.egress==='local'){head='<span class="badge b-local">🔀 LOCAL — stays in the house</span>';}
    else{head='<span class="badge b-cloud">CLOUD</span>';}
    let h=head+'<div class="row" style="margin-top:12px"><span class="k">Classification</span> <span class="chip">task: '+(j.task||'?')+'</span><span class="chip">risk: '+(j.risk||'?')+'</span><span class="chip">sensitivity: '+(j.sensitivity||'none')+'</span>'+((j.pii_types&&j.pii_types.length)?'<span class="chip">pii: '+j.pii_types.join(',')+'</span>':'')+'</div>';
    if(j.blocked){
      h+='<div class="row"><span class="k">Block code</span> <span class="mono">'+(j.block_code||'')+'</span></div>';
      h+='<div class="row"><span class="k">Reason</span> '+(j.block_reason||'')+'</div>';
    }else{
      h+='<div class="row"><span class="k">Selected model</span> <span class="mono">'+(j.selected_model||'')+'</span> <span class="chip">'+(j.tier||'')+'</span></div>';
      h+='<div class="row"><span class="k">Provider / egress</span> '+(j.provider||'')+' — <b>'+(j.egress||'')+'</b>'+((j.model_tags&&j.model_tags.length)?' <span class="chip">'+j.model_tags.join(' ')+'</span>':'')+'</div>';
    }
    h+='<div class="row"><span class="k">Policy</span> <span class="mono">'+(j.policy_version||'')+'</span></div>';
    if(j.reasons&&j.reasons.length){h+='<div class="row"><span class="k">Reasons (routing)</span> <span class="note">'+j.reasons.join(' · ')+'</span></div>';}
    res.innerHTML=h;
  }catch(e){res.innerHTML='<span class="badge b-block">fel</span> '+e.message;}
}
</script>
</body></html>`
