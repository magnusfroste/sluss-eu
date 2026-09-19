package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/provider"
	"github.com/magnusfroste/sluss/internal/server"
	"github.com/magnusfroste/sluss/internal/tenant"
)

type fakeAdapter struct {
	resp *openai.ChatResponse
}

func (f *fakeAdapter) Name() string { return "fake" }

func (f *fakeAdapter) Complete(context.Context, *provider.NormalizedModelRequest) (*openai.ChatResponse, error) {
	return f.resp, nil
}

func testChatResponse() *openai.ChatResponse {
	return &openai.ChatResponse{
		ID:    "chatcmpl_test",
		Model: "balanced-coder",
		Choices: []openai.Choice{{
			Message: openai.Message{Role: "assistant", Content: "hi"},
		}},
	}
}

// TestChatEndpointEnforcesScope verifies the chat route, wired through
// server.New, rejects a key that lacks the chat:completions scope (ISSUE-046).
func TestChatEndpointEnforcesScope(t *testing.T) {
	store := auth.NewInMemoryKeyStore()
	// Key is limited to router:decision — no chat scope.
	store.Add("scoped-key", &tenant.Tenant{
		ID: "tn", Project: "prj", KeyID: "key", Role: auth.RoleUser, Scopes: []string{auth.ScopeRouterDecision},
	})

	h := server.New(server.Config{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		KeyStore: store,
		Provider: &fakeAdapter{resp: testChatResponse()},
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	body, _ := json.Marshal(openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "ping"}},
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer scoped-key")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	var env openai.ErrorEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if env.Error.Code != "insufficient_scope" {
		t.Errorf("error code = %q, want insufficient_scope", env.Error.Code)
	}
}

func TestChatEndpointAllowsUserRoleWhenScopePresent(t *testing.T) {
	store := auth.NewInMemoryKeyStore()
	store.Add("chat-key", &tenant.Tenant{
		ID: "tn", Project: "prj", KeyID: "key", Role: auth.RoleUser, Scopes: []string{auth.ScopeChatCompletions},
	})

	h := server.New(server.Config{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		KeyStore: store,
		Provider: &fakeAdapter{resp: testChatResponse()},
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	body, _ := json.Marshal(openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: "ping"}},
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer chat-key")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDashboardEndpointRequiresAdminRole(t *testing.T) {
	store := auth.NewInMemoryKeyStore()
	store.Add("user-key", &tenant.Tenant{
		ID: "tn", Project: "prj", KeyID: "key_user", Role: auth.RoleUser,
	})
	store.Add("admin-key", &tenant.Tenant{
		ID: "tn", Project: "prj", KeyID: "key_admin", Role: auth.RoleAdmin,
	})
	store.Add("legacy-key", &tenant.Tenant{
		ID: "tn", Project: "prj", KeyID: "key_legacy",
	})

	h := server.New(server.Config{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		KeyStore: store,
		Provider: &fakeAdapter{resp: testChatResponse()},
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	cases := []struct {
		name       string
		path       string
		key        string
		wantStatus int
		wantCode   string
	}{
		{name: "missing bearer token", path: "/router/dashboard", wantStatus: http.StatusUnauthorized},
		{name: "user role forbidden", path: "/router/dashboard/data?task=chat", key: "user-key", wantStatus: http.StatusForbidden, wantCode: "insufficient_role"},
		{name: "admin role allowed", path: "/router/dashboard", key: "admin-key", wantStatus: http.StatusOK},
		{name: "legacy key allowed", path: "/router/dashboard/data", key: "legacy-key", wantStatus: http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, srv.URL+tc.path, nil)
			if tc.key != "" {
				req.Header.Set("Authorization", "Bearer "+tc.key)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if tc.wantCode == "" {
				return
			}

			var env openai.ErrorEnvelope
			if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
				t.Fatalf("decode error envelope: %v", err)
			}
			if env.Error.Code != tc.wantCode {
				t.Fatalf("error code = %q, want %q", env.Error.Code, tc.wantCode)
			}
		})
	}
}
