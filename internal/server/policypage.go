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
	"github.com/magnusfroste/sluss/internal/secondopinion"
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
func PolicyDryRunHandler(eng *engine.Engine, cache *policy.Cache, so *secondopinion.Classifier) http.HandlerFunc {
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
		dec, job, err := explainRoute(eng, cache, so, explainInput{
			RequestID: "policy-dry-run",
			Request:   &openai.ChatRequest{Messages: []openai.Message{{Role: "user", Content: body.Prompt}}},
		})
		out := map[string]any{
			"task":            string(job.TaskType),
			"task_confidence": job.TaskConfidence,
			"risk":            string(job.RiskLevel),
			"sensitivity":     string(job.Sensitivity),
			"sensitivities":   job.AllSensitivities(),
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
	// ActiveRules is the active policy in plain words (ISSUE-116).
	ActiveRules []policy.RuleSummary
	// Rows is ActiveRules ready for the page (ISSUE-150): one list, in
	// plain words, with a Remove button on the rules that were added here.
	Rows []policyRow
	// Drafts are rules added here that do not apply yet.
	Drafts []ConsoleRule
	// Classes are the kinds of data a rule can name, code and label.
	Classes []labelled
	// ClassJSON is code to label, for the dry-run's script.
	ClassJSON template.JS
	Packs     []policyPack
	// Console rules (no-YAML rule editing, ISSUE-093).
	RulesEnabled  bool
	ConsoleRules  []ConsoleRule
	ConsoleActive bool
	Sensitivities []string
	ToolRisks     []string // agent capability classes (ISSUE-111)
	TaskTypes     []string
	RiskLevels    []string
	// Data & retention card (ISSUE-096).
	RetentionDays  int
	PromptLogging  bool
	ResetAvailable bool
	Notice         string
	Error          string
}

// policyRow is one line of "Your rules".
type policyRow struct {
	When, Then, Kind string
	Title            string // the rule's id and its exact outcome, for a tooltip
	Note             string
	Removable        bool
	Index            int
}

type labelled struct{ Code, Label string }

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
				d.ActiveRules = p.Summaries()
			}
		}
		d.Packs = builtinPacks(d.PolicyVersion)
		d.RulesEnabled = o.History != nil
		d.ConsoleRules = LoadConsoleRules(o.History)
		d.ConsoleActive = ConsolePolicyActive(o.History)
		// While the rules added here are the ones in force, the active
		// policy is those rules in order and then the default; each of them
		// can be removed from the same list. Otherwise they are a draft.
		for i, r := range d.ActiveRules {
			row := policyRow{When: plainRule(r.When), Then: plainThen(r.Kind, r.Then), Kind: r.Kind,
				Title: r.ID + ": " + r.Then, Note: r.Description, Index: i}
			if d.ConsoleActive && i < len(d.ConsoleRules) {
				row.Removable = true
				row.Note = d.ConsoleRules[i].Note
			}
			if row.When == "" || row.When == "every request" {
				row.When = "everything else"
			}
			d.Rows = append(d.Rows, row)
		}
		if !d.ConsoleActive {
			d.Drafts = d.ConsoleRules
		}
		for _, c := range consoleSensitivities {
			d.Classes = append(d.Classes, labelled{c, dataClassLabel(c)})
		}
		if b, err := json.Marshal(dataClassLabels); err == nil {
			d.ClassJSON = template.JS(b)
		}
		d.Sensitivities = consoleSensitivities
		d.ToolRisks = consoleToolRisks
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
	"adminCSS":  adminCSSFunc,
	"adminNav":  adminNavFunc,
	"dsHead":    dsHeadFunc,
	"plainRule": plainRule,
	"advJS":     advancedScriptFunc,
	"taskLabel": taskLabel,
	"inc":       func(i int) int { return i + 1 },
}).Parse(policyHTML))

const policyHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Policy</title>
<style>
{{adminCSS}}
*{box-sizing:border-box}
body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
.hd{padding:20px 26px;border-bottom:1px solid #22324f}
.hd h1{font-size:1.3rem;margin:0}.hd .sub{color:#8fa1bf;font-size:.85rem;margin-top:4px}
.wrap{padding:22px 30px;display:flex;flex-direction:column;gap:18px;max-width:900px}
.card{background:#101c34;border:1px solid #22324f;border-radius:12px;padding:18px 20px;overflow-x:auto}
.card.danger{border-color:#5b2330}
table.rules{width:100%;border-collapse:collapse;margin-top:12px;font-size:.88rem}
table.rules th{text-align:left;font-size:.7rem;color:#8fa1bf;text-transform:uppercase;letter-spacing:.06em;padding:6px 10px;border-bottom:1px solid #22324f}
table.rules td{padding:8px 10px;border-bottom:1px solid #16223a;vertical-align:top}
table.rules .rid{font-family:ui-monospace,Menlo,monospace;font-size:.72rem;color:#64748b;margin-top:2px}
table.rules .then{font-weight:600}
tr.k-block .then{color:#f87171}tr.k-require .then{color:#4ade80}tr.k-force .then{color:#f4b740}
.card h2{margin:0 0 10px;font-size:1rem}
.kv{display:flex;gap:26px;flex-wrap:wrap;font-size:.9rem}
.kv b{color:#8fa1bf;font-weight:500}
.note{color:#8fa1bf;font-size:.85rem;line-height:1.6}
textarea{width:100%;background:#081328;border:1px solid #2c4066;border-radius:8px;color:#e8eef7;padding:11px 13px;font-size:.9rem;font-family:inherit;min-height:70px}
.btn{background:#fad100;color:#081328;border:0;border-radius:8px;padding:10px 18px;font-weight:700;font-size:14px;cursor:pointer;margin-top:10px}
.res{margin-top:14px;display:none}
.badge{display:inline-block;padding:3px 10px;border-radius:20px;font-size:.78rem;font-weight:600;margin-right:6px}
.b-local{background:#13351f;color:#4fd08a}.b-cloud{background:#33290f;color:#f4b740}.b-block{background:#361525;color:#f07ab0}
.b-neutral{background:#182a4a;color:#b7c4dc}
.res .row{margin:6px 0;font-size:.9rem}.res .k{color:#8fa1bf;display:inline-block;min-width:130px}
.mono{font-family:ui-monospace,monospace}
.chip{display:inline-block;padding:2px 8px;border-radius:6px;background:#182a4a;color:#b7c4dc;font-size:.78rem;margin:2px}
.eg{color:#5f6e87;font-size:.82rem;margin-top:6px}
.eg a{color:#7fd3ff;cursor:pointer}
.cardhd{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;margin-bottom:6px}
.cardhd h2{margin:0}
tr.k-mask .then{color:#f4b740}tr.k-default .then,tr.k-other .then{color:#b7c4dc;font-weight:500}
.linkbtn{background:transparent;border:0;color:#f07ab0;font:inherit;font-size:.82rem;cursor:pointer;padding:8px 4px}
.linkbtn:hover{text-decoration:underline}
.btn.ghost{background:transparent;color:#e8eef7;border:1px solid #2c4066}
.btn.dangerbtn{background:#3a1620;color:#f8b4bc;border:1px solid #5b2330}
.draft{margin-top:16px;border:1px dashed #7a5a1f;border-radius:10px;padding:12px 14px;background:#1a1608}
.drafthd{margin-bottom:4px}
.lbl{font-size:.82rem;color:#8fa1bf;margin:10px 0 6px}
.checks{display:flex;flex-wrap:wrap;gap:8px}
.check{display:inline-flex;gap:6px;align-items:center;font-size:.88rem;background:#081328;border:1px solid #22324f;border-radius:8px;padding:8px 11px;cursor:pointer}
.check input{width:auto}
.fields{display:flex;gap:10px;flex-wrap:wrap;margin:12px 0}
.fields label{font-size:.82rem;color:#8fa1bf;display:flex;flex-direction:column;gap:4px}
.fields select,.fields input,.card input[name=tenant_id]{background:#081328;border:1px solid #22324f;color:#e8eef7;border-radius:8px;padding:8px 10px;font:inherit;font-size:.9rem;min-width:180px}
.note a{color:#7fd3ff}
details.why{margin-top:10px}details.why summary{cursor:pointer;color:#8fa1bf;font-size:.85rem}
@media(max-width:640px){table.rules th:nth-child(3){display:none}.fields select,.fields input{min-width:0;width:100%}.fields label{width:100%}}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "policy"}}
<div class="tk-main">
{{dsHead "policy" "Policy" "What may go where. Add a rule, and test any sentence against the rules without sending it."}}
<div class="wrap">

{{if .Notice}}<div class="card" style="border-color:#1f6b45;color:#a7f3d0;background:#0f2f22">{{.Notice}}</div>{{end}}
{{if .Error}}<div class="card" style="border-color:#5b2330;color:#f8b4bc;background:#2a1118">{{.Error}}</div>{{end}}

<div class="card">
  <div class="cardhd">
    <h2>Your rules</h2>
    {{if .RulesEnabled}}<button class="btn" style="margin:0" type="button" onclick="document.getElementById('addrule').showModal()">Add a rule</button>{{end}}
  </div>
  {{if .Rows}}
  <table class="rules">
    <thead><tr><th>When</th><th>Then</th><th></th></tr></thead>
    <tbody>
    {{range .Rows}}
    <tr class="k-{{.Kind}}" title="{{.Title}}">
      <td>{{.When}}{{if .Note}}<div class="rid">{{.Note}}</div>{{end}}</td>
      <td class="then">{{.Then}}</td>
      <td style="text-align:right;white-space:nowrap">{{if .Removable}}<form method="post" action="/router/policy/rules/delete" style="margin:0" onsubmit="return confirm('Remove this rule? It stops applying at once.')"><input type="hidden" name="index" value="{{.Index}}"><button class="linkbtn" type="submit">Remove</button></form>{{end}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
  <div class="note" style="margin-top:10px">Every question is checked against these rules. When several match, the strictest wins. If no allowed model is left, the question is refused and recorded, never sent somewhere else.</div>
  {{else}}<div class="note">No rules yet. Every question may go to any model. Add a rule to keep a kind of data on your own hardware.</div>{{end}}

  {{if .Drafts}}
  <div class="draft">
    <div class="drafthd"><b>Not applied yet</b> <span class="note">{{len .Drafts}} {{if eq (len .Drafts) 1}}rule{{else}}rules{{end}} added here. Applying them replaces the rules above.</span></div>
    <table class="rules">
      <tbody>
      {{range $i, $r := .Drafts}}
      <tr class="k-{{$r.Kind}}">
        <td>{{$r.PlainWhen}}{{if $r.Note}}<div class="rid">{{$r.Note}}</div>{{end}}</td>
        <td class="then">{{$r.PlainThen}}</td>
        <td style="text-align:right;white-space:nowrap"><form method="post" action="/router/policy/rules/delete" style="margin:0"><input type="hidden" name="index" value="{{$i}}"><button class="linkbtn" type="submit">Remove</button></form></td>
      </tr>
      {{end}}
      </tbody>
    </table>
    <form method="post" action="/router/policy/activate" style="margin:12px 0 0"><button class="btn" style="margin:0" type="submit">Apply these rules</button></form>
  </div>
  {{end}}
</div>

<div class="card">
  <h2>Test a sentence</h2>
  <div class="note" style="margin-bottom:8px">Type or paste a question and see what the rules do with it. Nothing is sent to any model.</div>
  <textarea id="p" placeholder="e.g. Summarise the case for customer 811218-2392"></textarea>
  <div class="eg">Try:
    <a onclick="ex('Summarise the case for customer 811218-2392 who complained about an invoice.')">a personal identity number</a> ·
    <a onclick="ex('Write a letter to our customer Karin Ek about the late delivery.')">a name</a> ·
    <a onclick="ex('Write a short commit message for a bug fix.')">nothing sensitive</a>
  </div>
  <button class="btn" onclick="run()">Test</button>
  <div class="res" id="res"></div>
</div>

<details class="ds-adv" data-adv="policy">
  <summary>Advanced <small>more conditions, rule packs, retention, erasure</small></summary>
  <div class="ds-adv-body">

  <div class="card">
    <h2>What is in force</h2>
    {{if .HasPolicy}}
    <div class="kv">
      <span><b>policy</b> <span class="mono">{{.PolicyVersion}}</span></span>
      <span><b>rules</b> {{.RuleCount}}</span>
      <span><b>model list</b> <span class="mono">{{.RegistryVer}}</span></span>
      <span><b>source</b> {{if .ConsoleActive}}rules added here{{else}}the policy the server started with{{end}}</span>
    </div>
    {{else}}<div class="note">No policy is loaded; the built-in default applies.</div>{{end}}
    {{if .ConsoleActive}}
    <div class="note" style="margin-top:10px">Going back makes the policy the server started with apply again. The rules added here stay saved but stop applying.</div>
    <form method="post" action="/router/policy/rollback" style="margin:10px 0 0" onsubmit="return confirm('Go back to the policy the server started with? The rules added here stay saved but stop applying.')"><button class="btn ghost" style="margin:0" type="submit">Go back to the starting policy</button></form>
    {{end}}
    <div class="note" style="margin-top:10px">For an auditor: <a href="/router/compliance/report">control report</a> · <a href="/router/gap-report">gap report</a> · <a href="/router/incident-report?window=24h">incident, 24 h</a> · <a href="/router/incident-report">72 h</a> · <a href="/router/audit">audit log</a></div>
  </div>

  {{if .RulesEnabled}}
  <div class="card" id="morerule">
    <h2>A rule with more conditions</h2>
    <div class="note" style="margin-bottom:10px">Besides the kind of data, a rule can look at the kind of task, the risk, and what an agent's tools can do; and it can require any tag you have given your models, not only <span class="mono">local</span>. All the conditions you choose must hold.</div>
    <form method="post" action="/router/policy/rules">
      <div class="lbl">Kind of data</div>
      <div class="checks">
        {{range .Classes}}<label class="check"><input type="checkbox" name="sensitivity" value="{{.Code}}">{{.Label}}</label>{{end}}
      </div>
      <div class="lbl">What the agent's tools can do</div>
      <div class="checks">
        {{range .ToolRisks}}<label class="check"><input type="checkbox" name="tool_risk" value="{{.}}">{{.}}</label>{{end}}
      </div>
      <div class="fields">
        <label>Kind of task
          <select name="task_type"><option value="">any</option>{{range .TaskTypes}}<option value="{{.}}">{{taskLabel .}}</option>{{end}}</select></label>
        <label>Risk
          <select name="risk_level"><option value="">any</option>{{range .RiskLevels}}<option value="{{.}}">{{.}}</option>{{end}}</select></label>
        <label>Then
          <select name="action" id="ruleaction">
            <option value="require_tags">only models with these tags</option>
            <option value="mask">mask and send</option>
            <option value="block">block</option>
          </select></label>
        <label id="tagslabel">Tags, comma-separated
          <input name="tags" value="local"></label>
        <label>Note, optional
          <input name="note" placeholder="why this rule exists"></label>
      </div>
      <button class="btn" style="margin:0" type="submit">Add rule</button>
    </form>
  </div>
  {{end}}

  <div class="card">
    <h2>Rule packs</h2>
    <div class="note" style="margin-bottom:10px">Ready-made sets of rules for a regulation. Whoever runs the server chooses one at start with <span class="mono">ROUTER_POLICY_PATH=builtin:&lt;name&gt;</span>.</div>
    <table style="width:100%;border-collapse:collapse">
    {{range .Packs}}
      <tr style="border-top:1px solid #17253f">
        <td style="padding:8px 8px 8px 0;vertical-align:top;white-space:nowrap">
          <span class="mono">{{.Name}}</span>{{if .Active}} <span class="badge b-local">in use</span>{{end}}
        </td>
        <td style="padding:8px 0;color:#8fa1bf;font-size:.86rem">{{.Desc}} <span class="chip">{{.Rules}} rules</span></td>
      </tr>
    {{end}}
    </table>
  </div>

  <div class="card">
    <h2>What is kept, and for how long</h2>
    <div class="kv">
      <span><b>request history</b> {{if .RetentionDays}}{{.RetentionDays}} days, then removed automatically{{else}}no limit set{{end}}</span>
      <span><b>the text of questions</b> {{if .PromptLogging}}stored{{else}}never stored{{end}}</span>
    </div>
    <div class="note" style="margin-top:10px">The request history holds counts and classes, never the text of a question unless that is turned on. The audit log is separate and is never removed.</div>
  </div>

  {{if .ResetAvailable}}
  <div class="card danger">
    <h2>Erase</h2>
    <div class="note"><b>One tenant's data</b> (GDPR article 17): removes that tenant's request history and spend, and can revoke its keys. The erasure itself is written to the audit log.</div>
    <form method="post" action="/router/data/erase-tenant" onsubmit="return confirm('Erase all request data for this tenant? The erasure is recorded in the audit log.')" style="margin:8px 0 0;display:flex;gap:10px;flex-wrap:wrap;align-items:center">
      <input name="tenant_id" placeholder="tenant id" required style="min-width:220px">
      <label style="display:inline-flex;gap:6px;align-items:center;font-size:.85rem;color:#8fa1bf"><input type="checkbox" name="revoke_keys" checked style="width:auto">also revoke its keys</label>
      <button class="btn dangerbtn" style="margin:0" type="submit">Erase tenant data</button>
    </form>
    <div class="note" style="margin-top:16px"><b>Demo data</b>: clears request history, spend, chat sessions and counters. It does not touch the audit log, keys, users or models, and the reset itself is written to the audit log.</div>
    <form method="post" action="/router/data/reset" onsubmit="return confirm('Reset demo data? Request history, spend and chat sessions are cleared. The reset is recorded in the audit log.')" style="margin:10px 0 0">
      <button class="btn dangerbtn" style="margin:0" type="submit">Reset demo data</button>
    </form>
  </div>
  {{end}}

  </div>
</details>

</div></div></div>

{{if .RulesEnabled}}
<dialog class="ds-dialog" id="addrule">
  <form method="post" action="/router/policy/rules">
    <h2>Add a rule</h2>
    <div class="note" style="margin-bottom:14px">When a question holds this kind of data, do this.</div>
    <div class="lbl">When a question holds</div>
    <div class="checks">
      {{range .Classes}}<label class="check"><input type="checkbox" name="sensitivity" value="{{.Code}}">{{.Label}}</label>{{end}}
    </div>
    <div class="lbl" style="margin-top:14px">Then</div>
    <div style="display:flex;flex-direction:column;gap:8px">
      <label class="ds-choice"><input type="radio" name="action" value="require_tags" checked><span><b>Keep on own hardware</b><small>Only models you have tagged local may answer.</small></span></label>
      <label class="ds-choice"><input type="radio" name="action" value="mask"><span><b>Mask and send</b><small>Identifiers, names and keys are replaced before the question goes to a cloud model, and put back in the answer.</small></span></label>
      <label class="ds-choice"><input type="radio" name="action" value="block"><span><b>Block</b><small>The question is refused and recorded.</small></span></label>
    </div>
    <input type="hidden" name="tags" value="local">
    <div class="fields" style="margin-top:14px"><label style="flex:1">Note, optional<input name="note" placeholder="why this rule exists"></label></div>
    <div id="addruleerr" class="note" style="color:#f8b4bc;display:none;margin-top:8px">Choose at least one kind of data.</div>
    <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:18px">
      <button class="btn ghost" style="margin:0" type="button" onclick="document.getElementById('addrule').close()">Cancel</button>
      <button class="btn" style="margin:0" type="submit">Add rule</button>
    </div>
    <div class="note" style="margin-top:12px">Need the kind of task, the risk or an agent's tools as a condition? <a href="#morerule" onclick="document.getElementById('addrule').close();var d=document.querySelector('details[data-adv=policy]');d.open=true;">More conditions</a></div>
  </form>
</dialog>
{{end}}

{{advJS}}
<script>
var dlg=document.getElementById('addrule');
if(dlg){dlg.querySelector('form').addEventListener('submit',function(e){
  if(!dlg.querySelector('input[name=sensitivity]:checked')){e.preventDefault();document.getElementById('addruleerr').style.display='block';}
});}
var ra=document.getElementById('ruleaction');
if(ra){ra.onchange=function(){document.getElementById('tagslabel').style.display=(ra.value==='block'||ra.value==='mask')?'none':'';};}
var CLASS={{.ClassJSON}};
var PII={personnummer:'personal identity number',email:'e-mail address',phone:'phone number',iban:'IBAN',card:'card number',address:'postal address',name:'name',national_id:'identity number',birthdate:'date of birth',passport:'passport number',vehicle:'vehicle registration'};
function esc(s){return String(s==null?'':s).replace(/[&<>"]/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c];});}
function ex(t){document.getElementById('p').value=t;run();}
async function run(){
  var p=document.getElementById('p').value.trim();if(!p)return;
  var res=document.getElementById('res');res.style.display='block';res.innerHTML='<span class="note">testing…</span>';
  try{
    var r=await fetch('/router/policy/dryrun',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json'},body:JSON.stringify({prompt:p})});
    if(!r.ok){res.innerHTML='<span class="badge b-block">error</span> '+esc(await r.text());return;}
    var j=await r.json();
    var masked=(j.reasons||[]).some(function(x){return x.indexOf('mask and send: ')===0&&x.indexOf('nothing is masked')<0;});
    var head;
    if(j.blocked){head='<span class="badge b-block">Blocked</span> The question is refused and recorded.';}
    else if(j.egress==='local'){head='<span class="badge b-local">Stays on own hardware</span>';}
    else if(masked){head='<span class="badge b-cloud">Masked, then sent to the cloud</span>';}
    else{head='<span class="badge b-cloud">May go to the cloud</span>';}
    var found=(j.sensitivities||[]).map(function(c){return CLASS[c]||c;});
    var kinds=(j.pii_types||[]).map(function(t){return PII[t]||t;});
    var h='<div class="row">'+head+'</div>';
    h+='<div class="row"><span class="k">Found</span> '+(found.length?esc(found.join(', '))+(kinds.length?' <span class="note">('+esc(kinds.join(', '))+')</span>':''):'Nothing sensitive')+'</div>';
    if(!j.blocked){h+='<div class="row"><span class="k">Answered by</span> <span class="mono">'+esc(j.selected_model||'')+'</span> <span class="note">'+(j.egress==='local'?'on own hardware':'in the cloud')+'</span></div>';}
    var d='<div class="row"><span class="k">Task</span> '+esc(j.task||'')+' · risk '+esc(j.risk||'')+'</div>';
    if(j.blocked){d+='<div class="row"><span class="k">Block code</span> <span class="mono">'+esc(j.block_code||'')+'</span> '+esc(j.block_reason||'')+'</div>';}
    else{d+='<div class="row"><span class="k">Provider</span> '+esc(j.provider||'')+' · '+esc(j.tier||'')+((j.model_tags&&j.model_tags.length)?' · tags '+esc(j.model_tags.join(', ')):'')+'</div>';}
    d+='<div class="row"><span class="k">Policy</span> <span class="mono">'+esc(j.policy_version||'')+'</span></div>';
    if(j.reasons&&j.reasons.length){d+='<div class="row"><span class="k">Reasons</span> <span class="note">'+j.reasons.map(esc).join('<br>')+'</span></div>';}
    h+='<details class="why"><summary>Details</summary>'+d+'</details>';
    res.innerHTML=h;
  }catch(e){res.innerHTML='<span class="badge b-block">error</span> '+esc(e.message);}
}
</script>
</body></html>`
