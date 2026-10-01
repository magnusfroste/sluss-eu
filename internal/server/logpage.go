package server

import (
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/policy"
)

// LogOptions configures the full request-log page. History (durable SQLite) is
// preferred; RequestLog (in-memory ring) is the fallback when no data dir is set.
type LogOptions struct {
	History    *history.Store
	RequestLog *eventlog.RequestLogTracker
	Logger     *slog.Logger
	Version    string
	// Limit caps how many rows the page renders (newest first). 0 → 500.
	Limit int
	// Engine classifies rows recorded before egress was stored (ISSUE-116).
	Engine *engine.Engine
	// Cache supplies the active policy for the per-row explanation (ISSUE-117).
	Cache *policy.Cache
}

// logRow is a request row plus its explanation for the expandable detail.
type logRow struct {
	eventlog.RequestLogRecord
	// Why lists the active-policy rules that match the row's stored
	// classification (task, risk, data class) — re-evaluated, never stored.
	Why []policy.RuleSummary
}

// logFilters are the quick filters on the request log (ISSUE-117).
var logFilters = []string{"blocked", "sensitive", "local", "cloud"}

func logRowMatches(r eventlog.RequestLogRecord, f string) bool {
	switch f {
	case "blocked":
		return r.Blocked
	case "sensitive":
		return r.Sensitivity != "" && r.Sensitivity != "none"
	case "local", "cloud":
		return !r.Blocked && r.Egress == f
	}
	return true
}

// logFilterChip is one quick-filter link with its count.
type logFilterChip struct {
	Key    string
	Count  int
	Active bool
}

// LogPageData is the template payload for the log page.
type LogPageData struct {
	Version string
	Rows    []logRow
	Count   int
	Total   int
	Durable bool
	Filter  string
	Filters []logFilterChip
}

// LogPageHandler renders the full per-request routing log — every message with
// the model the router picked and what it cost. Lives on its own page so the
// dashboard can stay focused on statistics and insights.
func LogPageHandler(opts LogOptions) http.HandlerFunc {
	limit := opts.Limit
	if limit <= 0 {
		limit = 500
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var rows []eventlog.RequestLogRecord
		durable := false
		switch {
		case opts.History != nil:
			rows = opts.History.Recent(limit)
			durable = true
		case opts.RequestLog != nil:
			rows = opts.RequestLog.Recent(limit)
		}
		// Rows from before the egress column (ISSUE-116) are classified from
		// the model's current tags — the same rule the response header uses.
		legacy := ChatOptions{Engine: opts.Engine}
		for i := range rows {
			if rows[i].Egress == "" && !rows[i].Blocked {
				rows[i].Egress = legacy.legacyEgress(rows[i].Model, rows[i].Provider)
			}
		}
		filter := r.URL.Query().Get("show")
		known := false
		for _, f := range logFilters {
			known = known || f == filter
		}
		if !known {
			filter = ""
		}
		chips := make([]logFilterChip, 0, len(logFilters))
		for _, f := range logFilters {
			n := 0
			for _, row := range rows {
				if logRowMatches(row, f) {
					n++
				}
			}
			chips = append(chips, logFilterChip{Key: f, Count: n, Active: f == filter})
		}
		var active *policy.CompiledPolicy
		var summaries map[string]policy.RuleSummary
		if opts.Cache != nil {
			if p, ok := opts.Cache.Active(policy.Scope{}); ok {
				active = p
				summaries = map[string]policy.RuleSummary{}
				for _, s := range p.Summaries() {
					summaries[s.ID] = s
				}
			}
		}
		view := make([]logRow, 0, len(rows))
		for _, row := range rows {
			if filter != "" && !logRowMatches(row, filter) {
				continue
			}
			lr := logRow{RequestLogRecord: row}
			if active != nil {
				ev := active.Evaluate(policy.EvaluationInput{TaskType: row.TaskType, RiskLevel: row.RiskLevel, Sensitivity: row.Sensitivity})
				for _, id := range ev.MatchedRuleIDs {
					if s, ok := summaries[id]; ok {
						lr.Why = append(lr.Why, s)
					}
				}
			}
			view = append(view, lr)
		}
		data := LogPageData{Version: opts.Version, Rows: view, Count: len(view), Total: len(rows),
			Durable: durable, Filter: filter, Filters: chips}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := logTmpl.Execute(w, data); err != nil {
			if opts.Logger != nil {
				opts.Logger.Error("log page render failed", "err", err)
			}
			http.Error(w, "log render error", http.StatusInternalServerError)
		}
	}
}

var logTmpl = template.Must(template.New("log").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
	"usd":      readableUSD,
	"clock":    func(t time.Time) string { return t.Local().Format("15:04:05") },
	"day":      func(t time.Time) string { return t.Local().Format("2006-01-02") },
	"tierClass": func(model string) string {
		switch {
		case model == "":
			return ""
		case containsAny(model, "premium"):
			return "premium"
		case containsAny(model, "balanced"):
			return "balanced"
		default:
			return "cheap"
		}
	},
	"riskClass": func(r string) string {
		switch r {
		case "high", "critical":
			return "bad"
		case "medium":
			return "warn"
		default:
			return "ok"
		}
	},
}).Parse(logPageHTML))

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

const logPageHTML = `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sluss — Request log</title>
<style>
  {{adminCSS}}
  body{margin:0;background:#081328;color:#e8eef7;font-family:"IBM Plex Sans",system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
  header{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;
    padding:20px 26px;border-bottom:1px solid #22324f}
  h1{font-size:1.3rem;margin:0}
  .sub{color:#8fa1bf;font-size:.85rem;margin-top:4px}
  .links a{color:#fad100;text-decoration:none;font-weight:600;margin-left:18px;font-size:.92rem}
  .links a:hover{text-decoration:underline}
  .wrap{padding:22px 24px;overflow-x:auto}
  table{border-collapse:collapse;width:100%;min-width:720px;font-size:13px}
  th,td{text-align:left;padding:9px 9px;border-bottom:1px solid #1a2740;white-space:nowrap;vertical-align:top}
  td .slug{white-space:normal;max-width:230px;overflow-wrap:anywhere}
  th.num{text-align:right}
  td .slug.code{white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:200px}
  thead th{position:sticky;top:0;background:#101c34;color:#8fa1bf;font-size:11.5px;
    letter-spacing:.06em;text-transform:uppercase;font-weight:600}
  tbody tr:hover{background:#101c34}
  .mono{font-family:ui-monospace,Menlo,monospace}
  .num{text-align:right;font-variant-numeric:tabular-nums}
  .slug{color:#64748b;font-size:11.5px;font-family:ui-monospace,Menlo,monospace}
  .pill{display:inline-block;padding:2px 9px;border-radius:999px;font-weight:600;font-size:12px;font-family:ui-monospace,Menlo,monospace}
  .pill.cheap{background:#13351f;color:#4fd08a}.pill.balanced{background:#33290f;color:#f4b740}
  .pill.premium{background:#361529;color:#f07ab0}
  .ok{color:#22c55e}.warn{color:#f59e0b}.bad{color:#ef4444}
  .empty{color:#64748b;text-align:center;padding:3rem}
  .badge-src{font-size:12px;color:#8fa1bf}
  .eg{display:inline-block;padding:2px 9px;border-radius:999px;font-weight:700;font-size:12px}
  .eg-local{background:#0f3a26;color:#4ade80}.eg-cloud{background:#12264a;color:#7fb2ff}
  .eg-blocked{background:#3b1414;color:#f87171}
  .cls{font-family:ui-monospace,Menlo,monospace;font-size:12px;background:#111c30;border:1px solid #2a3a58;border-radius:6px;padding:1px 7px}
  tr.row-blocked td{background:rgba(239,68,68,.05)}
  tr.main{cursor:pointer}
  tr.main.open td{background:#101c34}
  tr.why td{white-space:normal;background:#0a1322;border-bottom:1px solid #22324f;padding:12px 16px 14px;font-size:13px;line-height:1.55}
  .why-head{margin-bottom:6px}.why-cls{color:#b7c4dc}
  .why-rules{margin-top:8px;color:#b7c4dc}.why-rules ol{margin:4px 0 0 18px;padding:0}
  .why-rules .rid{color:#64748b;font-size:11px;margin-left:6px}
  .k-block{color:#f87171}.k-require{color:#4ade80}.k-force{color:#f4b740}
  .why-note{margin-top:8px;color:#64748b;font-size:12px}
  .filters{display:flex;gap:8px;flex-wrap:wrap}
  .filters a{color:#b7c4dc;text-decoration:none;font-size:13px;padding:5px 11px;border:1px solid #22324f;border-radius:999px}
  .filters a span{color:#64748b;margin-left:3px}
  .filters a.on{background:#12305a;border-color:#2c5aa0;color:#e8eef7}
  .hint{color:#64748b;font-size:12.5px;margin:0 0 10px}
</style></head>
<body>
<div class="tk-shell">
{{adminNav "log"}}
<div class="tk-main">
<header>
  <div>
    <h1>Request log</h1>
    <div class="sub">{{if .Filter}}{{.Count}} of {{.Total}}{{else}}{{.Count}}{{end}} requests · <span class="badge-src">{{if .Durable}}durable (SQLite){{else}}in-memory (set ROUTER_DATA_DIR to persist){{end}}</span> · {{.Version}}</div>
  </div>
  <nav class="filters" aria-label="Filter requests">
    <a href="/router/log"{{if not .Filter}} class="on"{{end}}>All</a>
    {{range .Filters}}<a href="/router/log?show={{.Key}}"{{if .Active}} class="on"{{end}}>{{.Key}} <span>{{.Count}}</span></a>{{end}}
  </nav>
</header>
<div class="wrap">
<p class="hint">Click a row to see why it was routed that way.</p>
<table>
<thead><tr><th>Time</th><th>Task</th><th>Risk</th><th>Data class</th><th>Egress</th><th>Model</th>
<th class="num">Tokens</th><th class="num">Cost</th></tr></thead>
<tbody>
{{range .Rows}}
<tr class="main{{if .Blocked}} row-blocked{{end}}" onclick="tg(this)" tabindex="0" onkeydown="if(event.key==='Enter')tg(this)">
  <td class="mono">{{clock .Time}}<div class="slug">{{day .Time}}</div></td>
  <td>{{.TaskType}}{{if and .Blocked .BlockCode}}<div class="slug code" title="{{.BlockCode}}">{{.BlockCode}}</div>{{end}}</td>
  <td class="{{riskClass .RiskLevel}}">{{.RiskLevel}}</td>
  <td>{{if and .Sensitivity (ne .Sensitivity "none")}}<span class="cls">{{.Sensitivity}}</span>{{else}}<span style="color:#475569">—</span>{{end}}</td>
  <td>{{if .Blocked}}<span class="eg eg-blocked">blocked</span>{{else if eq .Egress "local"}}<span class="eg eg-local">local</span>{{else if eq .Egress "cloud"}}<span class="eg eg-cloud">cloud</span>{{else}}<span style="color:#475569">—</span>{{end}}</td>
  <td>{{if .Model}}<span class="pill {{tierClass .Model}}">{{.Model}}</span><div class="slug">{{.Provider}}{{if .ProviderModelID}} · {{.ProviderModelID}}{{end}}</div>{{else}}<span style="color:#475569">—</span>{{end}}</td>
  <td class="num mono">{{.InputTokens}} / {{.OutputTokens}}</td>
  <td class="num mono">{{usd .CostUSD}}</td>
</tr>
<tr class="why" hidden><td colspan="8">
  <div class="why-head">{{if .Blocked}}<span class="eg eg-blocked">blocked</span> fail-closed — <span class="mono">{{.BlockCode}}</span>. No provider was called.{{else if eq .Egress "local"}}<span class="eg eg-local">local</span> Sent to <b>{{.Model}}</b> on {{.Provider}}, a local/on-prem model — the data stayed in the house.{{else if eq .Egress "cloud"}}<span class="eg eg-cloud">cloud</span> Sent to <b>{{.Model}}</b> on {{.Provider}} — no rule required a local model for this classification.{{else}}Destination unknown — an older row whose model and provider were removed.{{end}}</div>
  <div class="why-cls">Classified (deterministic rules, no LLM): task <b>{{.TaskType}}</b> · risk <b>{{.RiskLevel}}</b> · data class <b>{{if .Sensitivity}}{{.Sensitivity}}{{else}}none{{end}}</b> · request <span class="mono">{{.RequestID}}</span></div>
  {{if .Why}}<div class="why-rules">Rules in the current policy that match this classification:
    <ol>{{range .Why}}<li><span class="w">{{.When}}</span> → <b class="k-{{.Kind}}">{{.Then}}</b> <span class="mono rid">{{.ID}}</span></li>{{end}}</ol>
  </div>{{end}}
  <div class="why-note">Re-evaluated from the stored classification — the prompt itself is never stored. Rules on agent tools or prompt terms are not re-checked here; the dry-run on Policy shows the full decision for a new prompt.</div>
</td></tr>
{{else}}
<tr><td colspan="8" class="empty">No requests yet — open the <a href="/chat" style="color:#fad100">live chat</a> and send one.</td></tr>
{{end}}
</tbody>
</table>
</div>
</div>
</div>
<script>
function tg(r){var d=r.nextElementSibling;if(!d||!d.classList.contains('why'))return;d.hidden=!d.hidden;r.classList.toggle('open',!d.hidden);}
</script>
</body></html>`
