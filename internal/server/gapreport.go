package server

// Shadow-AI gap report (ISSUE-095) — the CISO hook. Run the router in MONITOR
// MODE: the live policy stays permissive while a compliance pack (e.g.
// builtin:nis2-baseline) runs as the SHADOW policy. Nothing is blocked, nothing
// changes for users — but every sensitive prompt that went to the cloud when
// the pack would have kept it local (or blocked it) is counted. This report
// turns those counters into the number a CISO forwards to leadership:
// "N prompts with personal data went to a cloud provider; under the NIS2 pack
// 100% would have stayed in the house." Conversion is one env var: move the
// pack from ROUTER_SHADOW_POLICY_PATH to ROUTER_POLICY_PATH.
//
// Honesty rules: counters are exact since the last restart (in-memory), no
// prompt text is ever stored or shown, and the report says so.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/regime"
)

// GapReportOptions carries the sources for the gap report.
type GapReportOptions struct {
	Comparisons *eventlog.ComparisonTracker
	Engine      *engine.Engine // model → egress (local/cloud) for examples
	ShadowCache *policy.Cache  // nil/empty → monitor mode is OFF
	Profile     string         // explicit regime profile (ROUTER_PROFILE)
	Now         func() time.Time
}

type gapExample struct {
	Task        string `json:"task"`
	Sensitivity string `json:"sensitivity"`
	LiveModel   string `json:"live_model"`
	LiveEgress  string `json:"live_egress"`
	ShadowModel string `json:"shadow_model,omitempty"`
	ShadowWould string `json:"shadow_would"` // "local" | "cloud" | "blocked"
}

// GapReport is the JSON shape (?format=json).
type GapReport struct {
	GeneratedAt   time.Time           `json:"generated_at"`
	MonitorMode   bool                `json:"monitor_mode"`
	ShadowVersion string              `json:"shadow_policy_version,omitempty"`
	Profile       string              `json:"profile"`
	Gap           eventlog.GapSummary `json:"gap"`
	TotalCompared int64               `json:"total_compared"`
	Examples      []gapExample        `json:"examples"`
	Note          string              `json:"note"`
}

// BuildGapReport assembles the report from the comparison tracker.
func BuildGapReport(o GapReportOptions) GapReport {
	now := time.Now
	if o.Now != nil {
		now = o.Now
	}
	r := GapReport{
		GeneratedAt: now().UTC(),
		Note:        "Counters are exact since the last restart (in-memory). No prompt text is stored or shown — only classification and routing outcomes.",
	}
	if o.ShadowCache != nil {
		if p, ok := o.ShadowCache.Active(policy.Scope{}); ok {
			r.MonitorMode = true
			r.ShadowVersion = p.Version()
		}
	}
	r.Profile = regime.Resolve(o.Profile, r.ShadowVersion).Key
	if o.Comparisons != nil {
		r.Gap = o.Comparisons.Gap()
		r.TotalCompared = o.Comparisons.Summary().Total
		// Examples: sensitive records where the shadow pack would have acted.
		for _, rec := range o.Comparisons.Recent("") {
			if rec.Sensitivity != "pii" && rec.Sensitivity != "secrets_possible" {
				continue
			}
			c := rec.Comparison
			if !c.Secondary.Blocked && !c.RouteChanged {
				continue
			}
			ex := gapExample{
				Task:        rec.TaskType,
				Sensitivity: rec.Sensitivity,
				LiveModel:   c.Primary.SelectedModel,
			}
			ex.LiveEgress, _ = modelEgress(o.Engine, c.Primary.SelectedModel)
			if c.Secondary.Blocked {
				ex.ShadowWould = "blocked"
			} else {
				ex.ShadowModel = c.Secondary.SelectedModel
				ex.ShadowWould, _ = modelEgress(o.Engine, c.Secondary.SelectedModel)
			}
			r.Examples = append(r.Examples, ex)
			if len(r.Examples) >= 10 {
				break
			}
		}
	}
	return r
}

// RenderGapMarkdown renders the report as the shareable markdown document.
func RenderGapMarkdown(r GapReport) string {
	var b strings.Builder
	b.WriteString("# Shadow AI — gap report\n\n")
	fmt.Fprintf(&b, "Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339))
	b.WriteString("> Monitor mode: nothing is blocked and nothing changes for users — this report\n")
	b.WriteString("> shows what the compliance pack WOULD have done. No prompt text is stored.\n\n")

	if !r.MonitorMode {
		b.WriteString("## Monitor mode is OFF\n\n")
		b.WriteString("Enable it by running a compliance pack as the shadow policy — nothing changes in production:\n\n")
		b.WriteString("```\nROUTER_SHADOW_POLICY_PATH=builtin:nis2-baseline\n```\n\n")
		b.WriteString("After e.g. 14 days this report shows how many sensitive prompts went to the cloud\nand what the pack would have done instead. Converting to enforcement is one setting:\nmove the pack to `ROUTER_POLICY_PATH`.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "Shadow policy (the pack under evaluation): **%s**\n\n", r.ShadowVersion)
	b.WriteString("## The numbers\n\n")
	fmt.Fprintf(&b, "- Requests compared since restart: **%d**\n", r.TotalCompared)
	fmt.Fprintf(&b, "- Sensitive prompts (personal data / possible secrets): **%d** (pii: %d, secrets: %d)\n",
		r.Gap.SensitiveTotal, r.Gap.SensitivePII, r.Gap.SensitiveSecrets)
	fmt.Fprintf(&b, "- …where the pack would have **re-routed** (e.g. cloud → local): **%d**\n", r.Gap.SensitiveRouteChanged)
	fmt.Fprintf(&b, "- …where the pack would have **blocked** (fail-closed): **%d**\n", r.Gap.SensitiveShadowBlocked)
	acted := r.Gap.SensitiveRouteChanged + r.Gap.SensitiveShadowBlocked
	if r.Gap.SensitiveTotal > 0 {
		fmt.Fprintf(&b, "\n**%d of %d sensitive prompts (%.0f%%) would have been handled differently under the pack.**\n",
			acted, r.Gap.SensitiveTotal, 100*float64(acted)/float64(r.Gap.SensitiveTotal))
	} else {
		b.WriteString("\nNo sensitive prompts observed yet — leave monitor mode on and check back.\n")
	}

	if len(r.Examples) > 0 {
		b.WriteString("\n## Examples (type and outcome — never content)\n\n")
		b.WriteString("| Task | Sensitivity | Today (live) | Under the pack |\n|---|---|---|---|\n")
		for _, ex := range r.Examples {
			today := fmt.Sprintf("%s (%s)", ex.LiveModel, ex.LiveEgress)
			would := "**BLOCKED** (fail-closed)"
			if ex.ShadowWould != "blocked" {
				would = fmt.Sprintf("%s (%s)", ex.ShadowModel, ex.ShadowWould)
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", ex.Task, ex.Sensitivity, today, would)
		}
	}

	b.WriteString("\n## Next step\n\n")
	b.WriteString("Convert monitor mode to enforcement with one setting: move the pack from\n`ROUTER_SHADOW_POLICY_PATH` to `ROUTER_POLICY_PATH` and redeploy. Same rules,\nnow enforced and audited.\n\n")
	b.WriteString("> This is a control report, not a legal attestation of regulatory compliance.\n")
	return b.String()
}

// GapReportHandler serves the report (markdown default, ?format=json).
func GapReportHandler(o GapReportOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rep := BuildGapReport(o)
		if r.URL.Query().Get("format") == "json" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rep)
			return
		}
		md := RenderGapMarkdown(rep)
		if reportWantsHTML(r) {
			d := reportPageData{
				Title:       "Shadow AI — gap report",
				Subtitle:    "Monitor mode: nothing is blocked and nothing changes for users — this shows what the compliance pack would have done. No prompt text is stored.",
				Body:        markdownToHTML(md),
				DownloadURL: "/router/gap-report?format=md",
				JSONURL:     "/router/gap-report?format=json",
			}
			if rep.MonitorMode {
				acted := rep.Gap.SensitiveRouteChanged + rep.Gap.SensitiveShadowBlocked
				d.Stats = []reportStat{
					{Label: "Requests compared", Value: fmtInt(int(rep.TotalCompared)), Sub: "since restart"},
					{Label: "Sensitive prompts", Value: fmtInt(int(rep.Gap.SensitiveTotal))},
					{Label: "Would re-route", Value: fmtInt(int(rep.Gap.SensitiveRouteChanged)), Tone: toneIf(rep.Gap.SensitiveRouteChanged > 0, "warn")},
					{Label: "Would block", Value: fmtInt(int(rep.Gap.SensitiveShadowBlocked)), Tone: toneIf(rep.Gap.SensitiveShadowBlocked > 0, "bad")},
					{Label: "Handled differently", Value: fmtInt(int(acted)), Sub: "under the pack"},
				}
			}
			renderReportPage(w, d)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte(md))
	}
}
