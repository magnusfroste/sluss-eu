package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

func testRequest() *openai.ChatRequest {
	return &openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "fix the auth bug"}},
	}
}

func TestFetchDecision_Success(t *testing.T) {
	var gotAuth, gotExplain, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotExplain = r.Header.Get("X-Router-Explain")
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"selected_model":    "premium-reasoner",
			"selected_provider": "anthropic",
			"policy_version":    "pv_1",
			"timeout_ms":        30000,
			"decision_reasons":  []string{"task=hard_code_debugging", "risk=high"},
			"fallbacks":         []map[string]string{{"model_id": "balanced-coder", "provider_id": "openai"}},
		})
	}))
	defer srv.Close()

	dec, err := fetchDecision(context.Background(), srv.Client(), srv.URL, "secret", testRequest(), true)
	if err != nil {
		t.Fatalf("fetchDecision: %v", err)
	}
	if dec.SelectedModel != "premium-reasoner" || dec.SelectedProvider != "anthropic" {
		t.Errorf("unexpected decision: %+v", dec)
	}
	if len(dec.DecisionReasons) != 2 {
		t.Errorf("reasons = %v", dec.DecisionReasons)
	}
	if len(dec.Fallbacks) != 1 || dec.Fallbacks[0].ModelID != "balanced-coder" {
		t.Errorf("fallbacks = %+v", dec.Fallbacks)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotExplain != "true" {
		t.Errorf("X-Router-Explain = %q", gotExplain)
	}
	if gotPath != "/router/decision" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestFetchDecision_BlockedIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"blocked":      true,
			"block_code":   "provider_not_allowed",
			"block_reason": "policy blocks anthropic for this project",
		})
	}))
	defer srv.Close()

	dec, err := fetchDecision(context.Background(), srv.Client(), srv.URL, "k", testRequest(), false)
	if err != nil {
		t.Fatalf("blocked decision should not be an error: %v", err)
	}
	if !dec.Blocked || dec.BlockCode != "provider_not_allowed" {
		t.Errorf("expected blocked decision, got %+v", dec)
	}
}

func TestFetchDecision_AuthErrorSurfacesMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(openai.ErrorEnvelope{
			Error: openai.ErrorBody{Message: "invalid api key", Type: "unauthorized"},
		})
	}))
	defer srv.Close()

	_, err := fetchDecision(context.Background(), srv.Client(), srv.URL, "bad", testRequest(), false)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("error should surface envelope message, got %v", err)
	}
}

func TestFetchDecision_NoScopeForbiddenIsError(t *testing.T) {
	// A 403 that is NOT a policy block (no blocked field) must be an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(openai.ErrorEnvelope{
			Error: openai.ErrorBody{Message: "api key missing required scope: router:decision", Type: "forbidden", Code: "insufficient_scope"},
		})
	}))
	defer srv.Close()

	if _, err := fetchDecision(context.Background(), srv.Client(), srv.URL, "k", testRequest(), false); err == nil {
		t.Fatal("insufficient scope should be an error, not a decision")
	}
}

func TestRenderDecision(t *testing.T) {
	var buf bytes.Buffer
	render(&buf, decisionResult{
		SelectedModel:    "balanced-coder",
		SelectedProvider: "openai",
		PolicyVersion:    "pv_1",
		TimeoutMS:        20000,
		DecisionReasons:  []string{"task=simple_code_edit"},
		Fallbacks: []struct {
			ModelID    string `json:"model_id"`
			ProviderID string `json:"provider_id"`
		}{{ModelID: "premium-reasoner", ProviderID: "anthropic"}},
	})
	out := buf.String()
	for _, want := range []string{"balanced-coder", "openai", "pv_1", "Fallback chain", "premium-reasoner", "Explanations", "simple_code_edit"} {
		if !strings.Contains(out, want) {
			t.Errorf("render output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderBlocked(t *testing.T) {
	var buf bytes.Buffer
	render(&buf, decisionResult{Blocked: true, BlockCode: "model_not_allowed", BlockReason: "nope"})
	if !strings.Contains(buf.String(), "BLOCKED: model_not_allowed") {
		t.Errorf("unexpected blocked render: %s", buf.String())
	}
}
