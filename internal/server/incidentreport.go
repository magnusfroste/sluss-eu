package server

// Incident evidence pack (ISSUE-095 follow-up): NIS2 gives an essential entity
// 24 hours for an early warning and 72 hours for the incident notification.
// When something happens — a burst of blocked prompts, a suspected exfiltration
// path, an auditor's question — the CISO needs the LLM-traffic evidence for the
// window NOW, not after a log-spelunking session. This report assembles it from
// the durable request log: what was blocked (codes, classifications, tenants),
// where sensitive prompts actually went (local vs cloud), under which policy
// versions — classifications and counts only, never prompt content. It is
// evidence for a report, not a legal attestation.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/regime"
)

// incidentWindows maps the accepted ?window= values to durations. 24h and 72h
// mirror the NIS2 reporting deadlines; 7d covers the follow-up report.
var incidentWindows = map[string]time.Duration{
	"24h": 24 * time.Hour,
	"72h": 72 * time.Hour,
	"7d":  7 * 24 * time.Hour,
}

// IncidentReportOptions wires the report to its sources.
type IncidentReportOptions struct {
	History *history.Store
	Engine  *engine.Engine
	Cache   *policy.Cache
	Profile string // explicit regime profile; "" derives from the policy version
	// MaxRows bounds the window query (default 10k); if hit, the report says so.
	MaxRows int
}

type countRow struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// IncidentReport is the JSON shape (markdown renders from the same data).
type IncidentReport struct {
	GeneratedAt   time.Time `json:"generated_at"`
	Window        string    `json:"window"`
	WindowFrom    time.Time `json:"window_from"`
	Profile       string    `json:"profile"`
	ProfileName   string    `json:"profile_name"`
	PolicyVersion string    `json:"policy_version"` // active now; per-row versions live in the audit chain

	Total          int        `json:"total"`
	Blocked        int        `json:"blocked"`
	BlockedByCode  []countRow `json:"blocked_by_code"`
	BlockedByClass []countRow `json:"blocked_by_sensitivity"`
	BlockedByTask  []countRow `json:"blocked_by_task"`

	SensitiveTotal   int        `json:"sensitive_total"`
	SensitiveByClass []countRow `json:"sensitive_by_class"`
	SensitiveLocal   int        `json:"sensitive_local"`
	SensitiveCloud   int        `json:"sensitive_cloud"`
	CloudModels      []countRow `json:"sensitive_cloud_models"` // where sensitive traffic left the house

	Truncated bool `json:"truncated,omitempty"`
}

func topCounts(m map[string]int, n int) []countRow {
	out := make([]countRow, 0, len(m))
	for k, v := range m {
		out = append(out, countRow{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// BuildIncidentReport aggregates the window; "sensitive" matches the gap
// report's class set (see eventlogSensitive below).
func BuildIncidentReport(o IncidentReportOptions, window string, now time.Time) IncidentReport {
	dur, ok := incidentWindows[window]
	if !ok {
		window, dur = "72h", incidentWindows["72h"]
	}
	rep := IncidentReport{GeneratedAt: now.UTC(), Window: window, WindowFrom: now.Add(-dur).UTC()}
	if o.Cache != nil {
		if p, active := o.Cache.Active(policy.Scope{}); active {
			rep.PolicyVersion = p.Version()
		}
	}
	prof := regime.Resolve(o.Profile, rep.PolicyVersion)
	rep.Profile, rep.ProfileName = prof.Key, prof.Name

	maxRows := o.MaxRows
	if maxRows <= 0 {
		maxRows = 10_000
	}
	rows := o.History.Since(rep.WindowFrom, maxRows)
	rep.Truncated = len(rows) == maxRows

	byCode, byClass, byTask := map[string]int{}, map[string]int{}, map[string]int{}
	sensByClass, cloudModels := map[string]int{}, map[string]int{}
	for _, r := range rows {
		rep.Total++
		if r.Blocked {
			rep.Blocked++
			byCode[firstNonEmpty(r.BlockCode, "policy_block")]++
			byClass[firstNonEmpty(r.Sensitivity, "none")]++
			byTask[firstNonEmpty(r.TaskType, "unknown")]++
		}
		if eventlogSensitive(r.Sensitivity) {
			rep.SensitiveTotal++
			sensByClass[r.Sensitivity]++
			if !r.Blocked {
				egress, _ := modelEgress(o.Engine, r.Model)
				if egress == "local" {
					rep.SensitiveLocal++
				} else {
					rep.SensitiveCloud++
					cloudModels[firstNonEmpty(r.Model, "?")]++
				}
			}
		}
	}
	rep.BlockedByCode = topCounts(byCode, 10)
	rep.BlockedByClass = topCounts(byClass, 10)
	rep.BlockedByTask = topCounts(byTask, 10)
	rep.SensitiveByClass = topCounts(sensByClass, 10)
	rep.CloudModels = topCounts(cloudModels, 10)
	return rep
}

// eventlogSensitive mirrors eventlog's sensitive-class set without exporting it.
func eventlogSensitive(s string) bool {
	switch s {
	case "pii", "secrets_possible", "financial", "health", "legal", "security_classified":
		return true
	}
	return false
}

// RenderIncidentMarkdown renders the evidence pack as readable markdown.
func RenderIncidentMarkdown(r IncidentReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Incident evidence — LLM routing (last %s)\n\n", r.Window)
	fmt.Fprintf(&b, "Generated %s · window from %s · profile: %s · active policy: `%s`\n\n",
		r.GeneratedAt.Format(time.RFC3339), r.WindowFrom.Format(time.RFC3339), r.ProfileName, firstNonEmpty(r.PolicyVersion, "built-in default"))
	b.WriteString("Supports the incident-reporting timeline (NIS2: early warning within 24h, incident notification within 72h). ")
	b.WriteString("Counts and classifications only — **never prompt content**. Evidence for a report, **not a legal attestation**.\n\n")

	fmt.Fprintf(&b, "## Traffic in the window\n\n- Requests: **%d**\n- Blocked (fail-closed policy): **%d**\n- Sensitive prompts: **%d** — routed local: **%d**, to cloud: **%d**\n\n",
		r.Total, r.Blocked, r.SensitiveTotal, r.SensitiveLocal, r.SensitiveCloud)
	if r.Truncated {
		b.WriteString("> ⚠ The window query hit its row cap — counts are a lower bound. Narrow the window.\n\n")
	}

	section := func(title string, rows []countRow) {
		if len(rows) == 0 {
			return
		}
		fmt.Fprintf(&b, "## %s\n\n", title)
		for _, c := range rows {
			fmt.Fprintf(&b, "- `%s` — %d\n", c.Key, c.Count)
		}
		b.WriteString("\n")
	}
	section("Blocks by code", r.BlockedByCode)
	section("Blocks by data classification", r.BlockedByClass)
	section("Blocks by task type", r.BlockedByTask)
	section("Sensitive prompts by class", r.SensitiveByClass)
	section("Sensitive traffic that reached the cloud (by model)", r.CloudModels)

	b.WriteString("## Evidence pointers\n\n")
	b.WriteString("- Tamper-evident audit chain (who/what/when, hash-chained): `GET /router/audit/export` — verify offline with `audit-verify`.\n")
	b.WriteString("- Full request log: `/router/log`. Per-request lookup: MCP `explain_request` with the x-router-request-id.\n")
	b.WriteString("- Policy versions per decision are recorded on each audit/log row; the version above is the one active now.\n")
	return b.String()
}

// IncidentReportHandler serves GET /router/incident-report?window=24h|72h|7d
// (&format=json). Markdown by default — paste-ready for the incident channel.
func IncidentReportHandler(o IncidentReportOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if o.History == nil {
			http.Error(w, "incident report needs a data dir (ROUTER_DATA_DIR)", http.StatusServiceUnavailable)
			return
		}
		rep := BuildIncidentReport(o, strings.TrimSpace(r.URL.Query().Get("window")), time.Now())
		if r.URL.Query().Get("format") == "json" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rep)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte(RenderIncidentMarkdown(rep)))
	}
}
