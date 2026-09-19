package server

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/middleware"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/tenant"
)

func TestChatSetsUsageAndCostHeaders(t *testing.T) {
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	resp := testChatResponse()
	resp.Usage = openai.Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}
	fake := &fakeAdapter{resp: resp}

	base := ChatCompletionsHandler(fake, ChatOptions{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Engine:   engine.New(store),
		Adapters: map[string]provider.Adapter{"openai": fake, "anthropic": fake},
	})
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := tenant.WithTenant(r.Context(), &tenant.Tenant{ID: "tn", Project: "prj"})
		base.ServeHTTP(w, r.WithContext(ctx))
	}))

	rec := postChat(t, h, openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "say hello"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Router-Input-Tokens"); got != "1000" {
		t.Errorf("input tokens header = %q, want 1000", got)
	}
	if got := rec.Header().Get("X-Router-Output-Tokens"); got != "500" {
		t.Errorf("output tokens header = %q, want 500", got)
	}
	// Cost is priced from the selected model's registry cost; must be present and
	// non-zero for non-zero usage.
	cost := rec.Header().Get("X-Router-Cost-USD")
	if cost == "" || cost == "0.000000" {
		t.Errorf("cost header = %q, want a non-zero USD amount", cost)
	}
}
