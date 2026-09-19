package server

// Anthropic Messages API shim (POST /v1/messages). Anthropic-native clients —
// Claude Code, Codex-style tools, the Anthropic SDKs — speak the Messages wire
// format, not OpenAI's. This middleware translates a Messages request into the
// internal OpenAI-shaped request, runs it through the SAME routing/policy/
// provider path as /v1/chat/completions, and translates the response (JSON or
// SSE stream) back into Messages format. So "point Claude Code at your
// compliance router" just works.
//
// Scope (v1): text content in and out, streaming and non-streaming. Tool use,
// images and extended thinking are out of scope and pass through as text only.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/registry"
)

// --- Messages wire types (minimal) --------------------------------------------

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Messages    []anthropicMessage `json:"messages"`
	System      json.RawMessage    `json:"system,omitempty"` // string or []block
	Stream      bool               `json:"stream,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string OR []block
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// anthropicBlock is one content block (text / tool_use / tool_result).
type anthropicBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
	// tool_use
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	// tool_result
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"` // string or []block
}

// textFromContent extracts the concatenated text from an Anthropic content field
// that is either a plain string or an array of typed blocks (text blocks only).
func textFromContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var b strings.Builder
		for _, bl := range blocks {
			if bl.Type == "text" {
				b.WriteString(bl.Text)
			}
		}
		return b.String()
	}
	return ""
}

// toOpenAI maps a Messages request onto the internal OpenAI chat request. The
// model is set to "auto" so the router decides (the client's model name is a
// hint at most); system becomes a leading system message; tools and tool
// exchanges are translated so tool-using clients (Claude Code, Codex) work.
func (a *anthropicRequest) toOpenAI() *openai.ChatRequest {
	msgs := make([]openai.Message, 0, len(a.Messages)+1)
	if sys := textFromContent(a.System); sys != "" {
		msgs = append(msgs, openai.Message{Role: "system", Content: sys})
	}
	for _, m := range a.Messages {
		msgs = append(msgs, translateMessage(m)...)
	}
	out := &openai.ChatRequest{Model: "auto", Messages: msgs, Stream: a.Stream, Temperature: a.Temperature}
	if a.MaxTokens > 0 {
		mt := a.MaxTokens
		out.MaxTokens = &mt
	}
	if tools := translateTools(a.Tools); len(tools) > 0 {
		out.Tools = tools
	}
	return out
}

// translateMessage expands one Anthropic message into one or more OpenAI
// messages: text stays as content, assistant tool_use blocks become tool_calls,
// and user tool_result blocks become separate tool-role messages.
func translateMessage(m anthropicMessage) []openai.Message {
	role := m.Role
	if role != "user" && role != "assistant" && role != "system" {
		role = "user"
	}
	// Plain string content → a single message.
	var s string
	if json.Unmarshal(m.Content, &s) == nil {
		return []openai.Message{{Role: role, Content: s}}
	}
	var blocks []anthropicBlock
	if json.Unmarshal(m.Content, &blocks) != nil {
		return []openai.Message{{Role: role, Content: textFromContent(m.Content)}}
	}
	var text strings.Builder
	var toolCalls []any
	var toolResults []openai.Message
	for _, b := range blocks {
		switch b.Type {
		case "text":
			text.WriteString(b.Text)
		case "tool_use":
			args := string(b.Input)
			if strings.TrimSpace(args) == "" {
				args = "{}"
			}
			toolCalls = append(toolCalls, map[string]any{
				"id": b.ID, "type": "function",
				"function": map[string]any{"name": b.Name, "arguments": args},
			})
		case "tool_result":
			toolResults = append(toolResults, openai.Message{
				Role: "tool", ToolCallID: b.ToolUseID, Content: textFromContent(b.Content),
			})
		}
	}
	var out []openai.Message
	// Tool results answer the previous assistant turn — emit them first.
	out = append(out, toolResults...)
	if text.Len() > 0 || len(toolCalls) > 0 {
		msg := openai.Message{Role: role, Content: text.String()}
		if len(toolCalls) > 0 {
			msg.Role = "assistant"
			msg.ToolCalls = toolCalls
		}
		out = append(out, msg)
	}
	return out
}

// translateTools maps Anthropic tool definitions to OpenAI function tools.
func translateTools(tools []anthropicTool) []any {
	if len(tools) == 0 {
		return nil
	}
	out := make([]any, 0, len(tools))
	for _, t := range tools {
		fn := map[string]any{"name": t.Name}
		if t.Description != "" {
			fn["description"] = t.Description
		}
		if len(t.InputSchema) > 0 {
			fn["parameters"] = json.RawMessage(t.InputSchema)
		}
		out = append(out, map[string]any{"type": "function", "function": fn})
	}
	return out
}

// stopReasonFor maps an OpenAI finish_reason to an Anthropic stop_reason.
func stopReasonFor(finish string) string {
	switch finish {
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "stop_sequence"
	default:
		return "end_turn"
	}
}

// AnthropicMessagesHandler wraps the OpenAI chat handler with request/response
// translation so POST /v1/messages behaves like the Anthropic Messages API.
func AnthropicMessagesHandler(chat http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON body")
			return
		}
		if len(req.Messages) == 0 {
			writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "messages cannot be empty")
			return
		}
		body, err := json.Marshal(req.toOpenAI())
		if err != nil {
			writeAnthropicError(w, http.StatusInternalServerError, "api_error", "translation failed")
			return
		}
		// Re-enter the shared chat pipeline with an OpenAI-shaped body, capturing
		// its output through a translating writer.
		r2 := r.Clone(r.Context())
		r2.Body = nopCloser{bytes.NewReader(body)}
		r2.ContentLength = int64(len(body))
		aw := &anthropicWriter{w: w, stream: req.Stream, model: req.Model}
		chat(aw, r2)
		aw.finish()
	}
}

// --- translating response writer ----------------------------------------------

type anthropicWriter struct {
	w      http.ResponseWriter
	stream bool
	model  string

	status  int
	wrote   bool
	buf     bytes.Buffer // non-stream body / error body
	isSSE   bool
	sseLine bytes.Buffer // partial SSE line accumulator (stream)
	started bool         // emitted message_start + content_block_start
	stopped bool         // emitted the closing events
	outTok  int
	// streamed tool calls, accumulated by index until the stream ends.
	toolOrder []int
	tools     map[int]*streamTool
}

type streamTool struct {
	id, name string
	args     strings.Builder
}

func (aw *anthropicWriter) Header() http.Header { return aw.w.Header() }

func (aw *anthropicWriter) WriteHeader(status int) {
	if aw.wrote {
		return
	}
	aw.wrote = true
	aw.status = status
	aw.isSSE = strings.HasPrefix(aw.w.Header().Get("Content-Type"), "text/event-stream")
	if aw.isSSE && status == http.StatusOK {
		// Anthropic clients expect the SSE content type + an immediate message_start.
		aw.w.Header().Set("Content-Type", "text/event-stream")
		aw.w.WriteHeader(status)
		aw.emitStart()
		return
	}
	// Non-stream success and error bodies are buffered and translated in finish().
}

func (aw *anthropicWriter) Write(p []byte) (int, error) {
	if !aw.wrote {
		aw.WriteHeader(http.StatusOK)
	}
	if aw.isSSE && aw.status == http.StatusOK {
		return len(p), aw.consumeSSE(p)
	}
	return aw.buf.Write(p)
}

func (aw *anthropicWriter) Flush() {
	if f, ok := aw.w.(http.Flusher); ok {
		f.Flush()
	}
}

// emitStart writes message_start + content_block_start (once).
func (aw *anthropicWriter) emitStart() {
	if aw.started {
		return
	}
	aw.started = true
	aw.sse("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": "msg_stream", "type": "message", "role": "assistant",
			"model": aw.model, "content": []any{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": 0, "output_tokens": 0},
		},
	})
	aw.sse("content_block_start", map[string]any{
		"type": "content_block_start", "index": 0,
		"content_block": map[string]any{"type": "text", "text": ""},
	})
	aw.Flush()
}

// consumeSSE parses OpenAI SSE bytes (possibly partial) and emits Anthropic events.
func (aw *anthropicWriter) consumeSSE(p []byte) error {
	aw.sseLine.Write(p)
	data := aw.sseLine.Bytes()
	for {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimSpace(string(data[:idx]))
		data = data[idx+1:]
		aw.handleSSELine(line)
	}
	rest := append([]byte(nil), data...)
	aw.sseLine.Reset()
	aw.sseLine.Write(rest)
	return nil
}

func (aw *anthropicWriter) handleSSELine(line string) {
	if !strings.HasPrefix(line, "data:") {
		return
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "" {
		return
	}
	if payload == "[DONE]" {
		aw.emitStop()
		return
	}
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Index    int    `json:"index"`
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal([]byte(payload), &chunk) != nil {
		return
	}
	if chunk.Usage != nil && chunk.Usage.CompletionTokens > 0 {
		aw.outTok = chunk.Usage.CompletionTokens
	}
	for _, c := range chunk.Choices {
		if c.Delta.Content != "" {
			aw.sse("content_block_delta", map[string]any{
				"type": "content_block_delta", "index": 0,
				"delta": map[string]any{"type": "text_delta", "text": c.Delta.Content},
			})
			aw.Flush()
		}
		for _, tc := range c.Delta.ToolCalls {
			if aw.tools == nil {
				aw.tools = map[int]*streamTool{}
			}
			st := aw.tools[tc.Index]
			if st == nil {
				st = &streamTool{}
				aw.tools[tc.Index] = st
				aw.toolOrder = append(aw.toolOrder, tc.Index)
			}
			if tc.ID != "" {
				st.id = tc.ID
			}
			if tc.Function.Name != "" {
				st.name = tc.Function.Name
			}
			st.args.WriteString(tc.Function.Arguments)
		}
	}
}

// emitStop writes content_block_stop + message_delta + message_stop (once).
func (aw *anthropicWriter) emitStop() {
	if aw.stopped {
		return
	}
	aw.stopped = true
	// Close the text block (index 0), then emit any accumulated tool_use blocks.
	aw.sse("content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
	stop := "end_turn"
	idx := 1
	for _, i := range aw.toolOrder {
		st := aw.tools[i]
		aw.sse("content_block_start", map[string]any{
			"type": "content_block_start", "index": idx,
			"content_block": map[string]any{"type": "tool_use", "id": st.id, "name": st.name, "input": map[string]any{}},
		})
		if args := st.args.String(); args != "" {
			aw.sse("content_block_delta", map[string]any{
				"type": "content_block_delta", "index": idx,
				"delta": map[string]any{"type": "input_json_delta", "partial_json": args},
			})
		}
		aw.sse("content_block_stop", map[string]any{"type": "content_block_stop", "index": idx})
		idx++
		stop = "tool_use"
	}
	aw.sse("message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stop, "stop_sequence": nil},
		"usage": map[string]any{"output_tokens": aw.outTok},
	})
	aw.sse("message_stop", map[string]any{"type": "message_stop"})
	aw.Flush()
}

// sse writes one Anthropic SSE event.
func (aw *anthropicWriter) sse(event string, data any) {
	b, _ := json.Marshal(data)
	_, _ = aw.w.Write([]byte("event: " + event + "\ndata: " + string(b) + "\n\n"))
}

// finish is called after the wrapped handler returns: it closes a stream, or
// translates a buffered non-stream / error body.
func (aw *anthropicWriter) finish() {
	if aw.isSSE && aw.status == http.StatusOK {
		aw.emitStop() // in case the upstream ended without an explicit [DONE]
		return
	}
	if aw.status == 0 {
		aw.status = http.StatusOK
	}
	if aw.status >= 400 {
		aw.translateError()
		return
	}
	aw.translateJSON()
}

func (aw *anthropicWriter) translateJSON() {
	var resp openai.ChatResponse
	if err := json.Unmarshal(aw.buf.Bytes(), &resp); err != nil {
		writeAnthropicError(aw.w, http.StatusBadGateway, "api_error", "upstream response was not valid")
		return
	}
	var content []any
	stop := "end_turn"
	if len(resp.Choices) > 0 {
		msg := resp.Choices[0].Message
		stop = stopReasonFor(resp.Choices[0].FinishReason)
		if msg.Content != "" {
			content = append(content, map[string]any{"type": "text", "text": msg.Content})
		}
		for _, tc := range toolCallsFromAny(msg.ToolCalls) {
			content = append(content, map[string]any{
				"type": "tool_use", "id": tc.ID, "name": tc.Name, "input": rawJSONObject(tc.Arguments),
			})
			stop = "tool_use"
		}
	}
	if len(content) == 0 {
		content = append(content, map[string]any{"type": "text", "text": ""})
	}
	model := aw.model
	if resp.Model != "" {
		model = resp.Model
	}
	out := map[string]any{
		"id": firstNonEmpty(resp.ID, "msg_reply"), "type": "message", "role": "assistant",
		"model": model, "content": content, "stop_reason": stop, "stop_sequence": nil,
		"usage": map[string]any{"input_tokens": resp.Usage.PromptTokens, "output_tokens": resp.Usage.CompletionTokens},
	}
	aw.w.Header().Set("Content-Type", "application/json")
	aw.w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(aw.w).Encode(out)
}

// oaiToolCall is the minimal shape read back from an OpenAI tool_calls entry.
type oaiToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (t oaiToolCall) named() oaiToolCallFlat {
	return oaiToolCallFlat{ID: t.ID, Name: t.Function.Name, Arguments: t.Function.Arguments}
}

type oaiToolCallFlat struct {
	ID, Name, Arguments string
}

// toolCallsFromAny re-decodes the []any tool_calls into a typed, flat slice.
func toolCallsFromAny(in []any) []oaiToolCallFlat {
	if len(in) == 0 {
		return nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	var raw []oaiToolCall
	if json.Unmarshal(b, &raw) != nil {
		return nil
	}
	out := make([]oaiToolCallFlat, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.named())
	}
	return out
}

// rawJSONObject returns the tool arguments as a JSON object value, or {} when
// they are missing/invalid (Anthropic's tool_use.input must be an object).
func rawJSONObject(args string) json.RawMessage {
	args = strings.TrimSpace(args)
	if args == "" || !json.Valid([]byte(args)) {
		return json.RawMessage("{}")
	}
	return json.RawMessage(args)
}

func (aw *anthropicWriter) translateError() {
	var oe openai.ErrorEnvelope
	msg := "request failed"
	if json.Unmarshal(aw.buf.Bytes(), &oe) == nil && oe.Error.Message != "" {
		msg = oe.Error.Message
	}
	writeAnthropicError(aw.w, aw.status, anthropicErrorType(aw.status), msg)
}

func anthropicErrorType(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request_error"
	case http.StatusUnauthorized:
		return "authentication_error"
	case http.StatusForbidden:
		return "permission_error"
	case http.StatusNotFound:
		return "not_found_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	default:
		return "api_error"
	}
}

func writeAnthropicError(w http.ResponseWriter, status int, typ, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":  "error",
		"error": map[string]any{"type": typ, "message": msg},
	})
}

// AnthropicModelsNegotiator serves the model list in Anthropic shape when the
// caller sends an anthropic-version header (the Anthropic SDK / Claude Code),
// and delegates to the OpenAI-shaped handler otherwise — same path, right shape
// for each client.
func AnthropicModelsNegotiator(eng *engine.Engine, openaiHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("anthropic-version") == "" {
			openaiHandler(w, r)
			return
		}
		data := []any{}
		if eng != nil && eng.Registry != nil {
			if snap, err := eng.Registry.Active(); err == nil {
				for _, m := range snap.EnabledModelsWithCapabilities(registry.Capabilities{}) {
					data = append(data, map[string]any{"type": "model", "id": m.ID, "display_name": m.ID})
				}
			}
		}
		data = append([]any{map[string]any{"type": "model", "id": "auto", "display_name": "Sluss (auto-route)"}}, data...)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "has_more": false})
	}
}

// nopCloser adapts a byte reader to an io.ReadCloser for the re-entered request.
type nopCloser struct{ *bytes.Reader }

func (nopCloser) Close() error { return nil }
