package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/middleware"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/router"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// answerAdapter returns a fixed answer and counts calls (concurrency-safe).
type answerAdapter struct {
	name   string
	answer string
	calls  int64
}

func (a *answerAdapter) Name() string { return a.name }
func (a *answerAdapter) Complete(_ context.Context, _ *provider.NormalizedModelRequest) (*openai.ChatResponse, error) {
	atomic.AddInt64(&a.calls, 1)
	return &openai.ChatResponse{
		Model:   a.name,
		Choices: []openai.Choice{{Message: openai.Message{Role: "assistant", Content: a.answer}}},
		Usage:   openai.Usage{PromptTokens: 12, CompletionTokens: 8, TotalTokens: 20},
	}, nil
}

func TestCouncilEnabledGate(t *testing.T) {
	cfg := ChatOptions{CouncilTasks: map[string]bool{"security_review": true}}
	job := &router.JobDescriptor{TaskType: router.TaskSecurityReview}

	if !cfg.councilEnabled(job, false) {
		t.Fatal("council should be enabled for a listed task on the non-streaming path")
	}
	if cfg.councilEnabled(job, true) {
		t.Fatal("council must be disabled for streaming requests")
	}
	if cfg.councilEnabled(&router.JobDescriptor{TaskType: router.TaskSimpleChat}, false) {
		t.Fatal("council must be disabled for an unlisted task")
	}
	empty := ChatOptions{}
	if empty.councilEnabled(job, false) {
		t.Fatal("empty CouncilTasks must disable council")
	}
}

func TestCouncilBuildPanelDedupsAndCaps(t *testing.T) {
	a := &answerAdapter{name: "a"}
	b := &answerAdapter{name: "b"}
	cfg := ChatOptions{
		CouncilSize: 2,
		Adapters:    map[string]provider.Adapter{"pa": a, "pb": b},
	}
	dec := engine.RouteDecision{
		SelectedProvider: "pa", ProviderModelID: "pa-m1", SelectedModel: "m1",
		Fallbacks: []engine.FallbackEntry{
			{ProviderID: "pa", ProviderModelID: "pa-m1", ModelID: "m1"}, // dup of selected → skipped
			{ProviderID: "pb", ProviderModelID: "pb-m2", ModelID: "m2"},
			{ProviderID: "pb", ProviderModelID: "pb-m3", ModelID: "m3"}, // over cap → skipped
		},
	}
	panel := cfg.buildCouncilPanel(dec)
	if len(panel) != 2 {
		t.Fatalf("panel size = %d, want 2 (deduped + capped)", len(panel))
	}
	if panel[0].ModelID != "m1" || panel[1].ModelID != "m2" {
		t.Fatalf("panel = %q,%q want m1,m2", panel[0].ModelID, panel[1].ModelID)
	}
}

func TestCouncilBuildPanelSkipsMissingAdapter(t *testing.T) {
	cfg := ChatOptions{Adapters: map[string]provider.Adapter{}} // no adapters registered
	dec := engine.RouteDecision{SelectedProvider: "pa", ProviderModelID: "x", SelectedModel: "m1"}
	if got := len(cfg.buildCouncilPanel(dec)); got != 0 {
		t.Fatalf("panel with no resolvable adapter = %d, want 0", got)
	}
}

// End-to-end: a request whose task class is council-enabled runs the panel and
// returns the consensus answer, with council headers set and every member
// called exactly once.
func TestChat_CouncilFiresAndReturnsConsensus(t *testing.T) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	// A hard-debugging prompt routes at balanced tier → panel spans the balanced
	// (openai) and premium (anthropic) models: two distinct providers.
	req := openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "Debug this panic: nil pointer dereference in the stack trace, goroutine crashed"}},
	}
	// Discover the task class the router assigns, so the gate matches regardless
	// of classifier internals.
	probe := router.NewJobDescriptor(router.JobDescriptorInput{Request: &req})
	taskClass := string(probe.TaskType)

	const consensus = "The fix is to add a nil check before dereferencing"
	openaiAdapter := &answerAdapter{name: "openai", answer: consensus}
	anthropicAdapter := &answerAdapter{name: "anthropic", answer: consensus}

	base := ChatCompletionsHandler(openaiAdapter, ChatOptions{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Engine:       engine.New(store),
		Adapters:     map[string]provider.Adapter{"openai": openaiAdapter, "anthropic": anthropicAdapter},
		CouncilTasks: map[string]bool{taskClass: true},
	})
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := tenant.WithTenant(r.Context(), &tenant.Tenant{ID: "tn_c", Project: "prj_c"})
		base.ServeHTTP(w, r.WithContext(ctx))
	}))

	rec := postChat(t, h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("X-Router-Council") != "true" {
		t.Fatalf("expected council to fire for task %q; headers=%v", taskClass, rec.Header())
	}
	if got := rec.Header().Get("X-Router-Council-Success"); got != "2" {
		t.Fatalf("council success = %q, want 2", got)
	}
	if got := rec.Header().Get("X-Router-Council-Agreement"); got != "1.00" {
		t.Fatalf("council agreement = %q, want 1.00 (unanimous)", got)
	}

	totalCalls := atomic.LoadInt64(&openaiAdapter.calls) + atomic.LoadInt64(&anthropicAdapter.calls)
	if totalCalls != 2 {
		t.Fatalf("total member calls = %d, want 2 (one per panel member)", totalCalls)
	}

	var got openai.ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Choices) == 0 || got.Choices[0].Message.Content != consensus {
		t.Fatalf("body content = %+v, want consensus answer", got.Choices)
	}
}
