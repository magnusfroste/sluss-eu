package budget

import (
	"context"
	"testing"

	"github.com/magnusfroste/sluss/internal/eventlog"
)

func TestCheckNoCapIsOK(t *testing.T) {
	e := NewEvaluator(NewCaps(), NewLedger())
	if v := e.Check("tn", "prj"); v.Status != StatusOK {
		t.Errorf("no cap should be OK, got %s", v.Status)
	}
}

func TestCheckNilEvaluatorIsOK(t *testing.T) {
	var e *Evaluator
	if v := e.Check("tn", "prj"); v.Status != StatusOK {
		t.Errorf("nil evaluator should be OK, got %s", v.Status)
	}
}

func TestCheckTenantWarnAndOver(t *testing.T) {
	caps := NewCaps()
	caps.SetTenant("tn", Cap{LimitMicroUSD: 1_000_000, WarnThreshold: 0.8, Action: ActionBlock})
	ledger := NewLedger()
	e := NewEvaluator(caps, ledger)

	// Under threshold → OK.
	ledger.Add("tn", "", 500_000)
	if v := e.Check("tn", ""); v.Status != StatusOK {
		t.Fatalf("50%% should be OK, got %s", v.Status)
	}

	// Cross the 80% warning threshold.
	ledger.Add("tn", "", 350_000) // 850_000 total
	v := e.Check("tn", "")
	if v.Status != StatusWarn {
		t.Fatalf("85%% should warn, got %s", v.Status)
	}

	// Exceed the limit → over + block.
	ledger.Add("tn", "", 300_000) // 1_150_000 total
	v = e.Check("tn", "")
	if v.Status != StatusOver || !v.Blocked() {
		t.Fatalf("over limit should block, got status=%s action=%s", v.Status, v.Action)
	}
}

func TestCheckDowngradeAction(t *testing.T) {
	caps := NewCaps()
	caps.SetTenant("tn", Cap{LimitMicroUSD: 100, Action: ActionDowngrade})
	ledger := NewLedger()
	ledger.Add("tn", "", 200)
	e := NewEvaluator(caps, ledger)

	v := e.Check("tn", "")
	if !v.Downgrade() || v.Blocked() {
		t.Fatalf("expected downgrade, got status=%s action=%s", v.Status, v.Action)
	}
}

func TestProjectCapTakesPrecedence(t *testing.T) {
	caps := NewCaps()
	caps.SetTenant("tn", Cap{LimitMicroUSD: 1_000_000})       // generous tenant cap
	caps.SetProject("tn", "prj", Cap{LimitMicroUSD: 100_000}) // strict project cap
	ledger := NewLedger()
	ledger.Add("tn", "prj", 150_000) // also rolls into tenant total

	e := NewEvaluator(caps, ledger)
	v := e.Check("tn", "prj")
	if v.Status != StatusOver {
		t.Fatalf("project cap should be over, got %s (spent=%d limit=%d)", v.Status, v.SpentMicroUSD, v.LimitMicroUSD)
	}
	if v.LimitMicroUSD != 100_000 {
		t.Errorf("should use project limit, got %d", v.LimitMicroUSD)
	}
	// Tenant-only check (no project) stays under the generous tenant cap.
	if tv := e.Check("tn", ""); tv.Status != StatusOK {
		t.Errorf("tenant scope should be OK, got %s", tv.Status)
	}
}

func TestDefaultWarnThresholdAndAction(t *testing.T) {
	caps := NewCaps()
	caps.SetTenant("tn", Cap{LimitMicroUSD: 1000}) // no warn threshold, no action
	ledger := NewLedger()
	ledger.Add("tn", "", 850) // 85% → default 0.8 warns
	e := NewEvaluator(caps, ledger)
	if v := e.Check("tn", ""); v.Status != StatusWarn {
		t.Errorf("default warn threshold should warn at 85%%, got %s", v.Status)
	}
	ledger.Add("tn", "", 200) // over
	if v := e.Check("tn", ""); !v.Blocked() {
		t.Errorf("default action should be block, got %s/%s", v.Status, v.Action)
	}
}

func TestLedgerHandleAccruesFromDecisionEvents(t *testing.T) {
	ledger := NewLedger()
	ledger.Handle(context.Background(), eventlog.Event{
		Type: eventlog.EventTypeDecision,
		Decision: &eventlog.DecisionEvent{
			TenantID: "tn", ProjectID: "prj", EstimatedCostUSD: 0.5,
		},
	})
	// Blocked decisions and attempt events do not accrue.
	ledger.Handle(context.Background(), eventlog.Event{
		Type:     eventlog.EventTypeDecision,
		Decision: &eventlog.DecisionEvent{TenantID: "tn", EstimatedCostUSD: 9, Blocked: true},
	})
	ledger.Handle(context.Background(), eventlog.Event{
		Type:    eventlog.EventTypeAttempt,
		Attempt: &eventlog.AttemptEvent{Success: true, ActualCostUSD: 9},
	})

	if got := ledger.SpentTenant("tn"); got != 500_000 {
		t.Errorf("tenant spend = %d micro-USD, want 500000", got)
	}
	if got := ledger.SpentProject("tn", "prj"); got != 500_000 {
		t.Errorf("project spend = %d micro-USD, want 500000", got)
	}
}
