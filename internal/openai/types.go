// Package openai contains the minimal OpenAI-compatible request and response
// shapes that the router accepts on its public API and emits to clients.
package openai

import (
	"encoding/json"
	"errors"
	"strings"
)

type ChatRequest struct {
	Model               string         `json:"model"`
	Messages            []Message      `json:"messages"`
	Temperature         *float64       `json:"temperature,omitempty"`
	MaxTokens           *int           `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int           `json:"max_completion_tokens,omitempty"`
	Stream              bool           `json:"stream,omitempty"`
	StreamOptions       *StreamOptions `json:"stream_options,omitempty"`
	Tools               []any          `json:"tools,omitempty"`
	ResponseFormat      any            `json:"response_format,omitempty"`
	Metadata            map[string]any `json:"metadata,omitempty"`
	// ChatTemplateKwargs is the vLLM extension for per-request chat-template
	// switches (notably {"enable_thinking": bool} on Qwen3-style models). Only
	// the router sets it — and only for models flagged reasoning-capable — so
	// providers that don't know the field never see it.
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
}

// StreamOptions mirrors the OpenAI/OpenRouter streaming options. IncludeUsage
// asks the provider to emit a final chunk carrying token usage so the router can
// account real tokens/cost for streamed requests.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// ContentParts preserves the ORIGINAL array-form content (the multi-part
	// format vision clients and several SDKs send) so the provider receives
	// exactly what the client sent. Nil for plain string content. When set,
	// Content holds the concatenated text parts — which is what classification,
	// masking and logging read: an "attached" document is message text, and the
	// classifier scans all of it (ISSUE-102).
	ContentParts json.RawMessage `json:"-"`
	// NonTextParts marks that the message carried at least one non-text part
	// (image_url etc.) — the attachment signal. It sets requires_vision on the
	// job, so policy can confine or block what rules cannot read.
	NonTextParts bool `json:"-"`
	// Tool-calling fields (used by the Anthropic /v1/messages shim and any
	// tool-using client). Omitted when empty so plain chat is unaffected.
	ToolCalls  []any  `json:"tool_calls,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

// contentPart is the array-form content element: {"type":"text","text":...} or
// a non-text part such as {"type":"image_url",...}. Unknown fields are kept in
// the raw array (ContentParts) — we only need type/text for classification.
type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// UnmarshalJSON accepts both string content and array-form content parts.
// String stays the fast path. For arrays: text parts are concatenated into
// Content (so the full document text is classified), any non-text part sets
// NonTextParts, and the untouched array is kept in ContentParts for the
// provider call.
func (m *Message) UnmarshalJSON(data []byte) error {
	type alias Message
	aux := struct {
		Content json.RawMessage `json:"content"`
		*alias
	}{alias: (*alias)(m)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	raw := bytesTrim(aux.Content)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if raw[0] == '"' {
		return json.Unmarshal(raw, &m.Content)
	}
	if raw[0] != '[' {
		return errors.New("message content must be a string or an array of content parts")
	}
	var parts []contentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return errors.New("message content parts must be objects with a type field")
	}
	var texts []string
	for _, p := range parts {
		if p.Type == "text" {
			if p.Text != "" {
				texts = append(texts, p.Text)
			}
		} else {
			m.NonTextParts = true
		}
	}
	m.Content = strings.Join(texts, "\n")
	m.ContentParts = append(json.RawMessage(nil), raw...)
	return nil
}

// MarshalJSON re-emits the original content parts when present, so the
// provider sees exactly what the client sent; plain string content otherwise.
func (m Message) MarshalJSON() ([]byte, error) {
	type alias Message
	aux := struct {
		Content json.RawMessage `json:"content"`
		alias
	}{alias: alias(m)}
	if len(m.ContentParts) > 0 {
		aux.Content = m.ContentParts
	} else {
		b, err := json.Marshal(m.Content)
		if err != nil {
			return nil, err
		}
		aux.Content = b
	}
	return json.Marshal(aux)
}

func bytesTrim(b []byte) []byte {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	return b[i:]
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// ModelObject is one entry in an OpenAI-style GET /v1/models response.
type ModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"` // always "model"
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelList is the OpenAI-style GET /v1/models response envelope.
type ModelList struct {
	Object string        `json:"object"` // always "list"
	Data   []ModelObject `json:"data"`
}
