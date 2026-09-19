package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A non-streaming Messages request is translated to an OpenAI chat request, and
// the OpenAI response is translated back to Messages format.
func TestAnthropicNonStreaming(t *testing.T) {
	// Fake chat handler: asserts the translated OpenAI request, returns an
	// OpenAI-shaped response.
	var gotOpenAI map[string]any
	chat := func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotOpenAI)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"cmpl_1","model":"qwen36-27b","choices":[{"message":{"role":"assistant","content":"Stockholm."},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":3}}`))
	}
	h := AnthropicMessagesHandler(chat)

	body := `{"model":"claude-3-5-sonnet","max_tokens":100,"system":"Be terse.","messages":[{"role":"user","content":"Capital of Sweden?"}]}`
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))

	// Request translation: model→auto, system prepended, max_tokens carried.
	if gotOpenAI["model"] != "auto" {
		t.Fatalf("model should be auto, got %v", gotOpenAI["model"])
	}
	msgs, _ := gotOpenAI["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("want system+user = 2 messages, got %d (%+v)", len(msgs), msgs)
	}
	if first, _ := msgs[0].(map[string]any); first["role"] != "system" || first["content"] != "Be terse." {
		t.Fatalf("system message wrong: %+v", msgs[0])
	}

	// Response translation: Messages shape.
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	if out["type"] != "message" || out["role"] != "assistant" || out["stop_reason"] != "end_turn" {
		t.Fatalf("bad message envelope: %+v", out)
	}
	content, _ := out["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("want one content block, got %+v", out["content"])
	}
	blk, _ := content[0].(map[string]any)
	if blk["type"] != "text" || blk["text"] != "Stockholm." {
		t.Fatalf("content block wrong: %+v", blk)
	}
	usage, _ := out["usage"].(map[string]any)
	if usage["input_tokens"].(float64) != 11 || usage["output_tokens"].(float64) != 3 {
		t.Fatalf("usage wrong: %+v", usage)
	}
}

// A streaming Messages request produces Anthropic SSE events translated from the
// OpenAI SSE stream the chat pipeline emits.
func TestAnthropicStreaming(t *testing.T) {
	chat := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fl, _ := w.(http.Flusher)
		for _, d := range []string{
			`{"choices":[{"delta":{"content":"Stock"}}]}`,
			`{"choices":[{"delta":{"content":"holm."}}]}`,
			`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"completion_tokens":3}}`,
		} {
			_, _ = w.Write([]byte("data: " + d + "\n\n"))
			if fl != nil {
				fl.Flush()
			}
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}
	h := AnthropicMessagesHandler(chat)

	body := `{"model":"claude","max_tokens":50,"stream":true,"messages":[{"role":"user","content":"hej"}]}`
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))

	out := rec.Body.String()
	for _, want := range []string{
		"event: message_start", "event: content_block_start",
		"event: content_block_delta", `"text":"Stock"`, `"text":"holm."`,
		"event: content_block_stop", "event: message_delta", `"stop_reason":"end_turn"`,
		"event: message_stop", `"output_tokens":3`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stream missing %q\n---\n%s", want, out)
		}
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q, want text/event-stream", ct)
	}
}

// An upstream error (OpenAI error JSON, non-2xx) becomes an Anthropic error.
func TestAnthropicErrorTranslation(t *testing.T) {
	chat := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"provider_5xx: provider status 502","type":"provider_error"}}`))
	}
	h := AnthropicMessagesHandler(chat)
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/v1/messages",
		strings.NewReader(`{"max_tokens":10,"messages":[{"role":"user","content":"x"}]}`)))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["type"] != "error" {
		t.Fatalf("want error envelope, got %+v", out)
	}
	e, _ := out["error"].(map[string]any)
	if !strings.Contains(e["message"].(string), "502") {
		t.Fatalf("error message not carried: %+v", e)
	}
}

func TestAnthropicRejectsEmptyMessages(t *testing.T) {
	h := AnthropicMessagesHandler(func(w http.ResponseWriter, r *http.Request) { t.Fatal("should not reach chat") })
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"max_tokens":10,"messages":[]}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty messages should be 400, got %d", rec.Code)
	}
}

func TestTextFromContentBlocks(t *testing.T) {
	if got := textFromContent(json.RawMessage(`"hi"`)); got != "hi" {
		t.Fatalf("string content = %q", got)
	}
	if got := textFromContent(json.RawMessage(`[{"type":"text","text":"a"},{"type":"image"},{"type":"text","text":"b"}]`)); got != "ab" {
		t.Fatalf("block content = %q, want ab", got)
	}
}

// Tool definitions and a tool_use/tool_result exchange translate both ways.
func TestAnthropicToolRequestTranslation(t *testing.T) {
	var got map[string]any
	chat := func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Stockholm\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":5,"completion_tokens":7}}`))
	}
	body := `{"model":"claude","max_tokens":200,"tools":[{"name":"get_weather","description":"weather","input_schema":{"type":"object","properties":{"city":{"type":"string"}}}}],"messages":[
	  {"role":"user","content":"weather in Stockholm?"},
	  {"role":"assistant","content":[{"type":"tool_use","id":"call_0","name":"get_weather","input":{"city":"Stockholm"}}]},
	  {"role":"user","content":[{"type":"tool_result","tool_use_id":"call_0","content":"12C"}]}
	]}`
	rec := httptest.NewRecorder()
	AnthropicMessagesHandler(chat)(rec, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))

	// Request: tools translated to OpenAI function tools.
	tools, _ := got["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("want 1 tool, got %+v", got["tools"])
	}
	// Messages: assistant tool_call + a tool-role result message present.
	msgs, _ := got["messages"].([]any)
	var sawToolCall, sawToolMsg bool
	for _, mi := range msgs {
		m := mi.(map[string]any)
		if m["role"] == "assistant" && m["tool_calls"] != nil {
			sawToolCall = true
		}
		if m["role"] == "tool" && m["tool_call_id"] == "call_0" {
			sawToolMsg = true
		}
	}
	if !sawToolCall || !sawToolMsg {
		t.Fatalf("tool_call/%v tool_result/%v not translated: %+v", sawToolCall, sawToolMsg, msgs)
	}

	// Response: tool_calls → Anthropic tool_use block, stop_reason tool_use.
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["stop_reason"] != "tool_use" {
		t.Fatalf("stop_reason = %v, want tool_use", out["stop_reason"])
	}
	content, _ := out["content"].([]any)
	var toolUse map[string]any
	for _, c := range content {
		if m := c.(map[string]any); m["type"] == "tool_use" {
			toolUse = m
		}
	}
	if toolUse == nil || toolUse["name"] != "get_weather" {
		t.Fatalf("no tool_use block: %+v", content)
	}
	if inp, _ := toolUse["input"].(map[string]any); inp["city"] != "Stockholm" {
		t.Fatalf("tool input not parsed: %+v", toolUse["input"])
	}
}

// Streamed tool_calls become Anthropic tool_use SSE blocks.
func TestAnthropicStreamingToolUse(t *testing.T) {
	chat := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		for _, d := range []string{
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_9","function":{"name":"get_weather","arguments":"{\"ci"}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ty\":\"Sthlm\"}"}}]}}]}`,
			`{"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"completion_tokens":9}}`,
		} {
			_, _ = w.Write([]byte("data: " + d + "\n\n"))
			if fl != nil {
				fl.Flush()
			}
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}
	rec := httptest.NewRecorder()
	AnthropicMessagesHandler(chat)(rec, httptest.NewRequest(http.MethodPost, "/v1/messages",
		strings.NewReader(`{"max_tokens":50,"stream":true,"messages":[{"role":"user","content":"weather"}]}`)))
	out := rec.Body.String()
	for _, want := range []string{
		`"type":"tool_use"`, `"name":"get_weather"`, `"index":1`,
		`"type":"input_json_delta"`, `{\"city\":\"Sthlm\"}`,
		`"stop_reason":"tool_use"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stream missing %q\n---\n%s", want, out)
		}
	}
}

// GET /v1/models negotiates shape by the anthropic-version header.
func TestAnthropicModelsNegotiation(t *testing.T) {
	oai := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"auto"}]}`))
	}
	h := AnthropicModelsNegotiator(nil, oai)

	// No header → OpenAI shape (delegated).
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if !strings.Contains(rec.Body.String(), `"object":"list"`) {
		t.Fatalf("without header should delegate to OpenAI handler: %s", rec.Body.String())
	}

	// anthropic-version header → Anthropic shape.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("anthropic-version", "2023-06-01")
	h(rec, req)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := out["data"].([]any)
	if len(data) == 0 {
		t.Fatalf("anthropic models should include at least auto: %s", rec.Body.String())
	}
	first, _ := data[0].(map[string]any)
	if first["type"] != "model" || first["id"] != "auto" {
		t.Fatalf("anthropic model shape wrong: %+v", first)
	}
}
