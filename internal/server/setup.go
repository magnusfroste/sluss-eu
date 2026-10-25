package server

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/providercfg"
	"github.com/magnusfroste/sluss/internal/registry"
)

// Quick start (setup) — the "pick two models and go" path for a fresh
// instance. Four steps, each showing its LIVE status computed from the running
// gateway (never a stored checkbox): a routable cloud model, a routable local
// model, a policy that keeps personal data local, and a first dry-run. The
// actions reuse the roster and console-rule machinery, so everything a step
// does is the same audited change an admin could make by hand.

// SetupOptions wires the setup page to the live gateway.
type SetupOptions struct {
	Engine   *engine.Engine
	Cache    *policy.Cache
	Roster   providercfg.RosterStore
	Reloader *RosterReloader
	History  *history.Store
	Auditor  audit.Sink
	Logger   *slog.Logger
	Rules    PolicyRulesOptions
}

// SetupStatus is the live state of the four steps.
type SetupStatus struct {
	CloudModels []string
	LocalModels []string
	// Protects is true when the active policy keeps a personal-data prompt
	// local (requires a local-class tag) or blocks it.
	Protects      bool
	PolicyVersion string
	PIIOutcome    string // human summary of what the active policy does with PII
	Requests      int64
	ConsoleActive bool
	CanEdit       bool
}

// Done counts finished steps (of 4).
func (s SetupStatus) Done() int {
	n := 0
	for _, ok := range []bool{len(s.CloudModels) > 0, len(s.LocalModels) > 0, s.Protects, s.Requests > 0} {
		if ok {
			n++
		}
	}
	return n
}

// Complete reports whether every step is done.
func (s SetupStatus) Complete() bool { return s.Done() == 4 }

func (o SetupOptions) status() SetupStatus {
	st := SetupStatus{CanEdit: o.Roster != nil}
	if o.Engine != nil && o.Engine.Registry != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil {
			classify := ChatOptions{Engine: o.Engine}
			for _, m := range snap.EnabledModelsWithCapabilities(registry.Capabilities{Chat: true}) {
				if classify.egressForModel(m.ID) == "local" {
					st.LocalModels = append(st.LocalModels, m.ID)
				} else {
					st.CloudModels = append(st.CloudModels, m.ID)
				}
			}
		}
	}
	st.Protects, st.PolicyVersion, st.PIIOutcome = policyProtectsPII(o.Cache)
	if o.History != nil {
		st.Requests = o.History.TotalRequests() + o.History.BlockedRequests()
		st.ConsoleActive = ConsolePolicyActive(o.History)
	}
	return st
}

// policyProtectsPII evaluates the ACTIVE policy against a personal-data
// prompt's classification and reports whether it is kept local or blocked.
func policyProtectsPII(c *policy.Cache) (bool, string, string) {
	if c == nil {
		return false, "", "no policy loaded"
	}
	p, ok := c.Active(policy.Scope{})
	if !ok {
		return false, "", "no policy loaded"
	}
	ev := p.Evaluate(policy.EvaluationInput{TaskType: "summarization", RiskLevel: "medium", Sensitivity: "pii"})
	if ev.Blocked {
		return true, p.Version(), "personal data is blocked (fail-closed)"
	}
	if cons := ev.Route.Constraints; cons != nil {
		for _, t := range cons.RequireProviderTags {
			if localTags[t] {
				return true, p.Version(), "personal data may only go to models tagged " + strings.Join(cons.RequireProviderTags, " + ")
			}
		}
		if len(cons.RequireProviderTags) > 0 {
			return false, p.Version(), "personal data requires " + strings.Join(cons.RequireProviderTags, " + ") + " — not a local tag"
		}
	}
	return false, p.Version(), "personal data is routed like any other prompt — it can go to the cloud"
}

// SetupPageHandler renders the quick start.
func SetupPageHandler(o SetupOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d := struct {
			S      SetupStatus
			Notice string
			Error  string
		}{o.status(), r.URL.Query().Get("ok"), r.URL.Query().Get("err")}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := setupTmpl.Execute(w, d); err != nil && o.Logger != nil {
			o.Logger.Error("setup page render failed", "err", err)
		}
	}
}

var idUnsafe = regexp.MustCompile(`[^a-z0-9-]+`)

// slugID turns a display name into a roster id ("PrivAI RTX" → "privai-rtx").
func slugID(s string) string {
	s = idUnsafe.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	s = strings.Trim(s, "-")
	if len(s) > 32 {
		s = strings.Trim(s[:32], "-")
	}
	return s
}

var envName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

// SetupLocalHandler adds a local provider AND its model in one step: the
// provider is tagged local + on-prem, the model is balanced and priced above
// a cheap cloud model so it only wins when policy requires local (otherwise it
// would take all traffic and the cloud/local contrast disappears).
func SetupLocalHandler(o SetupOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if o.Roster == nil {
			redirectSetup(w, r, "", "the roster is read-only (no data dir configured)")
			return
		}
		_ = r.ParseForm()
		name := strings.TrimSpace(r.FormValue("name"))
		baseURL := strings.TrimSpace(r.FormValue("base_url"))
		keyEnv := strings.ToUpper(strings.TrimSpace(r.FormValue("key_env")))
		slug := strings.TrimSpace(r.FormValue("model"))
		id := slugID(name)
		switch {
		case id == "":
			redirectSetup(w, r, "", "give the server a name")
			return
		case !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://"):
			redirectSetup(w, r, "", "the base URL must start with http:// or https://")
			return
		case keyEnv != "" && !envName.MatchString(keyEnv):
			redirectSetup(w, r, "", "the key must be an environment variable NAME like LOCAL_LLM_KEY — never the key itself")
			return
		case slug == "":
			redirectSetup(w, r, "", "give the model name exactly as the server's /v1/models lists it")
			return
		}
		prov := providercfg.Provider{ID: id, Name: name, BaseURL: baseURL, KeyEnv: keyEnv, ComplianceTags: []string{"local", "on-prem"}}
		if err := o.Roster.UpsertRosterProvider(prov); err != nil {
			redirectSetup(w, r, "", "save provider: "+err.Error())
			return
		}
		model := providercfg.Model{ID: id + "-local", ProviderID: id, ProviderModelID: slug, Tier: "balanced",
			InputUSDPerMTok: 1.0, OutputUSDPerMTok: 4.0, Enabled: true}
		if err := o.Roster.UpsertRosterModel(model); err != nil {
			redirectSetup(w, r, "", "save model: "+err.Error())
			return
		}
		audit.Record(r.Context(), o.Auditor, audit.Entry{
			Action: audit.ActionProviderTags, Actor: AdminUserFromContext(r.Context()), Target: id,
			Reason: "quick start: local provider added with compliance tags [local, on-prem]",
		})
		okMsg, errMsg := applyRosterEdit(r.Context(), o.Reloader, AdminUserFromContext(r.Context()), "Local model "+model.ID+" added")
		redirectSetup(w, r, okMsg, errMsg)
	}
}

// SetupProtectHandler adds "personal data and possible secrets stay local"
// as console rules and activates them — ONLY when the active policy does not
// already protect personal data, so it never replaces a compliance pack.
func SetupProtectHandler(o SetupOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ok, _, _ := policyProtectsPII(o.Cache); ok {
			redirectSetup(w, r, "Your active policy already keeps personal data local — nothing changed.", "")
			return
		}
		if err := o.Rules.ready(); err != nil {
			redirectSetup(w, r, "", err.Error())
			return
		}
		rules := LoadConsoleRules(o.History)
		rules = append(rules, ConsoleRule{
			Sensitivities: []string{"pii", "secrets_possible"},
			Action:        "require_tags",
			Tags:          []string{"local"},
			Note:          "quick start: personal data and possible secrets stay in the house",
		})
		if err := saveConsoleRules(o.History, rules); err != nil {
			redirectSetup(w, r, "", "save rule: "+err.Error())
			return
		}
		v, err := o.Rules.activateConsoleRules(r, rules)
		if err != nil {
			redirectSetup(w, r, "", "activate: "+err.Error())
			return
		}
		redirectSetup(w, r, fmt.Sprintf("Personal data now stays local — policy %s is live (audited).", v), "")
	}
}

func redirectSetup(w http.ResponseWriter, r *http.Request, ok, errMsg string) {
	u := "/router/setup"
	if ok != "" {
		u += "?ok=" + urlQueryEscape(ok)
	} else if errMsg != "" {
		u += "?err=" + urlQueryEscape(errMsg)
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}

var setupTmpl = template.Must(template.New("setup").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
	"join":     func(s []string) string { return strings.Join(s, ", ") },
}).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Quick start</title>
<style>
  {{adminCSS}}
  *,*::before,*::after{box-sizing:border-box}
  body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
  header{padding:20px 26px;border-bottom:1px solid #22324f}
  h1{font-size:1.3rem;margin:0}
  .sub{color:#8fa1bf;font-size:.88rem;margin-top:4px}
  .wrap{padding:22px 26px 40px;max-width:860px;display:flex;flex-direction:column;gap:14px}
  .progress{display:flex;align-items:center;gap:12px;color:#8fa1bf;font-size:.88rem}
  .bar{flex:1;height:8px;background:#17253f;border-radius:4px;overflow:hidden;max-width:320px}
  .bar span{display:block;height:100%;background:#fad100;border-radius:4px}
  .step{background:#101c34;border:1px solid #22324f;border-radius:12px;padding:16px 20px}
  .step.done{border-color:#1f5c38}
  .sh{display:flex;align-items:center;gap:12px}
  .num{width:28px;height:28px;border-radius:50%;display:flex;align-items:center;justify-content:center;font-weight:700;font-size:.85rem;background:#17253f;color:#8fa1bf;flex-shrink:0}
  .done .num{background:#1fb054;color:#081328}
  .st{font-weight:600}
  .state{margin-left:auto;font-size:.78rem;font-weight:700;padding:3px 10px;border-radius:999px;background:#33290f;color:#f4b740;white-space:nowrap}
  .done .state{background:#0f3a26;color:#4ade80}
  .body{margin:10px 0 0 40px;color:#b7c4dc;font-size:.9rem;line-height:1.55}
  .body b{color:#e8eef7}
  code,.mono{font-family:"IBM Plex Mono",ui-monospace,Menlo,monospace;font-size:.85em}
  code{background:#0b1528;border:1px solid #22324f;border-radius:5px;padding:1px 6px}
  form.f{display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-top:12px}
  form.f .full{grid-column:1/-1}
  label{display:block;font-size:.75rem;color:#8fa1bf;margin-bottom:4px}
  input{width:100%;background:#081328;border:1px solid #2c4066;border-radius:8px;color:#e8eef7;padding:9px 11px;font-size:.88rem;font-family:inherit}
  input:focus{outline:none;border-color:#fad100}
  .btn{display:inline-block;background:#fad100;color:#081328;border:0;border-radius:8px;padding:9px 16px;font-weight:700;font-size:.88rem;cursor:pointer;text-decoration:none}
  .btn.ghost{background:#17253f;color:#e8eef7;border:1px solid #2c4066;font-weight:600}
  .banner{padding:11px 16px;border-radius:10px;font-size:.88rem}
  .banner.ok{background:#0f3a26;color:#a7f3d0;border:1px solid #1f5c38}
  .banner.err{background:#3b1414;color:#fecaca;border:1px solid #5b2330}
  .tries{display:flex;flex-direction:column;gap:8px;margin-top:12px}
  .try{display:flex;gap:10px;align-items:center;flex-wrap:wrap;background:#0b1528;border:1px solid #22324f;border-radius:8px;padding:9px 12px}
  .try .p{flex:1;min-width:220px;font-size:.86rem}
  .res{font-size:.8rem;font-weight:700;padding:3px 10px;border-radius:999px;background:#17253f;color:#8fa1bf}
  .res.local{background:#0f3a26;color:#4ade80}.res.cloud{background:#12264a;color:#7fb2ff}.res.blocked{background:#3b1414;color:#f87171}
  @media(max-width:700px){form.f{grid-template-columns:1fr}.body{margin-left:0}}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "setup"}}
<div class="tk-main">
<header>
  <h1>Quick start</h1>
  <div class="sub">Two models and one rule: harmless prompts go to a cheap cloud model, personal data stays on your own hardware. Each step shows its live state. Step by step in plain language: <a href="/manual/quick-start" style="color:#7fd3ff">the manual</a>.</div>
</header>
<div class="wrap">
  {{if .Notice}}<div class="banner ok">{{.Notice}}</div>{{end}}
  {{if .Error}}<div class="banner err">{{.Error}}</div>{{end}}
  <div class="progress"><div class="bar"><span style="width:{{if eq .S.Done 0}}2{{else if eq .S.Done 1}}25{{else if eq .S.Done 2}}50{{else if eq .S.Done 3}}75{{else}}100{{end}}%"></span></div>{{.S.Done}} of 4 done{{if .S.Complete}} — you are set up. Open the <a href="/router/dashboard" style="color:#7fd3ff">dashboard</a>.{{end}}</div>

  <div class="step{{if .S.CloudModels}} done{{end}}">
    <div class="sh"><span class="num">1</span><span class="st">A cloud model for harmless prompts</span><span class="state">{{if .S.CloudModels}}ready{{else}}missing{{end}}</span></div>
    <div class="body">{{if .S.CloudModels}}Routable: <b>{{join .S.CloudModels}}</b>. Cheap, everyday tasks go here.{{else}}No routable cloud model. Set <code>OPENROUTER_API_KEY</code> in the environment and restart — the built-in cheap, balanced and premium models then become routable — or add any OpenAI-compatible provider on <a href="/router/providers" style="color:#7fd3ff">Providers</a>.{{end}}</div>
  </div>

  <div class="step{{if .S.LocalModels}} done{{end}}">
    <div class="sh"><span class="num">2</span><span class="st">A local model for sensitive prompts</span><span class="state">{{if .S.LocalModels}}ready{{else}}missing{{end}}</span></div>
    <div class="body">
      {{if .S.LocalModels}}Routable and tagged local: <b>{{join .S.LocalModels}}</b>. Add another below, or edit it on <a href="/router/models" style="color:#7fd3ff">Models</a>.{{else}}Point Sluss at your own OpenAI-compatible server — vLLM, Ollama, LM Studio, a DGX or an RTX box. It is tagged <code>local</code> and <code>on-prem</code>, and priced so it only wins when your policy requires local.{{end}}
      {{if .S.CanEdit}}
      <form class="f" method="post" action="/router/setup/local">
        <div><label>Name</label><input name="name" placeholder="GPU server" required></div>
        <div><label>Base URL (OpenAI-compatible)</label><input name="base_url" placeholder="https://llm.internal.example/v1" required></div>
        <div><label>Model (exactly as its /v1/models lists it)</label><input name="model" placeholder="qwen3-27b" required></div>
        <div><label>Key env var name (optional — the value stays in the environment)</label><input name="key_env" placeholder="LOCAL_LLM_KEY"></div>
        <div class="full"><button class="btn" type="submit">Add local model</button> <span style="font-size:.8rem;color:#8fa1bf;margin-left:8px">Then press <b>Test</b> on <a href="/router/models" style="color:#7fd3ff">Models</a> to check the connection. A new key env var needs a restart.</span></div>
      </form>{{else}}<div style="margin-top:8px">Set <code>ROUTER_DATA_DIR</code> to add models from the console.</div>{{end}}
    </div>
  </div>

  <div class="step{{if .S.Protects}} done{{end}}">
    <div class="sh"><span class="num">3</span><span class="st">Personal data stays in the house</span><span class="state">{{if .S.Protects}}protected{{else}}not protected{{end}}</span></div>
    <div class="body">
      Active policy {{if .S.PolicyVersion}}<code>{{.S.PolicyVersion}}</code>{{end}}: {{.S.PIIOutcome}}.
      {{if not .S.Protects}}
      <form method="post" action="/router/setup/protect" style="margin-top:12px"><button class="btn" type="submit">Keep personal data and secrets local</button>
      <span style="font-size:.8rem;color:#8fa1bf;margin-left:8px">Adds the rule on <a href="/router/policy" style="color:#7fd3ff">Policy</a> and activates it — audited, no restart. Prefer a full pack? Set <code>ROUTER_POLICY_PATH=builtin:nis2-baseline</code>.</span></form>
      {{else}}<span style="color:#8fa1bf">See every rule on <a href="/router/policy" style="color:#7fd3ff">Policy</a>.</span>{{end}}
    </div>
  </div>

  <div class="step{{if gt .S.Requests 0}} done{{end}}">
    <div class="sh"><span class="num">4</span><span class="st">See it work</span><span class="state">{{if gt .S.Requests 0}}{{.S.Requests}} requests{{else}}not yet{{end}}</span></div>
    <div class="body">A dry-run shows the decision without sending anything: the first should stay local, the second go to the cloud. The step completes with the first real request — send one from the live chat or a connected client.
      <div class="tries">
        <div class="try"><span class="p">Summarise the case for customer 811218-9876 who complained about an invoice</span><span class="res" data-p="Summarise the case for customer 811218-9876 who complained about an invoice">—</span></div>
        <div class="try"><span class="p">write a concise git commit message for a bugfix</span><span class="res" data-p="write a concise git commit message for a bugfix">—</span></div>
      </div>
      <div style="margin-top:12px;display:flex;gap:10px;flex-wrap:wrap"><button class="btn" type="button" onclick="tryAll()">Run the dry-run</button><a class="btn ghost" href="/chat">Open the live chat</a><a class="btn ghost" href="/connect">Connect a client</a></div>
    </div>
  </div>
</div></div></div>
<script>
async function tryAll(){
  for(const el of document.querySelectorAll('.res[data-p]')){
    el.className='res';el.textContent='…';
    try{
      const r=await fetch('/router/policy/dryrun',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json'},body:JSON.stringify({prompt:el.dataset.p})});
      const j=await r.json();
      if(j.blocked){el.className='res blocked';el.textContent='blocked · '+(j.block_code||'fail-closed');}
      else{el.className='res '+(j.egress==='local'?'local':'cloud');el.textContent=(j.egress||'?')+' · '+(j.selected_model||'');}
    }catch(e){el.textContent='error';}
  }
}
</script>
</body></html>`))
