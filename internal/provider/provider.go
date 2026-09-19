// Package provider contains the provider-adapter interface and the in-tree
// adapters.
package provider

import (
	"context"
	"errors"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
)

type Adapter interface {
	Name() string
	Complete(ctx context.Context, req *NormalizedModelRequest) (*openai.ChatResponse, error)
}

type StreamingAdapter interface {
	Stream(ctx context.Context, req *NormalizedModelRequest) (<-chan StreamChunk, error)
}

type StreamChunk struct {
	Data []byte
	Done bool
	Err  error
	// InputTokens/OutputTokens carry usage from the provider's final usage chunk
	// (when stream_options.include_usage was requested); 0 on content chunks.
	InputTokens  int
	OutputTokens int
}

// NormalizedModelRequest is the provider-neutral request shape used between
// routing/policy/context processing and concrete provider adapters.
type NormalizedModelRequest struct {
	Model               string
	Messages            []openai.Message
	Temperature         *float64
	MaxTokens           *int
	MaxCompletionTokens *int
	Stream              bool
	Tools               []any
	ResponseFormat      any
	Metadata            map[string]any
	// EnableThinking, when non-nil, asks a reasoning-capable model to think (true)
	// or answer directly (false) via vLLM's chat_template_kwargs. Router-set only
	// (from the job's requires_reasoning); nil sends nothing, so ordinary models
	// and providers are unaffected.
	EnableThinking *bool
	// TimeoutHint, when >0, overrides the adapter's default per-request timeout.
	// The router raises it for reasoning-capable models so a long think isn't cut
	// off mid-stream.
	TimeoutHint time.Duration
}

func NormalizeChatRequest(req *openai.ChatRequest) *NormalizedModelRequest {
	if req == nil {
		return &NormalizedModelRequest{}
	}
	return &NormalizedModelRequest{
		Model:               req.Model,
		Messages:            append([]openai.Message(nil), req.Messages...),
		Temperature:         cloneFloat64Ptr(req.Temperature),
		MaxTokens:           cloneIntPtr(req.MaxTokens),
		MaxCompletionTokens: cloneIntPtr(req.MaxCompletionTokens),
		Stream:              req.Stream,
		Tools:               cloneAnySlice(req.Tools),
		ResponseFormat:      cloneAny(req.ResponseFormat),
		Metadata:            cloneMetadata(req.Metadata),
	}
}

func (r *NormalizedModelRequest) ToOpenAI() *openai.ChatRequest {
	if r == nil {
		return &openai.ChatRequest{}
	}
	out := &openai.ChatRequest{
		Model:               r.Model,
		Messages:            append([]openai.Message(nil), r.Messages...),
		Temperature:         cloneFloat64Ptr(r.Temperature),
		MaxTokens:           cloneIntPtr(r.MaxTokens),
		MaxCompletionTokens: cloneIntPtr(r.MaxCompletionTokens),
		Stream:              r.Stream,
		Tools:               cloneAnySlice(r.Tools),
		ResponseFormat:      cloneAny(r.ResponseFormat),
		Metadata:            cloneMetadata(r.Metadata),
	}
	if r.EnableThinking != nil {
		out.ChatTemplateKwargs = map[string]any{"enable_thinking": *r.EnableThinking}
	}
	return out
}

func (r *NormalizedModelRequest) Clone() *NormalizedModelRequest {
	if r == nil {
		return &NormalizedModelRequest{}
	}
	return &NormalizedModelRequest{
		Model:               r.Model,
		Messages:            append([]openai.Message(nil), r.Messages...),
		Temperature:         cloneFloat64Ptr(r.Temperature),
		MaxTokens:           cloneIntPtr(r.MaxTokens),
		MaxCompletionTokens: cloneIntPtr(r.MaxCompletionTokens),
		Stream:              r.Stream,
		Tools:               cloneAnySlice(r.Tools),
		ResponseFormat:      cloneAny(r.ResponseFormat),
		Metadata:            cloneMetadata(r.Metadata),
		EnableThinking:      cloneBoolPtr(r.EnableThinking),
		TimeoutHint:         r.TimeoutHint,
	}
}

func cloneMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAnySlice(in []any) []any {
	if len(in) == 0 {
		return nil
	}
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = cloneAny(v)
	}
	return out
}

func cloneAny(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return cloneMetadata(typed)
	case []any:
		return cloneAnySlice(typed)
	case map[string]string:
		out := make(map[string]string, len(typed))
		for k, value := range typed {
			out[k] = value
		}
		return out
	case []string:
		return append([]string(nil), typed...)
	case []map[string]any:
		out := make([]map[string]any, len(typed))
		for i, value := range typed {
			out[i] = cloneMetadata(value)
		}
		return out
	default:
		return v
	}
}

func cloneFloat64Ptr(in *float64) *float64 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneIntPtr(in *int) *int {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

var (
	ErrProviderTimeout   = errors.New("provider_timeout")
	ErrProviderRateLimit = errors.New("provider_rate_limit")
	ErrProviderAuth      = errors.New("provider_auth_error")
	ErrProvider5xx       = errors.New("provider_5xx")
	ErrProviderBadReq    = errors.New("provider_bad_request")
	ErrModelUnavailable  = errors.New("model_unavailable")
	ErrProviderBadResp   = errors.New("provider_bad_response")
	ErrStreamInterrupted = errors.New("stream_interrupted")
)
