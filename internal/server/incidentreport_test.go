package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
)

// seedIncidentHistory writes a mix of decisions into the durable log:
// inside/outside the window, blocked/routed, sensitive/plain.
func seedIncidentHistory(t *testing.T, now time.Time) *history.Store {
	t.Helper()
	store, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	add := func(id string, at time.Time, sensitivity, model, blockCode string, blocked bool) {
		store.Handle(context.Background(), eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
			RequestID: id, TenantID: "tn", TaskType: "summarization", RiskLevel: "medium",
			Sensitivity: sensitivity, SelectedModel: model, SelectedProvider: "p",
			Blocked: blocked, BlockCode: blockCode, DecidedAt: at,
		}})
	}
	in := now.Add(-2 * time.Hour)
	add("r1", in, "health", "", "console_rule_block", true) // blocked sensitive
	add("r2", in, "financial", "", "residency_no_compliant_provider", true)
	add("r3", in, "pii", "local-model", "", false)                   // sensitive → local
	add("r4", in, "legal", "cloud-model", "", false)                 // sensitive → cloud
	add("r5", in, "none", "cloud-model", "", false)                  // plain
	add("r6", now.Add(-100*time.Hour), "pii", "", "old_block", true) // outside 72h
	return store
}

func TestIncidentReportWindowAggregation(t *testing.T) {
	now := time.Unix(1_752_000_000, 0)
	store := seedIncidentHistory(t, now)
	o := IncidentReportOptions{History: store, Engine: localRegistryStore(t)}

	rep := BuildIncidentReport(o, "72h", now)
	if rep.Total != 5 || rep.Blocked != 2 {
		t.Fatalf("window totals wrong: %+v", rep)
	}
	if rep.SensitiveTotal != 4 || rep.SensitiveLocal != 1 || rep.SensitiveCloud != 1 {
		t.Fatalf("sensitive split wrong: total=%d local=%d cloud=%d",
			rep.SensitiveTotal, rep.SensitiveLocal, rep.SensitiveCloud)
	}
	codes := map[string]int{}
	for _, c := range rep.BlockedByCode {
		codes[c.Key] = c.Count
	}
	if codes["console_rule_block"] != 1 || codes["residency_no_compliant_provider"] != 1 {
		t.Fatalf("block codes wrong: %+v", rep.BlockedByCode)
	}
	if codes["old_block"] != 0 {
		t.Fatal("rows outside the window must not count")
	}
	// The cloud leak names the model.
	if len(rep.CloudModels) != 1 || rep.CloudModels[0].Key != "cloud-model" {
		t.Fatalf("cloud models wrong: %+v", rep.CloudModels)
	}

	// Unknown window falls back to 72h.
	if got := BuildIncidentReport(o, "banana", now); got.Window != "72h" {
		t.Fatalf("unknown window should default to 72h, got %q", got.Window)
	}
}

func TestIncidentReportMarkdownAndHandler(t *testing.T) {
	now := time.Now()
	store := seedIncidentHistory(t, now)
	o := IncidentReportOptions{History: store, Engine: localRegistryStore(t)}

	md := RenderIncidentMarkdown(BuildIncidentReport(o, "24h", now))
	for _, want := range []string{
		"Incident evidence", "last 24h", "early warning within 24h",
		"never prompt content", "not a legal attestation",
		"console_rule_block", "/router/audit/export",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}

	rec := httptest.NewRecorder()
	IncidentReportHandler(o)(rec, httptest.NewRequest(http.MethodGet, "/router/incident-report?window=72h&format=json", nil))
	body := rec.Body.String()
	for _, want := range []string{`"blocked":2`, `"sensitive_total":4`, `"window":"72h"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("json missing %q: %s", want, body)
		}
	}

	// No history configured → 503, not a crash.
	nr := httptest.NewRecorder()
	IncidentReportHandler(IncidentReportOptions{})(nr, httptest.NewRequest(http.MethodGet, "/router/incident-report", nil))
	if nr.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil history should 503, got %d", nr.Code)
	}
}

// The MCP surface exposes the same evidence to agents.
func TestIncidentReportMCPTool(t *testing.T) {
	now := time.Now()
	store := seedIncidentHistory(t, now)
	o := MCPOptions{Engine: localRegistryStore(t), History: store}
	m := callTool(t, o, "incident_report", map[string]any{"window": "72h"})
	mdText, _ := m["markdown"].(string)
	if !strings.Contains(mdText, "Incident evidence") {
		t.Fatalf("mcp markdown missing header: %+v", m)
	}
	repMap, _ := m["report"].(map[string]any)
	if blocked, _ := repMap["blocked"].(float64); blocked != 2 {
		t.Fatalf("mcp report blocked = %v, want 2", repMap["blocked"])
	}
}
