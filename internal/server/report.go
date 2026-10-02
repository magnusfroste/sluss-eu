package server

// Compliance report (ISSUE-076): a readable control report a CISO can hand to
// leadership or an auditor. It assembles ONLY existing data — policy cache,
// registry roster (with compliance tags), health/circuit state, audit window,
// request history/spend, and the savings/green math the dashboard already uses.
// No new hot-path dependencies; generation is on-demand and read-only.
//
// Language discipline: this is a CONTROL report ("kontrollrapport"), never a
// compliance certification — we present controls and evidence, we make no
// legal claims.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/health"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/regime"
	"github.com/magnusfroste/sluss/internal/spend"
)

// profileAuthority returns the supervisory context line for a profile key.
func profileAuthority(key string) string {
	if p, ok := regime.Get(key); ok {
		return p.Authority
	}
	return "the relevant national supervisory authority"
}

// ReportOptions carries the data sources for the compliance report. Everything
// is optional; missing sources render as honest empty states.
type ReportOptions struct {
	Dashboard   DashboardOptions // totals, per-tenant, savings, green
	Engine      *engine.Engine   // registry snapshot: providers/models + tags
	PolicyCache *policy.Cache    // active global policy version/settings
	Health      *health.Tracker  // provider health + open circuits
	AuditMemory *audit.MemorySink
	// Incident levers / config facts passed from wiring (env-derived).
	ConservativeMode bool
	BudgetUSD        float64
	AuditChainPath   string // "" when the tamper-evident chain is disabled
	// Profile is the explicit regime profile key (ROUTER_PROFILE): nis2 | dora |
	// gdpr | eu. Empty derives it from the active policy pack (ISSUE-094).
	Profile string
	Now     func() time.Time
}

// ComplianceReport is the JSON shape; the markdown renderer walks the same data.
type ComplianceReport struct {
	GeneratedAt     time.Time `json:"generated_at"`
	RegistryVersion string    `json:"registry_version"`

	// Regim-profil (ISSUE-094): styr ordval/rubriker, aldrig juridiska påståenden.
	Profile        string `json:"profile"`         // nis2 | dora | gdpr | eu
	ProfileName    string `json:"profile_name"`    // human name
	ProfileStatute string `json:"profile_statute"` // named statute (framing)
	ProfileTitle   string `json:"-"`
	ProfileFraming string `json:"-"`

	// Kontrollstatus
	PolicyVersion    string           `json:"policy_version"`
	PolicyRuleCount  int              `json:"policy_rule_count"`
	ConservativeMode bool             `json:"conservative_mode"`
	BudgetCapUSD     float64          `json:"budget_cap_usd"`
	AuditChain       bool             `json:"audit_chain_enabled"`
	Providers        []ReportProvider `json:"providers"`
	OpenCircuits     []string         `json:"open_circuits"`

	// Händelser
	TotalRequests   int64            `json:"total_requests"`
	BlockedRequests int64            `json:"blocked_requests"`
	AuditWindow     ReportAuditStats `json:"audit_window"`

	// Per avdelning
	Tenants []spend.TenantRow `json:"tenants"`

	// ROI (estimat)
	Savings SavingsSummary `json:"savings"`
	Green   GreenSummary   `json:"green"`
}

// ReportProvider is one provider row in the control-status section.
type ReportProvider struct {
	ID             string   `json:"id"`
	ComplianceTags []string `json:"compliance_tags,omitempty"`
	Models         int      `json:"models"`
	Health         float64  `json:"health"`
	HasHealth      bool     `json:"has_health"`
}

// ReportAuditStats summarizes the bounded in-memory audit window. Clearly
// labeled a window — the full, verifiable trail is the exported hash chain.
type ReportAuditStats struct {
	Entries       int            `json:"entries"`
	BlockedByCode map[string]int `json:"blocked_by_code,omitempty"`
	PIITypeCounts map[string]int `json:"pii_type_counts,omitempty"`
	KeyMutations  int            `json:"key_mutations"`
	PolicyReloads int            `json:"policy_reloads"`
}

// BuildComplianceReport assembles the report from the configured sources.
func BuildComplianceReport(o ReportOptions) ComplianceReport {
	now := time.Now
	if o.Now != nil {
		now = o.Now
	}
	r := ComplianceReport{
		GeneratedAt:      now().UTC(),
		ConservativeMode: o.ConservativeMode,
		BudgetCapUSD:     o.BudgetUSD,
		AuditChain:       o.AuditChainPath != "",
	}

	// Kontrollstatus: policy + registry inventory + circuits.
	if o.PolicyCache != nil {
		if pol, ok := o.PolicyCache.Active(policy.Scope{}); ok && pol != nil {
			r.PolicyVersion = pol.Version()
			r.PolicyRuleCount = pol.RuleCount()
		}
	}
	// Regim-profil: explicit (ROUTER_PROFILE) vinner, annars härledd från aktiv
	// policy-pack (pv_nis2_* → NIS2, pv_dora_* → DORA, ...).
	prof := regime.Resolve(o.Profile, r.PolicyVersion)
	r.Profile = prof.Key
	r.ProfileName = prof.Name
	r.ProfileStatute = prof.Statute
	r.ProfileTitle = prof.ReportName
	r.ProfileFraming = prof.Framing
	if o.Engine != nil && o.Engine.Registry != nil {
		if snap, err := o.Engine.Registry.Active(); err == nil {
			r.RegistryVersion = snap.RegistryVersion()
			var healthMap map[string]float64
			if o.Health != nil {
				healthMap = o.Health.Providers()
			}
			for _, p := range snap.Providers() {
				rp := ReportProvider{ID: p.ID, ComplianceTags: p.ComplianceTags,
					Models: len(snap.ModelsForProvider(p.ID))}
				if h, ok := healthMap[p.ID]; ok {
					rp.Health, rp.HasHealth = h, true
				}
				r.Providers = append(r.Providers, rp)
				if o.Health != nil && o.Health.CircuitOpen(p.ID) {
					r.OpenCircuits = append(r.OpenCircuits, p.ID)
				}
			}
			sort.Slice(r.Providers, func(i, j int) bool { return r.Providers[i].ID < r.Providers[j].ID })
			sort.Strings(r.OpenCircuits)
		}
	}

	// Händelser: durable totals from history, breakdowns from the audit window.
	if o.Dashboard.History != nil {
		r.TotalRequests = o.Dashboard.History.TotalRequests()
		r.BlockedRequests = o.Dashboard.History.BlockedRequests()
		r.Tenants = o.Dashboard.History.ByTenant()
	} else if o.Dashboard.Spend != nil {
		r.TotalRequests = o.Dashboard.Spend.TotalRequests()
		r.Tenants = o.Dashboard.Spend.ByTenant()
	}
	if o.AuditMemory != nil {
		r.AuditWindow = summarizeAuditWindow(o.AuditMemory.Entries())
	}

	// ROI: identical math to the dashboard (estimate, and labeled as such).
	d := buildDashboardData(o.Dashboard, "")
	r.Savings = d.Savings
	r.Green = d.Green
	return r
}

func summarizeAuditWindow(entries []audit.Entry) ReportAuditStats {
	s := ReportAuditStats{Entries: len(entries)}
	for _, e := range entries {
		switch e.Action {
		case audit.ActionRequestBlocked:
			code := e.Detail["block_code"]
			if code == "" {
				code = "unspecified"
			}
			if s.BlockedByCode == nil {
				s.BlockedByCode = map[string]int{}
			}
			s.BlockedByCode[code]++
			if pii := e.Detail["pii_types"]; pii != "" {
				if s.PIITypeCounts == nil {
					s.PIITypeCounts = map[string]int{}
				}
				for _, t := range strings.Split(pii, ",") {
					if t = strings.TrimSpace(t); t != "" {
						s.PIITypeCounts[t]++
					}
				}
			}
		case audit.ActionAPIKeyAdd, audit.ActionAPIKeyDisable:
			s.KeyMutations++
		case audit.ActionPolicyReload:
			s.PolicyReloads++
		}
	}
	return s
}

// RenderComplianceMarkdown renders the report as a readable markdown document.
// Auditor language first, model slugs second; estimates are labeled.
func RenderComplianceMarkdown(r ComplianceReport) string {
	var b strings.Builder
	title := r.ProfileTitle
	if title == "" {
		title = "Control report — LLM routing"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "Generated: %s · Registry: %s\n\n", r.GeneratedAt.Format(time.RFC3339), orDash(r.RegistryVersion))
	if r.ProfileName != "" {
		fmt.Fprintf(&b, "Regime profile: **%s** · %s · Supervisory context: %s\n\n",
			r.ProfileName, r.ProfileStatute, profileAuthority(r.Profile))
	}
	b.WriteString("> This is a control report: it presents active controls and observed events.\n> It is not a legal attestation of regulatory compliance.\n\n")
	if r.ProfileFraming != "" {
		fmt.Fprintf(&b, "%s\n\n", r.ProfileFraming)
	}

	b.WriteString("## 1. Control status\n\n")
	fmt.Fprintf(&b, "- Active policy version: **%s** (%d rules)\n", orDash(r.PolicyVersion), r.PolicyRuleCount)
	fmt.Fprintf(&b, "- Deterministic routing with no LLM in the decision: **yes** (architecture principle)\n")
	fmt.Fprintf(&b, "- Conservative mode (incident lever): %s\n", onOff(r.ConservativeMode))
	if r.BudgetCapUSD > 0 {
		fmt.Fprintf(&b, "- Budget cap: %.2f USD per tenant\n", r.BudgetCapUSD)
	} else {
		b.WriteString("- Budget cap: not set\n")
	}
	if r.AuditChain {
		b.WriteString("- Tamper-evident audit chain: **active** — export via `/router/audit/export`, verify offline with `audit-verify`\n")
	} else {
		b.WriteString("- Tamper-evident audit chain: not active (requires ROUTER_DATA_DIR)\n")
	}
	if len(r.OpenCircuits) > 0 {
		fmt.Fprintf(&b, "- ⚠️ Open circuits (provider excluded by the breaker): %s\n", strings.Join(r.OpenCircuits, ", "))
	}
	b.WriteString("\n### Providers (the egress surface)\n\n")
	if len(r.Providers) == 0 {
		b.WriteString("No providers registered.\n\n")
	} else {
		b.WriteString("| Provider | Compliance tags | Models | Health |\n|---|---|---|---|\n")
		for _, p := range r.Providers {
			tags := "—"
			if len(p.ComplianceTags) > 0 {
				tags = strings.Join(p.ComplianceTags, ", ")
			}
			healthStr := "—"
			if p.HasHealth {
				healthStr = fmt.Sprintf("%.0f%%", p.Health*100)
			}
			fmt.Fprintf(&b, "| %s | %s | %d | %s |\n", p.ID, tags, p.Models, healthStr)
		}
		b.WriteString("\nTags are operator attestations and drive the policy constraints `require_provider_tags`/`deny_provider_tags`.\n\n")
	}

	b.WriteString("## 2. Events\n\n")
	fmt.Fprintf(&b, "- Requests (totalt): **%d**\n", r.TotalRequests)
	fmt.Fprintf(&b, "- Blocked by policy before any provider call: **%d**\n", r.BlockedRequests)
	w := r.AuditWindow
	if w.Entries == 0 {
		b.WriteString("- Audit window (memory): no events yet\n\n")
	} else {
		fmt.Fprintf(&b, "- Audit window (last %d events in memory — the full chain is in the export):\n", w.Entries)
		for _, code := range sortedKeys(w.BlockedByCode) {
			fmt.Fprintf(&b, "  - blockerade `%s`: %d\n", code, w.BlockedByCode[code])
		}
		for _, t := range sortedKeys(w.PIITypeCounts) {
			fmt.Fprintf(&b, "  - PII hits `%s` (types, never values): %d\n", t, w.PIITypeCounts[t])
		}
		fmt.Fprintf(&b, "  - key events: %d · policy reloads: %d\n\n", w.KeyMutations, w.PolicyReloads)
	}

	b.WriteString("## 3. Per department (tenant)\n\n")
	if len(r.Tenants) == 0 {
		b.WriteString("No tenant data yet.\n\n")
	} else {
		b.WriteString("| Tenant | Requests | Kostnad (est.) |\n|---|---|---|\n")
		for _, t := range r.Tenants {
			fmt.Fprintf(&b, "| %s | %d | $%.6f |\n", t.TenantID, t.Requests, t.CostUSD)
		}
		b.WriteString("\n")
	}

	b.WriteString("## 4. Explainability and evidence\n\n")
	b.WriteString("- Every routing decision is deterministic (rules + scoring, no LLM) and carries\n  readable reasons (`decision_reasons`), including PII and residency exclusions.\n")
	b.WriteString("- Full traceability: export the hash-chained audit log and verify it offline —\n  any edited/removed/inserted record is detected.\n")
	b.WriteString("- No prompts or key values are stored in logs/audit (the masking principle).\n\n")

	b.WriteString("## 5. Cost and environment (estimates)\n\n")
	if r.Savings.PremiumBaselineUSD > 0 {
		fmt.Fprintf(&b, "- Paid: $%.6f · All-premium baseline: $%.6f → **saved $%.6f (%.1f%%)**\n",
			r.Savings.ActualUSD, r.Savings.PremiumBaselineUSD, r.Savings.SavedUSD, r.Savings.SavedPct)
	} else {
		b.WriteString("- No savings data yet (requires traffic + premium pricing).\n")
	}
	if r.Green.SavedWh > 0 {
		fmt.Fprintf(&b, "- Estimated energy saved: %.1f Wh (%.1f g CO₂e) vs all-premium\n", r.Green.SavedWh, r.Green.SavedCO2eGrams)
	}
	b.WriteString("\nThe baseline is the same token volume priced at the premium model — a deliberately\nconservative estimate, clearly labeled as such.\n")
	return b.String()
}

// ComplianceReportHandler serves the report as markdown (default) or JSON.
func ComplianceReportHandler(opts ReportOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := BuildComplianceReport(opts)
		if r.URL.Query().Get("format") == "json" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(report)
			return
		}
		md := RenderComplianceMarkdown(report)
		if reportWantsHTML(r) {
			title := report.ProfileTitle
			if title == "" {
				title = "Control report — LLM routing"
			}
			sub := "Active controls and observed events"
			if report.ProfileName != "" {
				sub += " · regime profile " + report.ProfileName + " · " + report.ProfileStatute
			}
			renderReportPage(w, reportPageData{
				Title:    title,
				Subtitle: sub,
				Stats: []reportStat{
					{Label: "Requests", Value: strconv.FormatInt(report.TotalRequests, 10), Sub: "retained history"},
					{Label: "Blocked by policy", Value: strconv.FormatInt(report.BlockedRequests, 10), Sub: "before any provider call", Tone: toneIf(report.BlockedRequests > 0, "bad")},
					{Label: "Active policy", Value: orDash(report.PolicyVersion), Sub: fmtInt(report.PolicyRuleCount) + " rules"},
					{Label: "Providers", Value: fmtInt(len(report.Providers)), Sub: "the egress surface"},
				},
				Body:        markdownToHTML(md),
				DownloadURL: "/router/compliance/report?format=md",
				JSONURL:     "/router/compliance/report?format=json",
			})
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="control-report.md"`)
		_, _ = w.Write([]byte(md))
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func onOff(b bool) string {
	if b {
		return "**on**"
	}
	return "av"
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
