package server

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/magnusfroste/sluss/internal/budget"
	"github.com/magnusfroste/sluss/internal/router"
)

func budgetCfg(t *testing.T, cap budget.Cap, spent int64) *ChatOptions {
	t.Helper()
	caps := budget.NewCaps()
	caps.SetTenant("tn", cap)
	ledger := budget.NewLedger()
	ledger.Add("tn", "", spent)
	return &ChatOptions{Budget: budget.NewEvaluator(caps, ledger)}
}

func TestApplyBudgetBlocks(t *testing.T) {
	cfg := budgetCfg(t, budget.Cap{LimitMicroUSD: 1000, Action: budget.ActionBlock}, 2000)
	job := &router.JobDescriptor{RequestID: "r1", TenantID: "tn", RouterMode: router.RouterModeAuto}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)

	if cont := cfg.applyBudget(rec, req.WithContext(context.Background()), job); cont {
		t.Fatal("over-budget block should stop the request (return false)")
	}
	if rec.Code != 402 {
		t.Errorf("status = %d, want 402", rec.Code)
	}
	// Router mode is untouched on a block.
	if job.RouterMode != router.RouterModeAuto {
		t.Errorf("router mode changed on block: %s", job.RouterMode)
	}
}

func TestApplyBudgetDowngrades(t *testing.T) {
	cfg := budgetCfg(t, budget.Cap{LimitMicroUSD: 1000, Action: budget.ActionDowngrade}, 2000)
	job := &router.JobDescriptor{RequestID: "r2", TenantID: "tn", RouterMode: router.RouterModeAuto}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)

	if cont := cfg.applyBudget(rec, req, job); !cont {
		t.Fatal("downgrade should let the request continue (return true)")
	}
	if job.RouterMode != router.RouterModeCheap {
		t.Errorf("router mode = %s, want cheap", job.RouterMode)
	}
	if rec.Header().Get("X-Router-Budget-Action") != "downgrade" {
		t.Errorf("missing downgrade header")
	}
}

func TestApplyBudgetWarns(t *testing.T) {
	cfg := budgetCfg(t, budget.Cap{LimitMicroUSD: 1000, WarnThreshold: 0.8}, 850)
	job := &router.JobDescriptor{RequestID: "r3", TenantID: "tn", RouterMode: router.RouterModeAuto}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)

	if cont := cfg.applyBudget(rec, req, job); !cont {
		t.Fatal("warning should not stop the request")
	}
	if rec.Header().Get("X-Router-Budget-Warning") != "true" {
		t.Errorf("missing budget warning header")
	}
	if job.RouterMode != router.RouterModeAuto {
		t.Errorf("warning must not downgrade, got %s", job.RouterMode)
	}
}

func TestApplyBudgetNilEvaluatorContinues(t *testing.T) {
	cfg := &ChatOptions{}
	job := &router.JobDescriptor{TenantID: "tn"}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	if cont := cfg.applyBudget(rec, req, job); !cont {
		t.Fatal("no budget configured should continue")
	}
}
