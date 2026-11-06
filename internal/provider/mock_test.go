package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

func TestMockAdapterUsesDefaultClientWhenClientIsNil(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(openai.ChatResponse{
			ID:    "chatcmpl_test",
			Model: "auto",
			Choices: []openai.Choice{{
				Message: openai.Message{Role: "assistant", Content: "ok"},
			}},
		})
	}))
	defer providerServer.Close()

	adapter := &MockAdapter{BaseURL: providerServer.URL}
	resp, err := adapter.Complete(context.Background(), &NormalizedModelRequest{Model: "auto"})
	if err != nil {
		t.Fatalf("expected nil-client adapter to use default client, got err=%v", err)
	}
	if resp.ID != "chatcmpl_test" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

// With no BaseURL the mock answers in-process: a container that ships only the
// router answers before any provider is configured, and the text says so.
func TestMockAdapterAnswersInProcessWithoutBaseURL(t *testing.T) {
	m := &MockAdapter{}
	resp, err := m.Complete(context.Background(), &NormalizedModelRequest{
		Model: "auto", Messages: []openai.Message{{Role: "user", Content: "hello there"}},
	})
	if err != nil {
		t.Fatalf("in-process mock failed: %v", err)
	}
	if len(resp.Choices) != 1 || !strings.Contains(resp.Choices[0].Message.Content, "no real model is connected") {
		t.Fatalf("unexpected answer: %+v", resp)
	}
	if resp.Usage.PromptTokens == 0 || resp.Usage.TotalTokens <= resp.Usage.PromptTokens {
		t.Errorf("usage should be filled: %+v", resp.Usage)
	}
}

// The mock streams too, so the chat renders its answer like any provider's.
func TestMockAdapterStreamsItsAnswer(t *testing.T) {
	m := &MockAdapter{}
	ch, err := m.Stream(context.Background(), &NormalizedModelRequest{Model: "auto", Messages: []openai.Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var chunks []StreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}
	if len(chunks) != 3 || !chunks[2].Done {
		t.Fatalf("want content, final and done chunks, got %+v", chunks)
	}
	if !strings.Contains(string(chunks[0].Data), `"delta":{"role":"assistant","content":"Test answer`) {
		t.Errorf("first chunk should carry the answer as a delta: %s", chunks[0].Data)
	}
	if !strings.Contains(string(chunks[1].Data), `"finish_reason":"stop"`) || chunks[1].InputTokens == 0 {
		t.Errorf("final chunk should carry finish_reason and usage: %s (in=%d)", chunks[1].Data, chunks[1].InputTokens)
	}
}
