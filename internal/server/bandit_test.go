package server

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/magnusfroste/sluss/internal/bandit"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/eventlog"
	"github.com/magnusfroste/sluss/internal/middleware"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/tenant"
)

func serveForBandit(t *testing.T, adapterErr error) *bandit.Bandit {
	t.Helper()
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	pol := mustForceModelPolicyCache(t, snap, "pv_bandit", "tn_b", "balanced-coder")
	b := bandit.New(1.41)

	adapter := &fakeAdapter{resp: testChatResponse(), err: adapterErr}
	base := ChatCompletionsHandler(adapter, ChatOptions{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Engine:      engine.New(store),
		Adapters:    map[string]provider.Adapter{"openai": adapter, "anthropic": adapter},
		PolicyCache: pol,
		EventQueue:  eventlog.NewQueue(4), // present so recordAttempt runs fully
		Bandit:      b,
	})
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := tenant.WithTenant(r.Context(), &tenant.Tenant{ID: "tn_b", Project: "prj_b"})
		base.ServeHTTP(w, r.WithContext(ctx))
	}))
	postChat(t, h, openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "route this"}},
	})
	return b
}

func TestBanditRecordsSuccessReward(t *testing.T) {
	b := serveForBandit(t, nil)
	snap := b.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("bandit arms = %d, want 1", len(snap))
	}
	if snap[0].ModelID != "balanced-coder" {
		t.Fatalf("arm model = %q, want balanced-coder", snap[0].ModelID)
	}
	if snap[0].Pulls != 1 || snap[0].MeanReward != 1.0 {
		t.Fatalf("arm = %+v, want 1 pull with reward 1.0 (success)", snap[0])
	}
}

func TestBanditRecordsFailureReward(t *testing.T) {
	b := serveForBandit(t, errors.New("boom"))
	snap := b.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("bandit arms = %d, want 1", len(snap))
	}
	if snap[0].Pulls != 1 || snap[0].MeanReward != 0.0 {
		t.Fatalf("arm = %+v, want 1 pull with reward 0.0 (failure)", snap[0])
	}
}
