package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
)

func TestOpenAIAdapterMapsRequestHeadersAndBaseURL(t *testing.T) {
	temperature := 0.4
	maxTokens := 128
	var got openai.ChatRequest

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/compatible/v1/chat/completions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if gotAuth := r.Header.Get("Authorization"); gotAuth != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", gotAuth)
		}
		if gotContentType := r.Header.Get("Content-Type"); gotContentType != "application/json" {
			t.Fatalf("unexpected content type %q", gotContentType)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(openai.ChatResponse{
			ID:    "chatcmpl_test",
			Model: "gpt-provider-4o-mini",
			Choices: []openai.Choice{{
				Index:        0,
				Message:      openai.Message{Role: "assistant", Content: "ok"},
				FinishReason: "stop",
			}},
			Usage: openai.Usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
		})
	}))
	defer providerServer.Close()

	adapter := &OpenAIAdapter{
		BaseURL: providerServer.URL + "/compatible/",
		APIKey:  "test-key",
	}
	resp, err := adapter.Complete(context.Background(), &NormalizedModelRequest{
		Model:       "gpt-provider-4o-mini",
		Messages:    []openai.Message{{Role: "user", Content: "hello"}},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
		Stream:      true,
		Tools: []any{
			map[string]any{"type": "function", "function": map[string]any{"name": "lookup"}},
		},
		Metadata: map[string]any{"tenant": "tn_1"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if resp.ID != "chatcmpl_test" || resp.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if got.Model != "gpt-provider-4o-mini" {
		t.Fatalf("adapter must use supplied provider model id exactly, got %q", got.Model)
	}
	if len(got.Messages) != 1 || got.Messages[0].Content != "hello" {
		t.Fatalf("messages not mapped: %+v", got.Messages)
	}
	if got.Temperature == nil || *got.Temperature != temperature {
		t.Fatalf("temperature not mapped: %v", got.Temperature)
	}
	if got.MaxTokens == nil || *got.MaxTokens != maxTokens {
		t.Fatalf("max_tokens not mapped: %v", got.MaxTokens)
	}
	if !got.Stream {
		t.Fatalf("stream flag not mapped")
	}
	if len(got.Tools) != 1 {
		t.Fatalf("tools not mapped: %+v", got.Tools)
	}
	if got.Metadata["tenant"] != "tn_1" {
		t.Fatalf("metadata not mapped: %+v", got.Metadata)
	}
}

func TestOpenAIAdapterStreamForwardsSSEDataFrames(t *testing.T) {
	var got openai.ChatRequest
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/compatible/v1/chat/completions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if gotAccept := r.Header.Get("Accept"); gotAccept != "text/event-stream" {
			t.Fatalf("unexpected accept header %q", gotAccept)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chunk_1\"}\n\n"))
		_, _ = w.Write([]byte(": keepalive\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"chunk_2\"}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer providerServer.Close()

	adapter := &OpenAIAdapter{BaseURL: providerServer.URL + "/compatible/"}
	chunks, err := adapter.Stream(context.Background(), &NormalizedModelRequest{
		Model:    "provider-model",
		Messages: []openai.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Stream returned setup error: %v", err)
	}

	var gotChunks []StreamChunk
	for chunk := range chunks {
		gotChunks = append(gotChunks, chunk)
	}
	if !got.Stream {
		t.Fatal("stream flag not forced on outbound request")
	}
	if len(gotChunks) != 3 {
		t.Fatalf("expected 3 chunks, got %+v", gotChunks)
	}
	if string(gotChunks[0].Data) != `{"id":"chunk_1"}` || string(gotChunks[1].Data) != `{"id":"chunk_2"}` {
		t.Fatalf("unexpected data chunks: %+v", gotChunks)
	}
	if !gotChunks[2].Done {
		t.Fatalf("expected final done chunk, got %+v", gotChunks[2])
	}
}

func TestOpenAIAdapterStreamEOFBeforeDoneIsInterrupted(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chunk_1\"}\n\n"))
	}))
	defer providerServer.Close()

	adapter := &OpenAIAdapter{BaseURL: providerServer.URL}
	chunks, err := adapter.Stream(context.Background(), &NormalizedModelRequest{Model: "provider-model"})
	if err != nil {
		t.Fatalf("Stream returned setup error: %v", err)
	}

	var got []StreamChunk
	for chunk := range chunks {
		got = append(got, chunk)
	}
	if len(got) != 2 {
		t.Fatalf("expected data chunk plus interruption, got %+v", got)
	}
	if string(got[0].Data) != `{"id":"chunk_1"}` {
		t.Fatalf("unexpected first chunk: %+v", got[0])
	}
	if !errors.Is(got[1].Err, ErrStreamInterrupted) {
		t.Fatalf("expected stream interruption, got %+v", got[1])
	}
}

func TestOpenAIAdapterStreamSetupErrorMapping(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(openai.ErrorEnvelope{Error: openai.ErrorBody{Type: "rate_limit_error"}})
	}))
	defer providerServer.Close()

	adapter := &OpenAIAdapter{BaseURL: providerServer.URL}
	_, err := adapter.Stream(context.Background(), &NormalizedModelRequest{Model: "provider-model"})
	if !errors.Is(err, ErrProviderRateLimit) {
		t.Fatalf("expected rate limit setup error, got %v", err)
	}
}

func TestOpenAIAdapterAcceptsVersionedBaseURL(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(openai.ChatResponse{ID: "chatcmpl_test"})
	}))
	defer providerServer.Close()

	adapter := &OpenAIAdapter{BaseURL: providerServer.URL + "/v1"}
	if _, err := adapter.Complete(context.Background(), &NormalizedModelRequest{Model: "provider-model"}); err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
}

func TestOpenAIAdapterStatusErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   openai.ErrorEnvelope
		want   error
	}{
		{
			name:   "bad request",
			status: http.StatusBadRequest,
			body:   openai.ErrorEnvelope{Error: openai.ErrorBody{Type: "invalid_request_error"}},
			want:   ErrProviderBadReq,
		},
		{
			name:   "unauthorized",
			status: http.StatusUnauthorized,
			body:   openai.ErrorEnvelope{Error: openai.ErrorBody{Type: "invalid_api_key"}},
			want:   ErrProviderAuth,
		},
		{
			name:   "forbidden",
			status: http.StatusForbidden,
			body:   openai.ErrorEnvelope{Error: openai.ErrorBody{Type: "insufficient_permissions"}},
			want:   ErrProviderAuth,
		},
		{
			name:   "rate limit",
			status: http.StatusTooManyRequests,
			body:   openai.ErrorEnvelope{Error: openai.ErrorBody{Type: "rate_limit_error"}},
			want:   ErrProviderRateLimit,
		},
		{
			name:   "5xx",
			status: http.StatusBadGateway,
			body:   openai.ErrorEnvelope{Error: openai.ErrorBody{Type: "server_error"}},
			want:   ErrProvider5xx,
		},
		{
			name:   "model unavailable body",
			status: http.StatusBadRequest,
			body: openai.ErrorEnvelope{Error: openai.ErrorBody{
				Type: "invalid_request_error",
				Code: "model_not_found",
			}},
			want: ErrModelUnavailable,
		},
		{
			name:   "model unavailable 404",
			status: http.StatusNotFound,
			body:   openai.ErrorEnvelope{Error: openai.ErrorBody{Type: "not_found_error"}},
			want:   ErrModelUnavailable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(tc.body)
			}))
			defer providerServer.Close()

			adapter := &OpenAIAdapter{BaseURL: providerServer.URL}
			_, err := adapter.Complete(context.Background(), &NormalizedModelRequest{Model: "provider-model"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestOpenAIAdapterTimeoutMapping(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(openai.ChatResponse{ID: "late"})
	}))
	defer providerServer.Close()

	adapter := &OpenAIAdapter{
		BaseURL: providerServer.URL,
		Timeout: 5 * time.Millisecond,
	}
	_, err := adapter.Complete(context.Background(), &NormalizedModelRequest{Model: "provider-model"})
	if !errors.Is(err, ErrProviderTimeout) {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestOpenAIAdapterUsageNormalizationAllowsMissingFields(t *testing.T) {
	tests := []struct {
		name string
		resp string
		want openai.Usage
	}{
		{
			name: "full usage",
			resp: `{"id":"chatcmpl_usage","model":"provider-model","choices":[],"usage":{"prompt_tokens":7,"completion_tokens":5,"total_tokens":12}}`,
			want: openai.Usage{PromptTokens: 7, CompletionTokens: 5, TotalTokens: 12},
		},
		{
			name: "missing usage",
			resp: `{"id":"chatcmpl_usage","model":"provider-model","choices":[]}`,
			want: openai.Usage{},
		},
		{
			name: "partial usage",
			resp: `{"id":"chatcmpl_usage","model":"provider-model","choices":[],"usage":{"prompt_tokens":7}}`,
			want: openai.Usage{PromptTokens: 7},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.resp))
			}))
			defer providerServer.Close()

			adapter := &OpenAIAdapter{BaseURL: providerServer.URL}
			got, err := adapter.Complete(context.Background(), &NormalizedModelRequest{Model: "provider-model"})
			if err != nil {
				t.Fatalf("Complete returned error: %v", err)
			}
			if got.Usage != tc.want {
				t.Fatalf("unexpected usage: got %+v want %+v", got.Usage, tc.want)
			}
		})
	}
}

func TestOpenAIAdapterBadJSONMapsToBadResponse(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	defer providerServer.Close()

	adapter := &OpenAIAdapter{BaseURL: providerServer.URL}
	_, err := adapter.Complete(context.Background(), &NormalizedModelRequest{Model: "provider-model"})
	if !errors.Is(err, ErrProviderBadResp) {
		t.Fatalf("expected bad response, got %v", err)
	}
}

func TestOpenAIAdapterAttributionHeaders(t *testing.T) {
	var referer, title string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		referer = r.Header.Get("HTTP-Referer")
		title = r.Header.Get("X-Title")
		_ = json.NewEncoder(w).Encode(openai.ChatResponse{ID: "x"})
	}))
	defer srv.Close()

	req := func() *NormalizedModelRequest {
		return &NormalizedModelRequest{Model: "m", Messages: []openai.Message{{Role: "user", Content: "hi"}}}
	}

	// Set → headers sent.
	a := &OpenAIAdapter{BaseURL: srv.URL + "/v1", APIKey: "k", Referer: "https://example.test", Title: "tokenizer"}
	if _, err := a.Complete(context.Background(), req()); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if referer != "https://example.test" || title != "tokenizer" {
		t.Fatalf("attribution headers not sent: referer=%q title=%q", referer, title)
	}

	// Unset → headers omitted (native OpenAI/Anthropic paths unaffected).
	referer, title = "sentinel", "sentinel"
	b := &OpenAIAdapter{BaseURL: srv.URL + "/v1", APIKey: "k"}
	if _, err := b.Complete(context.Background(), req()); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if referer != "" || title != "" {
		t.Fatalf("attribution headers should be omitted when unset: referer=%q title=%q", referer, title)
	}
}
