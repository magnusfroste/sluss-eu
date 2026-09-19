package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/decisioncache"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/middleware"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// engineHandler builds an engine-backed chat handler with a decision cache and a
// fake adapter for every provider, wrapped to inject a low-risk tenant context.
func engineHandler(t *testing.T, cache *decisioncache.Cache) http.Handler {
	t.Helper()
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	fake := &fakeAdapter{resp: testChatResponse()}
	adapters := map[string]provider.Adapter{"openai": fake, "anthropic": fake}

	base := ChatCompletionsHandler(fake, ChatOptions{
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		Engine:          engine.New(store),
		Adapters:        adapters,
		DecisionCache:   cache,
		RegistryVersion: snap.RegistryVersion(),
	})
	return middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := tenant.WithTenant(r.Context(), &tenant.Tenant{ID: "tn", Project: "prj"})
		base.ServeHTTP(w, r.WithContext(ctx))
	}))
}

func postLowRisk(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	return postChat(t, h, openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "say hello"}},
	})
}

func TestDecisionCacheMissThenHit(t *testing.T) {
	cache := decisioncache.New(time.Minute, 0)
	h := engineHandler(t, cache)

	first := postLowRisk(t, h)
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	if got := first.Header().Get("X-Router-Cache"); got != "miss" {
		t.Fatalf("first X-Router-Cache = %q, want miss", got)
	}
	if cache.Len() != 1 {
		t.Fatalf("decision should be cached, len=%d", cache.Len())
	}

	second := postLowRisk(t, h)
	if got := second.Header().Get("X-Router-Cache"); got != "hit" {
		t.Fatalf("second X-Router-Cache = %q, want hit", got)
	}
}

func TestDecisionCacheDisabledNoHeader(t *testing.T) {
	h := engineHandler(t, nil) // no cache configured
	rec := postLowRisk(t, h)
	if got := rec.Header().Get("X-Router-Cache"); got != "" {
		t.Errorf("no cache should set no header, got %q", got)
	}
}

func TestCacheStatusHelper(t *testing.T) {
	cases := []struct {
		cacheable, hit bool
		want           string
	}{
		{false, false, "bypass"},
		{true, false, "miss"},
		{true, true, "hit"},
	}
	for _, c := range cases {
		if got := cacheStatus(c.cacheable, c.hit); got != c.want {
			t.Errorf("cacheStatus(%v,%v) = %q, want %q", c.cacheable, c.hit, got, c.want)
		}
	}
}
