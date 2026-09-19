package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func testServer() *Server {
	s := NewServer("test", "0.0.1")
	s.Register(Tool{
		Name:        "echo",
		Description: "echoes its input",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{
			"msg": map[string]any{"type": "string"},
		}},
		Handler: func(_ context.Context, args json.RawMessage) (any, error) {
			var in struct {
				Msg string `json:"msg"`
			}
			_ = json.Unmarshal(args, &in)
			return map[string]any{"echo": in.Msg}, nil
		},
	})
	s.Register(Tool{
		Name:    "boom",
		Handler: func(context.Context, json.RawMessage) (any, error) { return nil, errors.New("kaboom") },
	})
	return s
}

func decode(t *testing.T, b []byte) rpcResponse {
	t.Helper()
	var r rpcResponse
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatalf("decode response: %v (raw: %s)", err, b)
	}
	if r.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", r.JSONRPC)
	}
	return r
}

func TestInitialize(t *testing.T) {
	s := testServer()
	resp, ok := s.HandleMessage(context.Background(),
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`))
	if !ok {
		t.Fatal("expected a response for initialize")
	}
	r := decode(t, resp)
	res := r.Result.(map[string]any)
	if res["protocolVersion"] != "2025-06-18" {
		t.Errorf("protocolVersion = %v, want echoed 2025-06-18", res["protocolVersion"])
	}
	if _, ok := res["capabilities"].(map[string]any)["tools"]; !ok {
		t.Errorf("expected tools capability")
	}
	if res["serverInfo"].(map[string]any)["name"] != "test" {
		t.Errorf("serverInfo.name = %v, want test", res["serverInfo"])
	}
}

func TestNotificationHasNoResponse(t *testing.T) {
	s := testServer()
	resp, ok := s.HandleMessage(context.Background(),
		[]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if ok || resp != nil {
		t.Fatalf("notification should get no response, got ok=%v resp=%s", ok, resp)
	}
}

func TestToolsList(t *testing.T) {
	s := testServer()
	resp, _ := s.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	r := decode(t, resp)
	tools := r.Result.(map[string]any)["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("want 2 tools, got %d", len(tools))
	}
	first := tools[0].(map[string]any)
	if first["name"] != "echo" {
		t.Errorf("first tool = %v, want echo", first["name"])
	}
	if _, ok := first["inputSchema"]; !ok {
		t.Errorf("tool missing inputSchema")
	}
}

func TestToolsCallSuccess(t *testing.T) {
	s := testServer()
	resp, _ := s.HandleMessage(context.Background(),
		[]byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"msg":"hi"}}}`))
	r := decode(t, resp)
	if r.Error != nil {
		t.Fatalf("unexpected error: %+v", r.Error)
	}
	content := r.Result.(map[string]any)["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, `"echo": "hi"`) {
		t.Errorf("content text = %q, want echoed hi", text)
	}
}

func TestToolsCallHandlerErrorIsIsError(t *testing.T) {
	s := testServer()
	resp, _ := s.HandleMessage(context.Background(),
		[]byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"boom"}}`))
	r := decode(t, resp)
	if r.Error != nil {
		t.Fatalf("handler failures must be results, not protocol errors: %+v", r.Error)
	}
	res := r.Result.(map[string]any)
	if res["isError"] != true {
		t.Errorf("expected isError=true, got %v", res["isError"])
	}
}

func TestUnknownToolIsError(t *testing.T) {
	s := testServer()
	resp, _ := s.HandleMessage(context.Background(),
		[]byte(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nope"}}`))
	r := decode(t, resp)
	if r.Error == nil {
		t.Fatal("expected an error for unknown tool")
	}
}

func TestUnknownMethod(t *testing.T) {
	s := testServer()
	resp, _ := s.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":6,"method":"does/not/exist"}`))
	r := decode(t, resp)
	if r.Error == nil || r.Error.Code != codeMethodNotFound {
		t.Fatalf("want method-not-found, got %+v", r.Error)
	}
}

func TestServeStdioRoundTrip(t *testing.T) {
	s := testServer()
	in := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n" +
			`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")
	var out bytes.Buffer
	if err := ServeStdio(context.Background(), s, in, &out); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	// initialize + tools/list responses; the notification produces no line.
	if len(lines) != 2 {
		t.Fatalf("want 2 response lines, got %d: %q", len(lines), out.String())
	}
	for _, ln := range lines {
		decode(t, []byte(ln))
	}
}
