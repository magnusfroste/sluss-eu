package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/spend"
)

func reportFixture(t *testing.T) ReportOptions {
	t.Helper()
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	mem := audit.NewMemorySink(0)
	audit.Record(context.Background(), mem, audit.Entry{
		Action: audit.ActionRequestBlocked, Actor: "tn_eko",
		Detail: map[string]string{"block_code": "provider_not_allowed", "pii_types": "personnummer"},
	})
	audit.Record(context.Background(), mem, audit.Entry{
		Action: audit.ActionRequestBlocked, Actor: "tn_eko",
		Detail: map[string]string{"block_code": "provider_not_allowed"},
	})
	audit.Record(context.Background(), mem, audit.Entry{Action: audit.ActionAPIKeyAdd, Target: "key_1"})

	sp := spend.New()
	sp.Handle(context.Background(), eventlog.Event{Type: eventlog.EventTypeDecision, Decision: &eventlog.DecisionEvent{
		TenantID: "tn_eko", SelectedModel: "cheap-general", SelectedProvider: "openai",
	}})
	sp.Handle(context.Background(), eventlog.Event{Type: eventlog.EventTypeAttempt, Attempt: &eventlog.AttemptEvent{
		TenantID: "tn_eko", ModelID: "cheap-general", ProviderID: "openai",
		Success: true, InputTokens: 1000, OutputTokens: 200, ActualCostUSD: 0.001,
	}})

	return ReportOptions{
		Dashboard: DashboardOptions{
			Spend:                      sp,
			PremiumInputMicrosPerMTok:  15_000_000,
			PremiumOutputMicrosPerMTok: 75_000_000,
		},
		Engine:           engine.New(store),
		AuditMemory:      mem,
		ConservativeMode: true,
		BudgetUSD:        50,
		AuditChainPath:   "/data/audit-chain.jsonl",
		Now:              func() time.Time { return time.Unix(1_700_000_000, 0) },
	}
}

func TestBuildComplianceReportStructure(t *testing.T) {
	r := BuildComplianceReport(reportFixture(t))

	if !r.AuditChain || !r.ConservativeMode || r.BudgetCapUSD != 50 {
		t.Fatalf("control facts wrong: %+v", r)
	}
	if r.RegistryVersion == "" || len(r.Providers) == 0 {
		t.Fatalf("registry inventory missing: %+v", r)
	}
	if r.TotalRequests != 1 || len(r.Tenants) != 1 || r.Tenants[0].TenantID != "tn_eko" {
		t.Fatalf("per-tenant data wrong: total=%d tenants=%+v", r.TotalRequests, r.Tenants)
	}
	w := r.AuditWindow
	if w.BlockedByCode["provider_not_allowed"] != 2 {
		t.Fatalf("blocked-by-code = %v, want provider_not_allowed=2", w.BlockedByCode)
	}
	if w.PIITypeCounts["personnummer"] != 1 {
		t.Fatalf("pii counts = %v, want personnummer=1", w.PIITypeCounts)
	}
	if w.KeyMutations != 1 {
		t.Fatalf("key mutations = %d, want 1", w.KeyMutations)
	}
	if r.Savings.PremiumBaselineUSD <= 0 {
		t.Fatalf("savings baseline missing: %+v", r.Savings)
	}
}

func TestRenderComplianceMarkdownSections(t *testing.T) {
	md := RenderComplianceMarkdown(BuildComplianceReport(reportFixture(t)))
	for _, want := range []string{
		"# Control report",
		"not a legal attestation", // credibility rule: never a legal claim
		"## 1. Control status",
		"## 2. Events",
		"## 3. Per department",
		"## 4. Explainability and evidence",
		"## 5. Cost and environment (estimates)",
		"provider_not_allowed",
		"personnummer",
		"tn_eko",
		"audit-verify",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
	// PII values must never appear — only type names.
	if strings.Contains(md, "811218") {
		t.Fatal("markdown leaked a PII value")
	}
}

func TestRenderComplianceMarkdownEmptyStatesAreHonest(t *testing.T) {
	md := RenderComplianceMarkdown(BuildComplianceReport(ReportOptions{
		Now: func() time.Time { return time.Unix(1_700_000_000, 0) },
	}))
	for _, want := range []string{
		"no events yet",
		"No tenant data yet",
		"No savings data yet",
		"No providers registered",
		"not active (requires ROUTER_DATA_DIR)",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("empty-state markdown missing %q:\n%s", want, md)
		}
	}
}
