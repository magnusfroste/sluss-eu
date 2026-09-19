package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/experiment"
	"github.com/magnusfroste/sluss/internal/middleware"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// serveWithExperiment posts a routed request through a handler whose primary
// policy forces balanced-coder and whose experiment variant forces
// premium-reasoning, returning the response recorder.
func serveWithExperiment(t *testing.T, exp experiment.Config, withVariant bool) *httptest.ResponseRecorder {
	t.Helper()
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	primary := mustForceModelPolicyCache(t, snap, "pv_primary", "tn_exp", "balanced-coder")
	variant := mustForceModelPolicyCache(t, snap, "pv_variant", "tn_exp", "premium-reasoning")

	adapter := &fakeAdapter{resp: testChatResponse()}
	opts := ChatOptions{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Engine:      engine.New(store),
		Adapters:    map[string]provider.Adapter{"openai": adapter, "anthropic": adapter},
		PolicyCache: primary,
		Experiment:  exp,
	}
	if withVariant {
		opts.ExperimentPolicyCache = variant
	}
	base := ChatCompletionsHandler(adapter, opts)
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := tenant.WithTenant(r.Context(), &tenant.Tenant{ID: "tn_exp", Project: "prj_exp"})
		base.ServeHTTP(w, r.WithContext(ctx))
	}))
	rec := postChat(t, h, openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "route this"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	return rec
}

// A 100% experiment serves the variant policy and tags the treatment arm.
func TestExperimentTreatmentServesVariantPolicy(t *testing.T) {
	rec := serveWithExperiment(t, experiment.Config{Percentage: 100, Salt: "s"}, true)
	if arm := rec.Header().Get("X-Router-Experiment-Arm"); arm != "treatment" {
		t.Fatalf("arm header = %q, want treatment", arm)
	}
	if got := rec.Header().Get("X-Router-Selected-Model"); got != "premium-reasoning" {
		t.Fatalf("selected model = %q, want premium-reasoning (variant policy)", got)
	}
}

// A disabled experiment keeps the primary policy and sets no arm header.
func TestExperimentDisabledKeepsPrimary(t *testing.T) {
	rec := serveWithExperiment(t, experiment.Config{Percentage: 0}, true)
	if arm := rec.Header().Get("X-Router-Experiment-Arm"); arm != "" {
		t.Fatalf("arm header = %q, want empty (experiment off)", arm)
	}
	if got := rec.Header().Get("X-Router-Selected-Model"); got != "balanced-coder" {
		t.Fatalf("selected model = %q, want balanced-coder (primary policy)", got)
	}
}

// Enabled percentage but no variant policy configured: stays control on the
// primary policy, no accidental treatment.
func TestExperimentNoVariantCacheKeepsPrimary(t *testing.T) {
	rec := serveWithExperiment(t, experiment.Config{Percentage: 100, Salt: "s"}, false)
	if arm := rec.Header().Get("X-Router-Experiment-Arm"); arm != "" {
		t.Fatalf("arm header = %q, want empty when no variant policy is set", arm)
	}
	if got := rec.Header().Get("X-Router-Selected-Model"); got != "balanced-coder" {
		t.Fatalf("selected model = %q, want balanced-coder", got)
	}
}
