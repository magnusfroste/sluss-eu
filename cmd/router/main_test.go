package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/magnusfroste/sluss/internal/budget"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/spend"
)

func TestLoadRuntimePolicyCacheFromPathEnablesContextPipeline(t *testing.T) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("default snapshot: %v", err)
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(`
version: pv_runtime_custom
settings:
  default_model_profile: balanced
  conservative_unknowns: true
  max_router_overhead_ms: 100
  default_timeout_ms: 30000
  default_retention: standard
rules:
  - id: enable_context_pipeline_for_local
    when:
      tenant: tn_local
    route:
      force:
        context_pipeline: true
`), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	cache, err := loadRuntimePolicyCache(snap, path)
	if err != nil {
		t.Fatalf("loadRuntimePolicyCache: %v", err)
	}
	compiled, ok := cache.Active(policy.Scope{})
	if !ok {
		t.Fatal("expected default runtime policy")
	}
	eval := compiled.Evaluate(policy.EvaluationInput{TenantID: "tn_local"})
	if !eval.Route.ContextPipelineEnabled() {
		t.Fatalf("context pipeline should be policy-enabled: %+v", eval.Route)
	}
}

func TestBuildPromptAdapterEnabledAppliesDefaultProfileRules(t *testing.T) {
	adapter := buildPromptAdapter(true)
	if adapter == nil {
		t.Fatal("expected enabled prompt adapter")
	}
	req := &provider.NormalizedModelRequest{
		Model:    "cheap-general",
		Messages: []openai.Message{{Role: "system", Content: "You are concise."}, {Role: "user", Content: "Hi"}},
	}

	adapted, result := adapter.Apply(req, provider.PromptAdapterContext{ModelID: "cheap-general"})
	if len(result.AppliedRules) != 1 || result.AppliedRules[0] != "cheap-system-cost-aware" {
		t.Fatalf("applied rules = %v", result.AppliedRules)
	}
	if adapted == nil || adapted.Messages[0].Content == req.Messages[0].Content {
		t.Fatalf("expected system prompt mutation, got %#v", adapted)
	}
	if req.Messages[0].Content != "You are concise." {
		t.Fatalf("adapter mutated original request: %q", req.Messages[0].Content)
	}
}

func TestBuildEventHandlerFansOutShadowComparisons(t *testing.T) {
	tracker := eventlog.NewComparisonTracker(10)
	handler := buildEventHandler(slog.New(slog.NewTextHandler(os.Stderr, nil)), spend.New(), budget.NewLedger(), tracker, eventlog.NewRequestLogTracker(10), nil, nil)
	handler.Handle(context.Background(), eventlog.Event{
		Type: eventlog.EventTypeDecision,
		Decision: &eventlog.DecisionEvent{
			RequestID: "req_1",
			TaskType:  "simple_chat",
			ShadowComparison: &engine.DecisionComparison{
				Changed: true,
				Primary: engine.RouteDecision{
					SelectedModel: "cheap-general",
				},
				Secondary: engine.RouteDecision{
					SelectedModel: "premium-reasoning",
				},
			},
		},
	})

	if got := tracker.Summary().ChangedCount; got != 1 {
		t.Fatalf("shadow changed count = %d, want 1", got)
	}
}
