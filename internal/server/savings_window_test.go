package server

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/history"
)

// ISSUE-110: savings_report must be able to answer "what did the last 24h
// cost/save?", not only all-time — otherwise a daily brief reports a
// cumulative number that never changes shape.
func TestSavingsReportWindow(t *testing.T) {
	store, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// A row is a decision (routing) plus the attempt that carries token usage
	// and realized cost — the same pair the live path emits.
	add := func(id string, at time.Time, model string, in, out int, cost float64) {
		ctx := context.Background()
		store.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
			RequestID: id, TenantID: "tn", TaskType: "summarization", RiskLevel: "low",
			SelectedModel: model, SelectedProvider: "p", DecidedAt: at,
		}})
		store.Handle(ctx, eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
			RequestID: id, TenantID: "tn", ProviderID: "p", ModelID: model, Success: true,
			InputTokens: in, OutputTokens: out, ActualCostUSD: cost, AttemptedAt: at,
		}})
	}
	now := time.Now()
	add("recent1", now.Add(-1*time.Hour), "cheap", 1_000_000, 1_000_000, 0.10)
	add("recent2", now.Add(-2*time.Hour), "cheap", 1_000_000, 1_000_000, 0.10)
	add("old", now.Add(-100*time.Hour), "cheap", 8_000_000, 8_000_000, 5.00) // outside 24h

	// Premium baseline: $3/Mtok in, $15/Mtok out.
	opts := DashboardOptions{History: store,
		PremiumInputMicrosPerMTok: 3_000_000, PremiumOutputMicrosPerMTok: 15_000_000,
		PremiumBaselineModel: "openai/gpt-4o"}
	mo := MCPOptions{Dashboard: opts}

	// All-time includes the old row.
	all, err := mo.savingsReport(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	allMap := all.(map[string]any)
	if allMap["window"] != "all" {
		t.Fatalf("default window should be all, got %v", allMap["window"])
	}
	if got := allMap["total_requests"]; got != int64(3) {
		t.Fatalf("all-time requests = %v, want 3", got)
	}

	// 24h window drops it.
	day, err := mo.savingsReport(context.Background(), json.RawMessage(`{"window":"24h"}`))
	if err != nil {
		t.Fatalf("24h: %v", err)
	}
	d := day.(map[string]any)
	if d["window"] != "24h" {
		t.Fatalf("window = %v, want 24h", d["window"])
	}
	if got := d["total_requests"]; got != int64(2) {
		t.Fatalf("24h requests = %v, want 2 (old row must be excluded)", got)
	}
	cost := d["total_cost_usd"].(float64)
	if cost < 0.19 || cost > 0.21 {
		t.Fatalf("24h cost = %v, want ~0.20", cost)
	}
	sav := d["savings"].(SavingsSummary)
	// 2 rows x (1M in + 1M out) = $6 + $30 = $36 baseline.
	if sav.PremiumBaselineUSD < 35.9 || sav.PremiumBaselineUSD > 36.1 {
		t.Fatalf("24h baseline = %v, want ~36", sav.PremiumBaselineUSD)
	}
	if sav.BaselineModel != "openai/gpt-4o" {
		t.Fatalf("baseline model = %q", sav.BaselineModel)
	}
	if d["window_from"] == nil {
		t.Fatal("windowed report should state window_from")
	}

	// Unknown window falls back to all-time rather than a wrong number.
	bad, _ := mo.savingsReport(context.Background(), json.RawMessage(`{"window":"banana"}`))
	if bad.(map[string]any)["window"] != "all" {
		t.Fatalf("unknown window should fall back to all")
	}
}
