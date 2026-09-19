package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
