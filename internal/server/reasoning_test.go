package server

import (
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
)

// reasoningRegistry has one reasoning-capable model (the on-prem Qwen case) and
// one ordinary cloud model.
func reasoningRegistry(t *testing.T) *engine.Engine {
	t.Helper()
	cost := registry.CostMetadata{Currency: "USD", InputMicrosPerMillionToken: 1_000_000, OutputMicrosPerMillionToken: 1_000_000}
	caps := registry.Capabilities{Chat: true, Streaming: true}
	def := registry.Definition{
		RegistryVersion: "reg-reasoning-test",
		CreatedAt:       time.Unix(1_700_000_000, 0),
		Providers: []registry.Provider{
			{ID: "dgx", Name: "DGX", Status: registry.ProviderStatusActive, ComplianceTags: []string{"local"}},
			{ID: "cloud", Name: "Cloud", Status: registry.ProviderStatusActive},
		},
		Models: []registry.Model{
			{ID: "qwen", ProviderID: "dgx", ProviderModelID: "qwen36-27b", Tier: registry.TierBalanced,
				Capabilities: caps, Cost: cost, Enabled: true, ReasoningCapable: true},
			{ID: "gpt", ProviderID: "cloud", ProviderModelID: "gpt-x", Tier: registry.TierBalanced,
				Capabilities: caps, Cost: cost, Enabled: true},
		},
	}
	snap, err := registry.NewSnapshot(def)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return engine.New(store)
}

// A reasoning-capable model gets an explicit thinking directive that follows the
// job's requires_reasoning signal, plus a raised timeout. Ordinary models get
// neither (nil → no extra request fields, adapter default timeout).
func TestReasoningDirectiveFollowsJobSignal(t *testing.T) {
	cfg := &ChatOptions{Engine: reasoningRegistry(t)}

	// Simple task: thinking explicitly OFF, raised timeout (slow on-prem GPU).
	simple := &router.JobDescriptor{TaskType: router.TaskSummarization, RequiresReasoning: false}
	think, timeout := cfg.reasoningDirective("qwen", simple)
	if think == nil || *think {
		t.Fatalf("simple task on reasoning model: want explicit false, got %v", think)
	}
	if timeout != reasoningTimeout {
		t.Fatalf("timeout = %v, want %v", timeout, reasoningTimeout)
	}

	// Hard task: thinking ON.
	hard := &router.JobDescriptor{TaskType: router.TaskHardCodeDebugging, RequiresReasoning: true}
	think, _ = cfg.reasoningDirective("qwen", hard)
	if think == nil || !*think {
		t.Fatalf("hard task on reasoning model: want explicit true, got %v", think)
	}

	// Ordinary model: no directive, no timeout override — nothing extra is sent.
	think, timeout = cfg.reasoningDirective("gpt", hard)
	if think != nil || timeout != 0 {
		t.Fatalf("ordinary model must get nil/0, got %v/%v", think, timeout)
	}

	// Unknown model id: nil/0 (fail quiet, adapter defaults apply).
	if think, timeout = cfg.reasoningDirective("nope", hard); think != nil || timeout != 0 {
		t.Fatalf("unknown model must get nil/0, got %v/%v", think, timeout)
	}
}
