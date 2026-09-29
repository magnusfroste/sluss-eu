package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/middleware"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/tenant"
)

func newFallbackHandler(t *testing.T, openaiAdapter, anthropicAdapter provider.Adapter) http.Handler {
	t.Helper()
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	base := ChatCompletionsHandler(openaiAdapter, ChatOptions{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Engine:   engine.New(store),
		Adapters: map[string]provider.Adapter{"openai": openaiAdapter, "anthropic": anthropicAdapter},
	})
	return middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := tenant.WithTenant(r.Context(), &tenant.Tenant{ID: "tn_fb", Project: "prj_fb"})
		base.ServeHTTP(w, r.WithContext(ctx))
	}))
}

// ISSUE-113: the routing engine builds a fallback chain before the first
// provider call, but the non-streaming path only ever tried the primary. A
// 429 on the primary must roll over to the next allowed candidate — the way a
// live demo survives a rate-limited provider.
func TestChat_NonStreamingFallsBackOnPrimaryFailure(t *testing.T) {
	primary := &fakeAdapter{err: fmt.Errorf("%w: provider status 429", provider.ErrProviderRateLimit)}
	fallback := &fakeAdapter{resp: testChatResponse()}
	h := newFallbackHandler(t, primary, fallback)

	rec := postChat(t, h, openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "write a git commit message for this diff"}},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s — fallback chain did not roll over", rec.Code, rec.Body.String())
	}
	if primary.completeCalls == 0 {
		t.Fatal("primary adapter should have been tried first")
	}
	if fallback.completeCalls != 1 {
		t.Fatalf("fallback adapter Complete calls = %d, want 1", fallback.completeCalls)
	}
	if got := rec.Header().Get("X-Router-Selected-Model"); got != "premium-reasoning" {
		t.Fatalf("selected model header = %q, want the anthropic fallback premium-reasoning", got)
	}
	if got := rec.Header().Get("X-Router-Fallback-Index"); got == "" || got == "0" {
		t.Fatalf("fallback index header = %q, want > 0 so the client can see a fallback happened", got)
	}
}

// When every candidate fails, the client sees the last provider error — not a
// generic 500 — and no candidate is retried beyond the chain.
func TestChat_NonStreamingAllCandidatesFail(t *testing.T) {
	primary := &fakeAdapter{err: fmt.Errorf("%w: provider status 429", provider.ErrProviderRateLimit)}
	fallback := &fakeAdapter{err: fmt.Errorf("%w: connection refused", provider.ErrProvider5xx)}
	h := newFallbackHandler(t, primary, fallback)

	rec := postChat(t, h, openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "write a git commit message for this diff"}},
	})

	if rec.Code == http.StatusOK {
		t.Fatalf("expected an error when every candidate fails, got 200: %s", rec.Body.String())
	}
	if primary.completeCalls == 0 || fallback.completeCalls == 0 {
		t.Fatalf("both providers should have been tried: primary=%d fallback=%d", primary.completeCalls, fallback.completeCalls)
	}
	if !strings.Contains(rec.Body.String(), "provider_5xx") {
		t.Fatalf("error should reflect the last attempt's failure, got: %s", rec.Body.String())
	}
}

// buildCompleteCandidates must skip providers without an adapter and collapse
// duplicate (provider, model) pairs, keeping the primary first.
func TestBuildCompleteCandidatesOrderAndDedupe(t *testing.T) {
	a := &fakeAdapter{}
	dec := engine.RouteDecision{
		SelectedProvider: "openai", ProviderModelID: "gpt-4.1", SelectedModel: "balanced-coder",
		Fallbacks: []engine.FallbackEntry{
			{ProviderID: "openai", ProviderModelID: "gpt-4.1", ModelID: "balanced-coder"}, // duplicate of primary
			{ProviderID: "missing", ProviderModelID: "x", ModelID: "x"},                   // no adapter
			{ProviderID: "anthropic", ProviderModelID: "claude-sonnet-4", ModelID: "premium-reasoning"},
		},
	}
	got := buildCompleteCandidates(dec, map[string]provider.Adapter{"openai": a, "anthropic": a})
	if len(got) != 2 || got[0].modelID != "balanced-coder" || got[1].modelID != "premium-reasoning" {
		t.Fatalf("candidates = %+v, want [balanced-coder premium-reasoning]", got)
	}
	_ = context.Background()
}
