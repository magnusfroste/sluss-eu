package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
)

// shadowEvent builds a decision event with a shadow comparison for the tracker.
func shadowEvent(sensitivity, primaryModel, shadowModel string, shadowBlocked, routeChanged bool) eventlog.Event {
	return eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
		RequestID: "r-" + sensitivity + primaryModel, TaskType: "summarization",
		Sensitivity: sensitivity, DecidedAt: time.Unix(1_700_000_000, 0),
		ShadowComparison: &engine.DecisionComparison{
			Primary:      engine.RouteDecision{SelectedModel: primaryModel},
			Secondary:    engine.RouteDecision{SelectedModel: shadowModel, Blocked: shadowBlocked},
			Changed:      routeChanged || shadowBlocked,
			RouteChanged: routeChanged,
		},
	}}
}

// The gap counters count only sensitive prompts, split re-route vs would-block.
func TestGapCountersTrackSensitiveShadowActions(t *testing.T) {
	tr := eventlog.NewComparisonTracker(10)
	ctx := context.Background()
	tr.Handle(ctx, shadowEvent("pii", "cheap-cloud", "qwen-local", false, true))    // re-routed
	tr.Handle(ctx, shadowEvent("secrets_possible", "cheap-cloud", "", true, false)) // blocked
	tr.Handle(ctx, shadowEvent("none", "cheap-cloud", "other", false, true))        // not sensitive → ignored
	tr.Handle(ctx, shadowEvent("pii", "qwen-local", "qwen-local", false, false))    // sensitive, no change

	g := tr.Gap()
	if g.SensitiveTotal != 3 || g.SensitivePII != 2 || g.SensitiveSecrets != 1 {
		t.Fatalf("sensitive counts wrong: %+v", g)
	}
	if g.SensitiveRouteChanged != 1 || g.SensitiveShadowBlocked != 1 {
		t.Fatalf("action counts wrong: %+v", g)
	}
}

// Monitor mode ON: the markdown carries the headline numbers and examples;
// monitor mode OFF: it carries the enable recipe instead.
func TestGapReportMarkdown(t *testing.T) {
	tr := eventlog.NewComparisonTracker(10)
	ctx := context.Background()
	tr.Handle(ctx, shadowEvent("pii", "cheap-general", "qwen", false, true))
	tr.Handle(ctx, shadowEvent("secrets_possible", "cheap-general", "", true, false))

	// OFF (no shadow cache): recipe.
	off := RenderGapMarkdown(BuildGapReport(GapReportOptions{Comparisons: tr}))
	if !strings.Contains(off, "ROUTER_SHADOW_POLICY_PATH=builtin:nis2-baseline") {
		t.Fatalf("off-state should show the enable recipe:\n%s", off)
	}

	// ON: fake monitor mode by reporting with a shadow version via JSON build.
	rep := BuildGapReport(GapReportOptions{Comparisons: tr, Engine: policyEngine(t)})
	rep.MonitorMode = true
	rep.ShadowVersion = "pv_nis2_baseline_2026_01"
	md := RenderGapMarkdown(rep)
	for _, want := range []string{
		"Sensitive prompts", "**2**", "re-routed", "blocked",
		"2 of 2 sensitive prompts (100%)",
		"never content", "not a legal attestation",
		"ROUTER_POLICY_PATH", // conversion step
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestGapReportHandlerJSON(t *testing.T) {
	tr := eventlog.NewComparisonTracker(10)
	tr.Handle(context.Background(), shadowEvent("pii", "cheap-general", "qwen", false, true))
	h := GapReportHandler(GapReportOptions{Comparisons: tr, Engine: policyEngine(t)})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/router/gap-report?format=json", nil))
	body := rec.Body.String()
	for _, want := range []string{`"sensitive_total":1`, `"sensitive_route_changed":1`, `"monitor_mode":false`} {
		if !strings.Contains(body, want) {
			t.Fatalf("json missing %q: %s", want, body)
		}
	}
}
