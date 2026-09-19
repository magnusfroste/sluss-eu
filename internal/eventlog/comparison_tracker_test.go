package eventlog_test

import (
	"context"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
)

func TestComparisonTrackerAggregatesAndFiltersRecentRecords(t *testing.T) {
	tracker := eventlog.NewComparisonTracker(2)

	baseTime := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)
	tracker.Handle(context.Background(), eventlog.Event{
		Type: eventlog.EventTypeDecision,
		Decision: &eventlog.DecisionEvent{
			RequestID: "req_1",
			TaskType:  "simple_chat",
			RiskLevel: "low",
			DecidedAt: baseTime,
			ShadowComparison: &engine.DecisionComparison{
				Primary:                    engine.RouteDecision{SelectedModel: "balanced-coder"},
				Secondary:                  engine.RouteDecision{SelectedModel: "premium-reasoning"},
				Changed:                    true,
				RouteChanged:               true,
				CostChanged:                true,
				EstimatedCostDeltaMicroUSD: 33000,
				EstimatedCostDeltaUSD:      0.033,
			},
		},
	})
	tracker.Handle(context.Background(), eventlog.Event{
		Type: eventlog.EventTypeDecision,
		Decision: &eventlog.DecisionEvent{
			RequestID: "req_2",
			TaskType:  "security_review",
			RiskLevel: "high",
			DecidedAt: baseTime.Add(time.Minute),
			ShadowComparison: &engine.DecisionComparison{
				Primary:              engine.RouteDecision{SelectedModel: "premium-reasoning"},
				Secondary:            engine.RouteDecision{SelectedModel: "premium-reasoning"},
				PolicyVersionChanged: true,
			},
		},
	})
	tracker.Handle(context.Background(), eventlog.Event{
		Type: eventlog.EventTypeDecision,
		Decision: &eventlog.DecisionEvent{
			RequestID: "req_3",
			TaskType:  "simple_chat",
			RiskLevel: "low",
			DecidedAt: baseTime.Add(2 * time.Minute),
			ShadowComparison: &engine.DecisionComparison{
				Primary:                    engine.RouteDecision{SelectedModel: "balanced-coder"},
				Secondary:                  engine.RouteDecision{SelectedModel: "cheap-general"},
				Changed:                    true,
				RouteChanged:               true,
				FallbackChanged:            true,
				TimeoutChanged:             true,
				RequiresVerifierChanged:    true,
				EstimatedCostDeltaMicroUSD: -11000,
				EstimatedCostDeltaUSD:      -0.011,
			},
		},
	})

	summary := tracker.Summary()
	if summary.Total != 3 {
		t.Fatalf("total = %d, want 3", summary.Total)
	}
	if summary.ChangedCount != 2 {
		t.Fatalf("changed count = %d, want 2", summary.ChangedCount)
	}
	if summary.RouteChangedCount != 2 {
		t.Fatalf("route changed count = %d, want 2", summary.RouteChangedCount)
	}
	if summary.PolicyVersionChangedCount != 1 {
		t.Fatalf("policy version changed count = %d, want 1", summary.PolicyVersionChangedCount)
	}
	if summary.EstimatedCostDeltaMicroUSD != 22000 {
		t.Fatalf("delta micro USD = %d, want 22000", summary.EstimatedCostDeltaMicroUSD)
	}

	recent := tracker.Recent("")
	if len(recent) != 2 {
		t.Fatalf("recent len = %d, want 2", len(recent))
	}
	if recent[0].RequestID != "req_3" || recent[1].RequestID != "req_2" {
		t.Fatalf("recent order = %#v", recent)
	}
	filtered := tracker.Recent("simple_chat")
	if len(filtered) != 1 || filtered[0].RequestID != "req_3" {
		t.Fatalf("filtered recent = %#v", filtered)
	}
}

func TestComparisonTrackerIgnoresDecisionEventsWithoutShadowComparison(t *testing.T) {
	tracker := eventlog.NewComparisonTracker(5)
	tracker.Handle(context.Background(), eventlog.Event{
		Type: eventlog.EventTypeDecision,
		Decision: &eventlog.DecisionEvent{
			RequestID: "req_without_shadow",
		},
	})

	if summary := tracker.Summary(); summary.Total != 0 {
		t.Fatalf("summary total = %d, want 0", summary.Total)
	}
	if recent := tracker.Recent(""); len(recent) != 0 {
		t.Fatalf("recent len = %d, want 0", len(recent))
	}
}
