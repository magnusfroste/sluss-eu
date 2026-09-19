package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magnusfroste/sluss/internal/auth"
	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/registry"
	"github.com/magnusfroste/sluss/internal/server"
	"github.com/magnusfroste/sluss/internal/tenant"
)

func mcpTestHandler(t *testing.T, dashboardPassword string) (http.Handler, *auth.InMemoryKeyStore) {
	t.Helper()
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	ks := auth.NewInMemoryKeyStore()
	ks.Add("mcp-key", &tenant.Tenant{ID: "tn", Project: "prj", KeyID: "k"})
	h := server.New(server.Config{
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		KeyStore:          ks,
		Engine:            engine.New(store),
		RegistryVersion:   snap.RegistryVersion(),
		MCPEnabled:        true,
		DashboardPassword: dashboardPassword,
	})
	return h, ks
}

func postMCP(t *testing.T, h http.Handler, auth string, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(body)))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestMCPHTTPRequiresAuth(t *testing.T) {
	h, _ := mcpTestHandler(t, "")
	resp := postMCP(t, h, "", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestMCPHTTPWithAPIKey(t *testing.T) {
	h, _ := mcpTestHandler(t, "")
	resp := postMCP(t, h, "Bearer mcp-key", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	// 10 read tools + 2 always-on usage tools (write tools are gated off here).
	if len(out.Result.Tools) != 12 {
		t.Fatalf("want 12 tools, got %d", len(out.Result.Tools))
	}
}

func TestMCPHTTPWithDashboardPassword(t *testing.T) {
	h, _ := mcpTestHandler(t, "s3cret")
	// The dashboard password, presented as a Bearer token, is accepted.
	resp := postMCP(t, h, "Bearer s3cret", `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// A wrong secret is rejected.
	bad := postMCP(t, h, "Bearer nope", `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	if bad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for wrong secret", bad.StatusCode)
	}
}

func TestMCPHTTPToolCall(t *testing.T) {
	h, _ := mcpTestHandler(t, "")
	resp := postMCP(t, h, "Bearer mcp-key",
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"route_explain","arguments":{"prompt":"list files in the current directory"}}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	if out.Result.IsError {
		t.Fatalf("route_explain returned isError: %s", body)
	}
	if len(out.Result.Content) == 0 || !bytes.Contains([]byte(out.Result.Content[0].Text), []byte("selected_model")) {
		t.Fatalf("expected selected_model in content, got %s", body)
	}
}

func TestMCPRouteExplainSignals(t *testing.T) {
	h, _ := mcpTestHandler(t, "")
	resp := postMCP(t, h, "Bearer mcp-key",
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"route_explain","arguments":{"prompt":"debug this deadlock in my concurrent Go code"}}}`)
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil || len(out.Result.Content) == 0 {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	text := out.Result.Content[0].Text
	// The enriched result exposes the classification signals (policy_version is
	// omitempty — absent under the default no-version policy).
	for _, want := range []string{"signals", "sensitivity", "requires_code", "task_confidence", "requires_reasoning"} {
		if !bytes.Contains([]byte(text), []byte(want)) {
			t.Errorf("route_explain result missing %q: %s", want, text)
		}
	}
}

func TestMCPGetPolicy(t *testing.T) {
	h, _ := mcpTestHandler(t, "")
	resp := postMCP(t, h, "Bearer mcp-key",
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_policy","arguments":{}}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil || len(out.Result.Content) == 0 {
		t.Fatalf("get_policy decode: %v (body %s)", err, body)
	}
	// With a policy cache: found + rule_count/version. Without one (minimal test
	// harness): a graceful "policy cache not configured" error. Either is fine —
	// the tool must respond without panicking.
	txt := out.Result.Content[0].Text
	if !bytes.Contains([]byte(txt), []byte("found")) && !bytes.Contains([]byte(txt), []byte("policy")) {
		t.Errorf("get_policy result unexpected: %s", txt)
	}
}

func TestMCPExplainRequestNotFound(t *testing.T) {
	h, _ := mcpTestHandler(t, "")
	resp := postMCP(t, h, "Bearer mcp-key",
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"explain_request","arguments":{"request_id":"does-not-exist"}}}`)
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil || len(out.Result.Content) == 0 {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	txt := out.Result.Content[0].Text
	if !bytes.Contains([]byte(txt), []byte("found")) && !bytes.Contains([]byte(txt), []byte("history")) {
		t.Errorf("explain_request unexpected: %s", txt)
	}
}

func TestMCPServerInfo(t *testing.T) {
	h, _ := mcpTestHandler(t, "")
	resp := postMCP(t, h, "Bearer mcp-key",
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"server_info","arguments":{}}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.Result.IsError || len(out.Result.Content) == 0 {
		t.Fatalf("server_info failed: %v (body %s)", err, body)
	}
	text := out.Result.Content[0].Text
	for _, want := range []string{"registry_version", "roster_source", "enabled_models", "flags", "conservative_mode", "pricing_sync"} {
		if !bytes.Contains([]byte(text), []byte(want)) {
			t.Errorf("server_info missing %q: %s", want, text)
		}
	}
}

func TestMCPDisabledByDefault(t *testing.T) {
	snap, _ := registry.DefaultSnapshot()
	store, _ := registry.NewStore(snap)
	ks := auth.NewInMemoryKeyStore()
	ks.Add("mcp-key", &tenant.Tenant{ID: "tn", Project: "prj", KeyID: "k"})
	h := server.New(server.Config{
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		KeyStore:        ks,
		Engine:          engine.New(store),
		RegistryVersion: snap.RegistryVersion(),
		// MCPEnabled defaults to false.
	})
	resp := postMCP(t, h, "Bearer mcp-key", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when MCP disabled", resp.StatusCode)
	}
}
