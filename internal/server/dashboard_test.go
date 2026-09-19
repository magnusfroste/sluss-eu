package server

import (
	"context"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
)

func TestBuildDashboardDataIncludesShadowComparisons(t *testing.T) {
	tracker := eventlog.NewComparisonTracker(10)
	tracker.Handle(context.Background(), eventlog.Event{
		Type: eventlog.EventTypeDecision,
		Decision: &eventlog.DecisionEvent{
			RequestID: "req_shadow_1",
			TaskType:  "simple_chat",
			RiskLevel: "low",
			DecidedAt: time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC),
			ShadowComparison: &engine.DecisionComparison{
				Primary:                    engine.RouteDecision{SelectedModel: "balanced-coder", PolicyVersion: "pv_a"},
				Secondary:                  engine.RouteDecision{SelectedModel: "premium-reasoning", PolicyVersion: "pv_b"},
				Changed:                    true,
				RouteChanged:               true,
				EstimatedCostDeltaMicroUSD: 12000,
				EstimatedCostDeltaUSD:      0.012,
			},
		},
	})
	tracker.Handle(context.Background(), eventlog.Event{
		Type: eventlog.EventTypeDecision,
		Decision: &eventlog.DecisionEvent{
			RequestID: "req_shadow_2",
			TaskType:  "security_review",
			RiskLevel: "high",
			DecidedAt: time.Date(2026, 6, 12, 10, 1, 0, 0, time.UTC),
			ShadowComparison: &engine.DecisionComparison{
				Primary:              engine.RouteDecision{SelectedModel: "premium-reasoning", PolicyVersion: "pv_a"},
				Secondary:            engine.RouteDecision{SelectedModel: "premium-reasoning", PolicyVersion: "pv_b"},
				PolicyVersionChanged: true,
			},
		},
	})

	data := buildDashboardData(DashboardOptions{Comparisons: tracker, Version: "registry_test"}, "simple_chat")
	if data.Version != "registry_test" {
		t.Fatalf("version = %q, want registry_test", data.Version)
	}
	if data.ShadowSummary.Total != 2 {
		t.Fatalf("shadow summary total = %d, want 2", data.ShadowSummary.Total)
	}
	if data.ShadowSummary.ChangedCount != 1 {
		t.Fatalf("shadow summary changed = %d, want 1", data.ShadowSummary.ChangedCount)
	}
	if len(data.ShadowRecent) != 1 {
		t.Fatalf("shadow recent len = %d, want 1", len(data.ShadowRecent))
	}
	if got := data.ShadowRecent[0].Comparison.Secondary.SelectedModel; got != "premium-reasoning" {
		t.Fatalf("shadow selected model = %q, want premium-reasoning", got)
	}
}
