package server

import (
	"context"
	"testing"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/router"
)

func TestAuditBlockedRecordsEntry(t *testing.T) {
	mem := audit.NewMemorySink(0)
	cfg := &ChatOptions{Auditor: mem}

	model := "gpt-4o"
	job := &router.JobDescriptor{
		RequestID:     "req_blocked",
		TenantID:      "tn_1",
		ProjectID:     "prj_1",
		TaskType:      router.TaskSecurityReview,
		RiskLevel:     router.RiskHigh,
		ExplicitModel: &model,
	}
	dec := engine.RouteDecision{
		Blocked:     true,
		BlockCode:   "model_not_allowed",
		BlockReason: "policy forbids pinned model for high-risk task",
	}

	cfg.auditBlocked(context.Background(), job, dec)

	entries := mem.Entries()
	if len(entries) != 1 {
		t.Fatalf("want 1 audit entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Action != audit.ActionRequestBlocked {
		t.Errorf("action = %q, want %q", e.Action, audit.ActionRequestBlocked)
	}
	if e.Outcome != audit.OutcomeBlocked {
		t.Errorf("outcome = %q, want %q", e.Outcome, audit.OutcomeBlocked)
	}
	if e.RequestID != "req_blocked" || e.TenantID != "tn_1" || e.ProjectID != "prj_1" {
		t.Errorf("identifiers not propagated: %+v", e)
	}
	if e.Target != "gpt-4o" {
		t.Errorf("target = %q, want pinned model gpt-4o", e.Target)
	}
	if e.Detail["block_code"] != "model_not_allowed" {
		t.Errorf("detail block_code = %q", e.Detail["block_code"])
	}
}

func TestAuditBlockedNilAuditorIsNoop(t *testing.T) {
	cfg := &ChatOptions{}
	// Must not panic without an auditor configured.
	cfg.auditBlocked(context.Background(), &router.JobDescriptor{}, engine.RouteDecision{})
}
