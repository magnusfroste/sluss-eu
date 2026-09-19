package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/middleware"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/server"
	"github.com/magnusfroste/sluss/internal/tenant"
)

// TestSprint1_EndToEnd is the Sprint 1 Definition-of-Done acceptance test:
// a client posts an OpenAI-style chat completion to the router, which proxies
// to a mock provider and returns a normalized response with a request id.
func TestSprint1_EndToEnd(t *testing.T) {
	// Stand up an in-process mock provider that mirrors the cmd/mock-provider
	// binary closely enough for the proxy path.
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req openai.ChatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(openai.ChatResponse{
			ID:      "chatcmpl_e2e",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []openai.Choice{{
				Message:      openai.Message{Role: "assistant", Content: "ok"},
				FinishReason: "stop",
			}},
			Usage: openai.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		})
	}))
	defer mockSrv.Close()

	store := auth.NewInMemoryKeyStore()
	store.Add("test-key", &tenant.Tenant{ID: "tn_test", Project: "prj_test", KeyID: "key_test"})

	h := server.New(server.Config{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		KeyStore: store,
		Provider: &provider.MockAdapter{
			BaseURL: mockSrv.URL,
			Client:  &http.Client{Timeout: 5 * time.Second},
		},
	})

	routerSrv := httptest.NewServer(h)
	defer routerSrv.Close()

	body, _ := json.Marshal(openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "ping"}},
	})

	req, _ := http.NewRequest(http.MethodPost, routerSrv.URL+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("router request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(raw))
	}

	if id := resp.Header.Get(middleware.HeaderRequestID); !strings.HasPrefix(id, "req_") {
		t.Fatalf("expected req_ id header, got %q", id)
	}
	if got := resp.Header.Get("X-Router-Selected-Model"); got != "auto" {
		t.Fatalf("expected X-Router-Selected-Model=auto, got %q", got)
	}

	var out openai.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("invalid response: %v", err)
	}
	if out.ID != "chatcmpl_e2e" || len(out.Choices) != 1 || out.Choices[0].Message.Content != "ok" {
		t.Fatalf("unexpected response: %+v", out)
	}
}

type panicAdapter struct{}

func (panicAdapter) Name() string { return "panic" }
func (panicAdapter) Complete(_ context.Context, _ *provider.NormalizedModelRequest) (*openai.ChatResponse, error) {
	panic("provider must not be called when request is rejected upstream")
}

func TestSprint1_RejectsUnauthenticated(t *testing.T) {
	store := auth.NewInMemoryKeyStore()
	h := server.New(server.Config{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		KeyStore: store,
		Provider: panicAdapter{},
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/chat/completions", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSprint1_HealthzAndReadyzArePublic(t *testing.T) {
	h := server.New(server.Config{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		KeyStore: auth.NewInMemoryKeyStore(),
		Provider: panicAdapter{},
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	for _, path := range []string{"/healthz", "/readyz"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, resp.StatusCode)
		}
	}
}
