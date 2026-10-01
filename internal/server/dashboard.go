package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/bandit"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/health"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/outcomes"
	"github.com/magnusfroste/sluss/internal/spend"
)

// DashboardOptions configures the dashboard handler.
type DashboardOptions struct {
	Spend       *spend.Tracker
	Health      *health.Tracker
	Outcomes    *outcomes.Store
	Comparisons *eventlog.ComparisonTracker
	RequestLog  *eventlog.RequestLogTracker
	// History, when set, is the durable SQLite-backed source (ISSUE-070) and
	// takes precedence over the in-memory Spend/RequestLog for totals, route
	// distribution and the recent-requests log — so the dashboard survives
	// restarts and redeploys.
	History *history.Store
	Logger  *slog.Logger
	Version string // registry version label

	// Premium-tier per-token pricing (micros per million tokens), used to
	// compute the "saved vs all-premium" baseline. Zero disables the card.
	PremiumInputMicrosPerMTok  int64
	PremiumOutputMicrosPerMTok int64
	// PremiumBaselineModel names the model behind that baseline (e.g.
	// "openai/gpt-4o") so the dashboard can say which counterfactual it prices.
	PremiumBaselineModel string

	// Green receipt: estimated energy per million tokens (Wh) per model, and
	// the premium-tier baseline, so the same counterfactual math that prices
	// savings can report estimated energy/CO2e saved. Zero disables the line.
	EnergyWhPerMTokByModel map[string]float64
	PremiumEnergyWhPerMTok float64
	GridCO2eGramsPerKWh    float64

	// CISO-facing sections (ISSUE-081). Optional; each renders an honest empty
	// state when its source is absent.
	Engine           *engine.Engine    // provider inventory + compliance tags (egress surface)
	AuditMemory      *audit.MemorySink // block/PII breakdowns for the audit window
	Bandit           *bandit.Bandit    // learned best model per task class
	ConservativeMode bool              // incident lever status
}

// GreenSummary is the estimated environmental counterpart of SavingsSummary:
// the energy the observed traffic used versus an all-premium baseline, using
// per-tier order-of-magnitude energy estimates. Clearly an estimate — routing
// to smaller models saves watts with the same math that saves dollars.
type GreenSummary struct {
	ActualWh       float64 `json:"actual_wh"`
	BaselineWh     float64 `json:"baseline_wh"`
	SavedWh        float64 `json:"saved_wh"`
	SavedCO2eGrams float64 `json:"saved_co2e_grams"`
}

// SavingsSummary quantifies the headline value proposition: what the observed
// traffic actually cost versus what it would have cost if every request had used
// the premium model (same token usage, premium pricing).
type SavingsSummary struct {
	ActualUSD          float64 `json:"actual_usd"`
	PremiumBaselineUSD float64 `json:"premium_baseline_usd"`
	SavedUSD           float64 `json:"saved_usd"`
	SavedPct           float64 `json:"saved_pct"`
	BaselineModel      string  `json:"baseline_model,omitempty"`
}

// DashboardData is the JSON payload returned by /router/dashboard/data.
type DashboardData struct {
	Version        string                      `json:"registry_version"`
	TotalRequests  int64                       `json:"total_requests"`
	TotalCostUSD   float64                     `json:"total_cost_usd"`
	Savings        SavingsSummary              `json:"savings"`
	Green          GreenSummary                `json:"green"`
	RoutesByModel  []spend.ModelRow            `json:"routes_by_model"`
	SpendByTenant  []spend.TenantRow           `json:"spend_by_tenant"`
	ProviderHealth map[string]float64          `json:"provider_health"`
	Acceptance     []outcomes.AcceptanceRow    `json:"acceptance"`
	OutcomeCount   int                         `json:"outcome_count"`
	ShadowSummary  eventlog.ComparisonSummary  `json:"shadow_summary"`
	ShadowRecent   []eventlog.ComparisonRecord `json:"shadow_recent"`
	RecentRequests []eventlog.RequestLogRecord `json:"recent_requests"`
	TaskFilter     string                      `json:"task_filter,omitempty"`

	// CISO-facing sections (ISSUE-081): make the egress control and the Fas 4
	// learning visible, which until now lived only in headers/CLI/logs.
	Egress   EgressComplianceView `json:"egress_compliance"`
	Learning LearningRoutingView  `json:"learning_routing"`
	// DataFlow leads the dashboard (ISSUE-116): where the data went, per
	// sensitivity class, from durable history.
	DataFlow DataFlowView `json:"data_flow"`
	// Durable is true when totals come from SQLite history (they survive
	// restarts), false for in-memory counters.
	Durable bool `json:"durable"`
}

// kvRow is a sorted key/count pair for template rendering.
type kvRow struct {
	Key string
	Val int
}

// EgressComplianceView surfaces what left the house, where, and what the gate
// stopped — the CISO's "do we have control?" view.
type EgressComplianceView struct {
	Providers        []ReportProvider `json:"providers"`
	OpenCircuits     []string         `json:"open_circuits"`
	ConservativeMode bool             `json:"conservative_mode"`
	BlockedByCode    map[string]int   `json:"blocked_by_code,omitempty"`
	PIITypeCounts    map[string]int   `json:"pii_type_counts,omitempty"`
	AuditWindow      int              `json:"audit_window"`
	Available        bool             `json:"available"`
}

// LearningRoutingView surfaces the Fas 4 online learning (bandit arms) so the
// improvement loop is visible, not just in logs.
type LearningRoutingView struct {
	BanditArms    []bandit.ArmStat `json:"bandit_arms,omitempty"`
	BanditEnabled bool             `json:"bandit_enabled"`
}

// DashboardHandler returns the /router/dashboard HTML handler and
// /router/dashboard/data JSON handler.
func DashboardHandler(opts DashboardOptions) (html http.HandlerFunc, data http.HandlerFunc) {
	data = func(w http.ResponseWriter, r *http.Request) {
		d := buildDashboardData(opts, r.URL.Query().Get("task"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(d)
	}

	html = func(w http.ResponseWriter, r *http.Request) {
		d := buildDashboardData(opts, r.URL.Query().Get("task"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := dashboardTmpl.Execute(w, d); err != nil {
			slog.Default().Error("dashboard template error", "err", err)
		}
	}
	return html, data
}

func buildDashboardData(opts DashboardOptions, taskFilter string) DashboardData {
	d := DashboardData{
		Version:        opts.Version,
		ProviderHealth: map[string]float64{},
		TaskFilter:     taskFilter,
	}
	switch {
	case opts.History != nil:
		d.TotalRequests = opts.History.TotalRequests()
		d.TotalCostUSD = opts.History.TotalCostUSD()
		d.RoutesByModel = opts.History.ByModel()
		d.SpendByTenant = opts.History.ByTenant()
	case opts.Spend != nil:
		d.TotalRequests = opts.Spend.TotalRequests()
		d.TotalCostUSD = opts.Spend.TotalCostUSD()
		d.RoutesByModel = opts.Spend.ByModel()
		d.SpendByTenant = opts.Spend.ByTenant()
	}
	if len(d.RoutesByModel) > 0 || d.TotalCostUSD > 0 {
		d.Savings = computeSavings(d.RoutesByModel, d.TotalCostUSD, opts.PremiumInputMicrosPerMTok, opts.PremiumOutputMicrosPerMTok, opts.PremiumBaselineModel)
		d.Green = computeGreen(d.RoutesByModel, opts.EnergyWhPerMTokByModel, opts.PremiumEnergyWhPerMTok, opts.GridCO2eGramsPerKWh)
	}
	if opts.Health != nil {
		d.ProviderHealth = opts.Health.Providers()
	}
	if opts.Outcomes != nil {
		d.OutcomeCount = opts.Outcomes.Count()
		d.Acceptance = opts.Outcomes.Acceptance(taskFilter)
	}
	if opts.Comparisons != nil {
		d.ShadowSummary = opts.Comparisons.Summary()
		d.ShadowRecent = opts.Comparisons.Recent(taskFilter)
	}
	if opts.History != nil {
		d.RecentRequests = opts.History.Recent(50)
	} else if opts.RequestLog != nil {
		d.RecentRequests = opts.RequestLog.Recent(50)
	}
	d.Egress = buildEgressView(opts)
	d.Learning = buildLearningView(opts)
	var memLog []eventlog.RequestLogRecord
	if opts.History == nil && opts.RequestLog != nil {
		memLog = opts.RequestLog.Recent(0)
	}
	d.DataFlow = buildDataFlow(opts.History, memLog, opts.Engine)
	d.Durable = opts.History != nil
	return d
}

// buildEgressView assembles the "Egress & Compliance" section: the provider
// inventory (with compliance tags), open circuits, conservative mode, and the
// audit-window block/PII breakdowns. Reuses the compliance report's summarizer.
func buildEgressView(opts DashboardOptions) EgressComplianceView {
	v := EgressComplianceView{ConservativeMode: opts.ConservativeMode}
	if opts.Engine != nil && opts.Engine.Registry != nil {
		if snap, err := opts.Engine.Registry.Active(); err == nil {
			v.Available = true
			var healthMap map[string]float64
			if opts.Health != nil {
				healthMap = opts.Health.Providers()
			}
			for _, p := range snap.Providers() {
				rp := ReportProvider{ID: p.ID, ComplianceTags: p.ComplianceTags,
					Models: len(snap.ModelsForProvider(p.ID))}
				if h, ok := healthMap[p.ID]; ok {
					rp.Health, rp.HasHealth = h, true
				}
				v.Providers = append(v.Providers, rp)
				if opts.Health != nil && opts.Health.CircuitOpen(p.ID) {
					v.OpenCircuits = append(v.OpenCircuits, p.ID)
				}
			}
			sort.Slice(v.Providers, func(i, j int) bool { return v.Providers[i].ID < v.Providers[j].ID })
			sort.Strings(v.OpenCircuits)
		}
	}
	if opts.AuditMemory != nil {
		w := summarizeAuditWindow(opts.AuditMemory.Entries())
		v.Available = true
		v.AuditWindow = w.Entries
		v.BlockedByCode = w.BlockedByCode
		v.PIITypeCounts = w.PIITypeCounts
	}
	return v
}

// buildLearningView surfaces the bandit's learned best model per task class.
func buildLearningView(opts DashboardOptions) LearningRoutingView {
	v := LearningRoutingView{}
	if opts.Bandit != nil && opts.Bandit.Arms() > 0 {
		v.BanditEnabled = true
		v.BanditArms = opts.Bandit.Snapshot()
	} else if opts.Bandit != nil {
		v.BanditEnabled = true
	}
	return v
}

// computeSavings estimates spend versus an all-premium baseline: every request's
// actual token usage repriced at the premium model's per-token rate. The
// baseline is an estimate (premium might generate different-length outputs), so
// it is the conservative, observable "what you'd pay without routing" figure.
func computeSavings(rows []spend.ModelRow, actualUSD float64, premInMicrosPerMTok, premOutMicrosPerMTok int64, baselineModel string) SavingsSummary {
	s := SavingsSummary{ActualUSD: actualUSD, BaselineModel: baselineModel}
	if premInMicrosPerMTok <= 0 && premOutMicrosPerMTok <= 0 {
		return s
	}
	var premiumUSD float64
	for _, r := range rows {
		// micros = tokens * (microsPerMillionTok / 1e6); USD = micros / 1e6.
		premiumUSD += (float64(r.InputTokens)*float64(premInMicrosPerMTok) +
			float64(r.OutputTokens)*float64(premOutMicrosPerMTok)) / 1e12
	}
	s.PremiumBaselineUSD = premiumUSD
	s.SavedUSD = premiumUSD - actualUSD
	if premiumUSD > 0 {
		s.SavedPct = (1 - actualUSD/premiumUSD) * 100
	}
	return s
}

// computeGreen estimates energy versus an all-premium baseline: each request's
// actual token usage priced in Wh at the model's estimated energy rate vs the
// premium tier's. Models without an estimate count as premium (conservative:
// they contribute zero saving). CO2e uses a configurable grid intensity.
func computeGreen(rows []spend.ModelRow, whByModel map[string]float64, premiumWhPerMTok, gridGramsPerKWh float64) GreenSummary {
	var g GreenSummary
	if premiumWhPerMTok <= 0 {
		return g
	}
	for _, r := range rows {
		tokens := float64(r.InputTokens + r.OutputTokens)
		modelWh, ok := whByModel[r.ModelID]
		if !ok || modelWh <= 0 {
			modelWh = premiumWhPerMTok
		}
		g.ActualWh += tokens * modelWh / 1e6
		g.BaselineWh += tokens * premiumWhPerMTok / 1e6
	}
	g.SavedWh = g.BaselineWh - g.ActualWh
	if gridGramsPerKWh > 0 {
		g.SavedCO2eGrams = g.SavedWh / 1000 * gridGramsPerKWh
	}
	return g
}

var dashboardTmpl = template.Must(template.New("dashboard").Funcs(template.FuncMap{
	"adminCSS": adminCSSFunc,
	"adminNav": adminNavFunc,
	"usd":      readableUSD,
	"pct":      func(v float64) string { return fmt.Sprintf("%.0f%%", v*100) },
	"clock":    func(t time.Time) string { return t.Local().Format("15:04:05") },
	"wh": func(v float64) string {
		if v >= 1000 {
			return fmt.Sprintf("%.2f kWh", v/1000)
		}
		return fmt.Sprintf("%.1f Wh", v)
	},
	"co2": func(grams float64) string {
		if grams >= 1000 {
			return fmt.Sprintf("%.2f kg", grams/1000)
		}
		return fmt.Sprintf("%.1f g", grams)
	},
	"join":   func(s []string) string { return strings.Join(s, ", ") },
	"hasMap": func(m map[string]int) bool { return len(m) > 0 },
	"sumMap": func(m map[string]int) int {
		n := 0
		for _, v := range m {
			n += v
		}
		return n
	},
	"sortedMap": func(m map[string]int) []kvRow {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		rows := make([]kvRow, 0, len(keys))
		for _, k := range keys {
			rows = append(rows, kvRow{Key: k, Val: m[k]})
		}
		return rows
	},
	"meanPct": func(v float64) string { return fmt.Sprintf("%.0f%%", v*100) },
	"healthClass": func(v float64) string {
		switch {
		case v >= 0.9:
			return "ok"
		case v >= 0.5:
			return "warn"
		default:
			return "bad"
		}
	},
	"bar": func(requests, total int64) string {
		if total == 0 {
			return "0%"
		}
		pct := float64(requests) / float64(total) * 100
		return fmt.Sprintf("%.0f%%", pct)
	},
	"repeat": func(n int, s string) string { return strings.Repeat(s, n) },
}).Parse(`<!DOCTYPE html>
<html lang="sv">
<head>
<meta charset="utf-8">
<title>Sluss — Router Dashboard</title>
<style>
{{adminCSS}}
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:"IBM Plex Sans",system-ui,sans-serif;background:#081328;color:#e2e8f0}
h1{font-size:1.5rem;font-weight:700;margin-bottom:0.25rem;color:#f8fafc}
.subtitle{font-size:0.85rem;color:#64748b;margin-bottom:2rem}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:1rem;margin-bottom:2rem}
.card{background:#101c34;border:1px solid #22324f;border-radius:10px;padding:1.25rem}
.card-label{font-size:0.75rem;color:#64748b;text-transform:uppercase;letter-spacing:.05em;margin-bottom:0.35rem}
.card-value{font-size:1.75rem;font-weight:700;color:#f8fafc}
.card-sub{font-size:0.78rem;color:#94a3b8;margin-top:0.2rem}
.hero{background:linear-gradient(135deg,#064e3b,#065f46);border:1px solid #10b981;border-radius:12px;padding:1.5rem 1.75rem;margin-bottom:1.5rem}
.hero-label{font-size:0.78rem;color:#a7f3d0;text-transform:uppercase;letter-spacing:.06em}
.hero-value{font-size:2.6rem;font-weight:800;color:#34d399;line-height:1.1;margin:0.15rem 0}
.hero-sub{font-size:0.95rem;color:#d1fae5}
table{width:100%;border-collapse:collapse;margin-bottom:2rem}
th{text-align:left;font-size:0.72rem;color:#64748b;text-transform:uppercase;letter-spacing:.05em;padding:0.6rem 0.75rem;border-bottom:1px solid #22324f}
td{padding:0.6rem 0.75rem;border-bottom:1px solid #101c34;font-size:0.88rem}
tr:hover td{background:#101c34}
.bar-bg{background:#22324f;border-radius:4px;height:6px;width:120px;display:inline-block;vertical-align:middle;margin-left:0.5rem}
.bar-fill{background:#3b82f6;border-radius:4px;height:6px;display:block}
h2{font-size:1rem;font-weight:600;margin-bottom:0.75rem;color:#cbd5e1}
section{margin-bottom:2.5rem}
.ok{color:#22c55e}.warn{color:#f59e0b}.bad{color:#ef4444}
.dot{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:6px;vertical-align:middle}
.dot-ok{background:#22c55e}.dot-warn{background:#f59e0b}.dot-bad{background:#ef4444}
.mono{font-family:ui-monospace,monospace;font-size:0.82rem}
.num{text-align:right;font-variant-numeric:tabular-nums}
th.num{text-align:right}
.ev-links{display:flex;flex-wrap:wrap;gap:6px 14px;margin:6px 0 4px}
.ev-links a{color:#7fd3ff;text-decoration:none;font-weight:600;font-size:0.92rem;border-bottom:1px solid rgba(127,211,255,.3)}
.ev-links a:hover{border-bottom-color:#7fd3ff}
.flow-bar{display:flex;height:14px;border-radius:7px;overflow:hidden;background:#22324f;margin:0.25rem 0 0.6rem}
.seg{display:block;height:100%}
.seg-local{background:#22c55e}.seg-cloud{background:#3b82f6}.seg-blocked{background:#ef4444}
.flow-legend{display:flex;gap:18px;font-size:0.82rem;color:#cbd5e1;margin-bottom:1rem}
.flow-legend i{display:inline-block;width:10px;height:10px;border-radius:3px;margin-right:6px;vertical-align:-1px}
.flow-table{max-width:760px}
.cls{font-family:ui-monospace,monospace;font-size:0.8rem;background:#1e293b;border:1px solid #334155;border-radius:6px;padding:1px 7px}
.note{font-size:0.78rem;color:#f59e0b}
details.adv{margin:0 0 2.5rem;border:1px solid #22324f;border-radius:10px;padding:0.9rem 1.1rem}
details.adv>summary{cursor:pointer;color:#94a3b8;font-size:0.9rem;font-weight:600}
details.adv[open]>summary{margin-bottom:1.25rem}
</style>
</head>
<body>
<div class="tk-shell">
{{adminNav "dashboard"}}
<div class="tk-main" style="padding:2rem 2rem 3rem">
<h1>Dashboard</h1>
<p class="subtitle">Where your AI data went, what the gate stopped, and what routing saved — live.</p>

{{with .DataFlow}}
<div class="grid" style="margin-bottom:1rem">
  <div class="card">
    <div class="card-label">Stayed in the house</div>
    <div class="card-value ok">{{.Local}}</div>
    <div class="card-sub">{{.Pct .Local}}% of requests · local / on-prem models</div>
  </div>
  <div class="card">
    <div class="card-label">Blocked fail-closed</div>
    <div class="card-value{{if gt .Blocked 0}} bad{{end}}">{{.Blocked}}</div>
    <div class="card-sub">no compliant model · never a silent fallback</div>
  </div>
  <div class="card">
    <div class="card-label">Personal data (PII)</div>
    <div class="card-value">{{.PII.Total}}</div>
    <div class="card-sub">{{if gt .PII.Cloud 0}}<span class="bad">{{.PII.Cloud}} went to the cloud</span>{{else if gt .PII.Total 0}}{{.PII.Local}} kept in the house · {{.PII.Blocked}} blocked{{else}}none seen yet{{end}} · types only, never values</div>
  </div>
  <div class="card ev">
    <div class="card-label">Evidence</div>
    <div class="ev-links"><a href="/router/incident-report?window=24h">Incident 24h</a><a href="/router/gap-report">Gap report</a><a href="/router/audit/export">Audit chain</a><a href="/router/compliance/report">Control report</a></div>
    <div class="card-sub">counts and classes — never prompt content</div>
  </div>
</div>

<section class="flow">
<h2>Where your data went</h2>
{{if gt .Total 0}}
<div class="flow-bar" role="img" aria-label="{{.Local}} local, {{.Cloud}} cloud, {{.Blocked}} blocked">
  {{if gt .Local 0}}<span class="seg seg-local" style="width:{{.Pct .Local}}%"></span>{{end}}{{if gt .Cloud 0}}<span class="seg seg-cloud" style="width:{{.Pct .Cloud}}%"></span>{{end}}{{if gt .Blocked 0}}<span class="seg seg-blocked" style="width:{{.Pct .Blocked}}%"></span>{{end}}
</div>
<div class="flow-legend"><span><i class="seg-local"></i>Local {{.Local}}</span><span><i class="seg-cloud"></i>Cloud {{.Cloud}}</span><span><i class="seg-blocked"></i>Blocked {{.Blocked}}</span>{{if gt .Unknown 0}}<span style="color:#94a3b8">Unknown {{.Unknown}} (older rows whose model and provider were removed)</span>{{end}}</div>
<table class="flow-table">
<thead><tr><th>Data class</th><th class="num">Local</th><th class="num">Cloud</th><th class="num">Blocked</th><th></th></tr></thead>
<tbody>
{{range .Rows}}
<tr>
  <td>{{if eq .Sensitivity "none"}}<span style="color:#94a3b8">no sensitive data</span>{{else}}<span class="cls">{{.Sensitivity}}</span>{{end}}</td>
  <td class="num ok">{{.Local}}</td>
  <td class="num{{if and (gt .Cloud 0) (ne .Sensitivity "none")}} warn{{end}}">{{.Cloud}}</td>
  <td class="num{{if gt .Blocked 0}} bad{{end}}">{{.Blocked}}</td>
  <td class="note">{{if and (gt .Cloud 0) (ne .Sensitivity "none")}}left the house — allowed by your policy{{end}}</td>
</tr>
{{end}}
</tbody>
</table>
<p class="subtitle" style="margin:-1.25rem 0 0">Retained history · classification by deterministic rules, no LLM · a sensitive class in the cloud column is a policy choice you can change on <a href="/router/policy" style="color:#7fd3ff">Policy</a>.</p>
{{else}}
<p class="subtitle">No requests yet — send one from <a href="/demo" style="color:#7fd3ff">Live chat</a> or any connected client.</p>
{{end}}
</section>
{{end}}

{{if gt .Savings.PremiumBaselineUSD 0.0}}
<div class="hero">
  <div class="hero-label">Saved vs all-premium</div>
  <div class="hero-value">{{printf "%.1f%%" .Savings.SavedPct}} cheaper</div>
  <div class="hero-sub">Saved <strong>{{usd .Savings.SavedUSD}}</strong> — you paid {{usd .Savings.ActualUSD}}; routing everything to the premium model{{with .Savings.BaselineModel}} ({{.}}){{end}} would have cost {{usd .Savings.PremiumBaselineUSD}}.</div>
  {{if gt .Green.SavedWh 0.0}}<div class="hero-sub" style="margin-top:6px">Also saved ≈ <strong>{{wh .Green.SavedWh}}</strong> of energy ({{co2 .Green.SavedCO2eGrams}} CO₂e) vs all-premium — estimated from per-tier energy figures.</div>{{end}}
</div>
{{end}}

<div class="grid">
  <div class="card">
    <div class="card-label">Total requests</div>
    <div class="card-value">{{.TotalRequests}}</div>
    <div class="card-sub">{{if .Durable}}routed · retained history{{else}}routed · since last restart{{end}}</div>
  </div>
  <div class="card">
    <div class="card-label">Estimated spend</div>
    <div class="card-value">{{usd .TotalCostUSD}}</div>
    <div class="card-sub">USD (estimate)</div>
  </div>
  <div class="card">
    <div class="card-label">Models tracked</div>
    <div class="card-value">{{len .RoutesByModel}}</div>
    <div class="card-sub">with requests</div>
  </div>
  <div class="card">
    <div class="card-label">Tenants</div>
    <div class="card-value">{{len .SpendByTenant}}</div>
    <div class="card-sub">active</div>
  </div>
{{if gt .ShadowSummary.Total 0}}
  <div class="card">
    <div class="card-label">Shadow comparisons</div>
    <div class="card-value">{{.ShadowSummary.Total}}</div>
    <div class="card-sub">{{.ShadowSummary.ChangedCount}} changed vs actual</div>
  </div>
  <div class="card">
    <div class="card-label">Shadow cost delta</div>
    <div class="card-value">{{usd .ShadowSummary.EstimatedCostDeltaUSD}}</div>
    <div class="card-sub">shadow minus actual</div>
  </div>
{{end}}
</div>

<section>
<h2>Route distribution</h2>
<table>
<thead><tr><th>Model</th><th>Provider</th><th>Requests</th><th>Distribution</th><th>Input tokens</th><th>Output tokens</th><th>Cost (est.)</th></tr></thead>
<tbody>
{{$total := .TotalRequests}}
{{range .RoutesByModel}}
<tr>
  <td class="mono">{{.ModelID}}</td>
  <td class="mono">{{.ProviderID}}</td>
  <td>{{.Requests}}</td>
  <td>
    <span class="bar-bg"><span class="bar-fill" style="width:{{bar .Requests $total}}"></span></span>
    <span style="font-size:0.78rem;color:#94a3b8;margin-left:4px">{{bar .Requests $total}}</span>
  </td>
  <td>{{.InputTokens}}</td>
  <td>{{.OutputTokens}}</td>
  <td>{{usd .CostUSD}}</td>
</tr>
{{else}}<tr><td colspan="7" style="color:#64748b;text-align:center;padding:1.5rem">No requests yet — send some!</td></tr>
{{end}}
</tbody>
</table>
</section>

<section>
<h2>Provider health</h2>
<table>
<thead><tr><th>Provider</th><th>Health score</th><th>Status</th></tr></thead>
<tbody>
{{range $id, $score := .ProviderHealth}}
<tr>
  <td class="mono">{{$id}}</td>
  <td>{{pct $score}}</td>
  <td><span class="dot dot-{{healthClass $score}}"></span><span class="{{healthClass $score}}">{{if ge $score 0.9}}Healthy{{else if ge $score 0.5}}Degraded{{else}}Down{{end}}</span></td>
</tr>
{{else}}<tr><td colspan="3" style="color:#64748b;text-align:center;padding:1.5rem">No provider calls recorded yet</td></tr>
{{end}}
</tbody>
</table>
</section>

<section>
<h2>Egress &amp; Compliance</h2>
{{if .Egress.Available}}
<p style="font-size:0.78rem;color:#64748b;margin-bottom:0.6rem">
  Where data left the house, and what the gate stopped · conservative mode: {{if .Egress.ConservativeMode}}<span class="warn">on</span>{{else}}off{{end}}
  {{if .Egress.OpenCircuits}} · <span class="bad">open circuits: {{join .Egress.OpenCircuits}}</span>{{end}}
  {{if gt .Egress.AuditWindow 0}} · audit window: {{.Egress.AuditWindow}} events{{end}}
</p>
<table>
<thead><tr><th>Provider (egress)</th><th>Compliance tags</th><th>Models</th><th>Health</th></tr></thead>
<tbody>
{{range .Egress.Providers}}
<tr>
  <td class="mono">{{.ID}}</td>
  <td>{{if .ComplianceTags}}{{range .ComplianceTags}}<span class="dot dot-ok"></span>{{.}} {{end}}{{else}}<span style="color:#64748b">— otaggad</span>{{end}}</td>
  <td>{{.Models}}</td>
  <td>{{if .HasHealth}}<span class="{{healthClass .Health}}">{{pct .Health}}</span>{{else}}—{{end}}</td>
</tr>
{{else}}<tr><td colspan="4" style="color:#64748b;text-align:center;padding:1rem">No providers registered</td></tr>
{{end}}
</tbody>
</table>
{{if or (hasMap .Egress.BlockedByCode) (hasMap .Egress.PIITypeCounts)}}
<p style="font-size:0.82rem;color:#cbd5e1;margin-top:0.4rem">
  {{if hasMap .Egress.BlockedByCode}}<strong>Blocked (audit window):</strong> {{range sortedMap .Egress.BlockedByCode}}<span class="mono">{{.Key}}</span>×{{.Val}} {{end}}<br>{{end}}
  {{if hasMap .Egress.PIITypeCounts}}<strong>PII hits (types, never values):</strong> {{range sortedMap .Egress.PIITypeCounts}}<span class="mono">{{.Key}}</span>×{{.Val}} {{end}}{{end}}
</p>
{{else}}<p style="font-size:0.78rem;color:#64748b">No blocks or PII hits in the audit window yet.</p>{{end}}
{{else}}
<p style="color:#64748b;padding:0.5rem 0">The egress view requires registry/audit — no data yet.</p>
{{end}}
</section>

<details class="adv"{{if or .Learning.BanditArms (gt .ShadowSummary.Total 0) (gt .OutcomeCount 0)}} open{{end}}>
<summary>Advanced — adaptive learning, shadow routing, acceptance feedback</summary>
<section>
<h2>Learning &amp; routing</h2>
{{if .Learning.BanditEnabled}}
  {{if .Learning.BanditArms}}
  <p style="font-size:0.78rem;color:#64748b;margin-bottom:0.6rem">What the router has learned per task class from real outcomes (UCB1 bandit).</p>
  <table>
  <thead><tr><th>Task class</th><th>Model</th><th>Pulls</th><th>Mean reward</th></tr></thead>
  <tbody>
  {{range .Learning.BanditArms}}
  <tr><td class="mono">{{.TaskClass}}</td><td class="mono">{{.ModelID}}</td><td>{{.Pulls}}</td><td>{{meanPct .MeanReward}}</td></tr>
  {{end}}
  </tbody>
  </table>
  {{else}}<p style="font-size:0.78rem;color:#64748b">Bandit active but nothing learned yet (no pulls).</p>{{end}}
{{else}}<p style="font-size:0.78rem;color:#64748b">Bandit routing is off — routes follow policy and scoring only. (Adaptive learning can be enabled with <span class="mono">ROUTER_BANDIT_ENABLED</span>.)</p>{{end}}
</section>

<section>
<h2>Shadow routing {{if .TaskFilter}}<span style="font-size:0.78rem;color:#64748b">· filtered: {{.TaskFilter}}</span>{{end}}</h2>
<p style="font-size:0.78rem;color:#64748b;margin-bottom:0.6rem">
  {{.ShadowSummary.ChangedCount}} / {{.ShadowSummary.Total}} comparisons changed · route {{.ShadowSummary.RouteChangedCount}} · fallback {{.ShadowSummary.FallbackChangedCount}} · timeout {{.ShadowSummary.TimeoutChangedCount}} · verifier {{.ShadowSummary.VerifierChangedCount}} · policy version {{.ShadowSummary.PolicyVersionChangedCount}} · cost {{.ShadowSummary.CostChangedCount}}
</p>
<table>
<thead><tr><th>Request</th><th>Task</th><th>Actual</th><th>Shadow</th><th>Changed</th><th>Cost delta</th><th>Policy versions</th></tr></thead>
<tbody>
{{range .ShadowRecent}}
<tr>
  <td class="mono">{{.RequestID}}</td>
  <td class="mono">{{.TaskType}}</td>
  <td class="mono">{{.Comparison.Primary.SelectedProvider}}/{{.Comparison.Primary.SelectedModel}}</td>
  <td class="mono">{{.Comparison.Secondary.SelectedProvider}}/{{.Comparison.Secondary.SelectedModel}}</td>
  <td>{{if .Comparison.Changed}}yes{{else}}no{{end}}</td>
  <td>{{usd .Comparison.EstimatedCostDeltaUSD}}</td>
  <td class="mono">{{.Comparison.Primary.PolicyVersion}} → {{.Comparison.Secondary.PolicyVersion}}</td>
</tr>
{{else}}<tr><td colspan="7" style="color:#64748b;text-align:center;padding:1.5rem">No shadow comparisons recorded yet</td></tr>
{{end}}
</tbody>
</table>
</section>

<section>
<h2>Acceptance feedback {{if .TaskFilter}}<span style="font-size:0.78rem;color:#64748b">· filtered: {{.TaskFilter}}</span>{{end}}</h2>
<p style="font-size:0.78rem;color:#64748b;margin-bottom:0.6rem">{{.OutcomeCount}} outcome(s) reported · filter by task class via <span class="mono">?task=&lt;task_type&gt;</span></p>
<table>
<thead><tr><th>Model</th><th>Task class</th><th>Outcomes</th><th>Accepted</th><th>Rejected</th><th>Partial</th><th>Acceptance rate</th></tr></thead>
<tbody>
{{range .Acceptance}}
<tr>
  <td class="mono">{{.Model}}</td>
  <td class="mono">{{.TaskType}}</td>
  <td>{{.Total}}</td>
  <td class="ok">{{.Accepted}}</td>
  <td class="bad">{{.Rejected}}</td>
  <td class="warn">{{.Partial}}</td>
  <td>
    <span class="bar-bg"><span class="bar-fill" style="width:{{pct .AcceptanceRate}};background:{{if ge .AcceptanceRate 0.7}}#22c55e{{else if ge .AcceptanceRate 0.4}}#f59e0b{{else}}#ef4444{{end}}"></span></span>
    <span style="font-size:0.78rem;color:#94a3b8;margin-left:4px">{{pct .AcceptanceRate}}</span>
  </td>
</tr>
{{else}}<tr><td colspan="7" style="color:#64748b;text-align:center;padding:1.5rem">No outcomes reported yet — POST to /router/outcomes</td></tr>
{{end}}
</tbody>
</table>
</section>

</details>

<section>
<h2>Spend by tenant</h2>
<table>
<thead><tr><th>Tenant</th><th>Requests</th><th>Cost (est.)</th></tr></thead>
<tbody>
{{range .SpendByTenant}}
<tr>
  <td class="mono">{{.TenantID}}</td>
  <td>{{.Requests}}</td>
  <td>{{usd .CostUSD}}</td>
</tr>
{{else}}<tr><td colspan="3" style="color:#64748b;text-align:center;padding:1.5rem">No tenant data yet</td></tr>
{{end}}
</tbody>
</table>
</section>

<p style="font-size:0.72rem;color:#334155">
  JSON: <a href="/router/dashboard/data" style="color:#3b82f6">/router/dashboard/data</a> &nbsp;·&nbsp;
  Metrics: <a href="/metrics" style="color:#3b82f6">/metrics</a>
</p>
</div>
</div>
</body>
</html>
`))
