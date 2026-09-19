package provider

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

func TestNormalizedModelRequestCloneIsolatesNestedToolsAndMetadata(t *testing.T) {
	temperature := 0.2
	maxTokens := 10
	maxCompletionTokens := 20
	req := NormalizeChatRequest(&openai.ChatRequest{
		Model:               "auto",
		Temperature:         &temperature,
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Tools: []any{
			map[string]any{
				"function": map[string]any{
					"name": "search",
				},
			},
		},
		ResponseFormat: map[string]any{
			"type": "json_schema",
		},
		Metadata: map[string]any{
			"trace": map[string]any{
				"tenant": "tn_1",
			},
			"tags": []any{"alpha"},
		},
	})

	clone := req.Clone()
	*clone.Temperature = 0.8
	*clone.MaxTokens = 20
	*clone.MaxCompletionTokens = 40
	clone.Tools[0].(map[string]any)["function"].(map[string]any)["name"] = "mutated"
	clone.ResponseFormat.(map[string]any)["type"] = "text"
	clone.Metadata["trace"].(map[string]any)["tenant"] = "tn_2"
	clone.Metadata["tags"].([]any)[0] = "beta"

	if *req.Temperature != 0.2 {
		t.Fatalf("temperature pointer was shared, got %v", *req.Temperature)
	}
	if *req.MaxTokens != 10 {
		t.Fatalf("max tokens pointer was shared, got %v", *req.MaxTokens)
	}
	if *req.MaxCompletionTokens != 20 {
		t.Fatalf("max completion tokens pointer was shared, got %v", *req.MaxCompletionTokens)
	}
	gotToolName := req.Tools[0].(map[string]any)["function"].(map[string]any)["name"]
	if gotToolName != "search" {
		t.Fatalf("nested tool map was shared, got %v", gotToolName)
	}
	gotFormat := req.ResponseFormat.(map[string]any)["type"]
	if gotFormat != "json_schema" {
		t.Fatalf("response format map was shared, got %v", gotFormat)
	}
	gotTenant := req.Metadata["trace"].(map[string]any)["tenant"]
	if gotTenant != "tn_1" {
		t.Fatalf("nested metadata map was shared, got %v", gotTenant)
	}
	gotTag := req.Metadata["tags"].([]any)[0]
	if gotTag != "alpha" {
		t.Fatalf("nested metadata slice was shared, got %v", gotTag)
	}
}

func TestNormalizeChatRequestIsolatesSourceNestedToolsAndMetadata(t *testing.T) {
	maxCompletionTokens := 20
	req := &openai.ChatRequest{
		Model:               "auto",
		MaxCompletionTokens: &maxCompletionTokens,
		Tools: []any{
			map[string]any{"function": map[string]any{"name": "search"}},
		},
		ResponseFormat: map[string]any{"type": "json_schema"},
		Metadata: map[string]any{
			"trace": map[string]any{"tenant": "tn_1"},
		},
	}

	normalized := NormalizeChatRequest(req)
	*normalized.MaxCompletionTokens = 40
	normalized.Tools[0].(map[string]any)["function"].(map[string]any)["name"] = "mutated"
	normalized.ResponseFormat.(map[string]any)["type"] = "text"
	normalized.Metadata["trace"].(map[string]any)["tenant"] = "tn_2"

	if *req.MaxCompletionTokens != 20 {
		t.Fatalf("source max completion tokens pointer was shared, got %v", *req.MaxCompletionTokens)
	}
	gotToolName := req.Tools[0].(map[string]any)["function"].(map[string]any)["name"]
	if gotToolName != "search" {
		t.Fatalf("source nested tool map was shared, got %v", gotToolName)
	}
	gotFormat := req.ResponseFormat.(map[string]any)["type"]
	if gotFormat != "json_schema" {
		t.Fatalf("source response format map was shared, got %v", gotFormat)
	}
	gotTenant := req.Metadata["trace"].(map[string]any)["tenant"]
	if gotTenant != "tn_1" {
		t.Fatalf("source nested metadata map was shared, got %v", gotTenant)
	}
}

func TestNormalizedModelRequestToOpenAIPreservesNewRequestFields(t *testing.T) {
	maxCompletionTokens := 99
	req := &NormalizedModelRequest{
		Model:               "auto",
		MaxCompletionTokens: &maxCompletionTokens,
		ResponseFormat:      map[string]any{"type": "json_schema"},
	}

	out := req.ToOpenAI()
	if out.MaxCompletionTokens == nil || *out.MaxCompletionTokens != 99 {
		t.Fatalf("expected max_completion_tokens preserved, got %#v", out.MaxCompletionTokens)
	}
	if got := out.ResponseFormat.(map[string]any)["type"]; got != "json_schema" {
		t.Fatalf("expected response_format preserved, got %v", got)
	}

	*out.MaxCompletionTokens = 11
	out.ResponseFormat.(map[string]any)["type"] = "text"
	if *req.MaxCompletionTokens != 99 {
		t.Fatalf("to openai shared max completion pointer, got %v", *req.MaxCompletionTokens)
	}
	if got := req.ResponseFormat.(map[string]any)["type"]; got != "json_schema" {
		t.Fatalf("to openai shared response format map, got %v", got)
	}
}
