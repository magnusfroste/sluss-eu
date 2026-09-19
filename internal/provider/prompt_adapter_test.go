package provider

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

func TestPromptAdapterApplySupportsModelAndProfileSpecificSystemPromptMutations(t *testing.T) {
	adapter := &PromptAdapter{
		Enabled: true,
		ModelProfiles: map[string]string{
			"premium-reasoning": "premium",
		},
		Rules: []PromptAdapterRule{
			{
				Name: "cheap-model-prefix",
				Match: PromptAdapterMatch{
					ModelIDs: []string{"cheap-general"},
				},
				Mutation: SystemPromptMutation{Prefix: "[cheap] "},
			},
			{
				Name: "premium-profile-suffix",
				Match: PromptAdapterMatch{
					Profiles: []string{"premium"},
				},
				Mutation: SystemPromptMutation{Suffix: " [premium]"},
			},
		},
	}

	t.Run("model-specific", func(t *testing.T) {
		req := &NormalizedModelRequest{
			Model: "gpt-4.1-mini",
			Messages: []openai.Message{
				{Role: "system", Content: "baseline"},
				{Role: "user", Content: "hello"},
				{Role: "system", Content: "guardrails"},
			},
		}

		got, result := adapter.Apply(req, PromptAdapterContext{ModelID: "cheap-general", ProviderModelID: "gpt-4.1-mini"})
		if got == nil {
			t.Fatal("expected adapted request")
		}
		if len(result.AppliedRules) != 1 || result.AppliedRules[0] != "cheap-model-prefix" {
			t.Fatalf("expected exact model rule applied, got %+v", result.AppliedRules)
		}
		if got.Messages[0].Content != "[cheap] baseline" {
			t.Fatalf("expected first system message to get prefix, got %q", got.Messages[0].Content)
		}
		if got.Messages[1].Content != "hello" {
			t.Fatalf("expected user message unchanged, got %q", got.Messages[1].Content)
		}
		if got.Messages[2].Content != "guardrails" {
			t.Fatalf("expected trailing system message unchanged, got %q", got.Messages[2].Content)
		}
		if req.Messages[0].Content != "baseline" {
			t.Fatalf("expected original request unchanged, got %q", req.Messages[0].Content)
		}
	})

	t.Run("profile-specific", func(t *testing.T) {
		req := &NormalizedModelRequest{
			Model: "claude-sonnet-4",
			Messages: []openai.Message{
				{Role: "system", Content: "baseline"},
				{Role: "user", Content: "hello"},
				{Role: "system", Content: "guardrails"},
			},
		}

		got, result := adapter.Apply(req, PromptAdapterContext{ModelID: "premium-reasoning", ProviderModelID: "claude-sonnet-4"})
		if got == nil {
			t.Fatal("expected adapted request")
		}
		if len(result.AppliedRules) != 1 || result.AppliedRules[0] != "premium-profile-suffix" {
			t.Fatalf("expected premium profile rule applied, got %+v", result.AppliedRules)
		}
		if got.Messages[0].Content != "baseline" {
			t.Fatalf("expected first system message unchanged, got %q", got.Messages[0].Content)
		}
		if got.Messages[2].Content != "guardrails [premium]" {
			t.Fatalf("expected last system message to get suffix, got %q", got.Messages[2].Content)
		}
		if req.Messages[2].Content != "guardrails" {
			t.Fatalf("expected original request unchanged, got %q", req.Messages[2].Content)
		}
	})
}

func TestPromptAdapterApplyDisabledByDefaultAndNonTargetsUnchanged(t *testing.T) {
	adapter := &PromptAdapter{
		Rules: []PromptAdapterRule{
			{
				Name: "cheap-model-prefix",
				Match: PromptAdapterMatch{
					ModelIDs: []string{"cheap-general"},
				},
				Mutation: SystemPromptMutation{Prefix: "[cheap] "},
			},
		},
	}
	req := &NormalizedModelRequest{
		Model: "gpt-4.1-mini",
		Messages: []openai.Message{
			{Role: "system", Content: "baseline"},
			{Role: "user", Content: "hello"},
		},
	}

	got, result := adapter.Apply(req, PromptAdapterContext{ModelID: "cheap-general", ProviderModelID: "gpt-4.1-mini"})
	if got != nil {
		t.Fatalf("expected disabled adapter to skip mutations, got %+v", got.Messages)
	}
	if len(result.AppliedRules) != 0 {
		t.Fatalf("expected no applied rules when disabled, got %+v", result.AppliedRules)
	}

	adapter.Enabled = true
	got, result = adapter.Apply(req, PromptAdapterContext{ModelID: "balanced-coder", ProviderModelID: "gpt-4.1"})
	if got != nil {
		t.Fatalf("expected unmatched request unchanged, got %+v", got.Messages)
	}
	if len(result.AppliedRules) != 0 {
		t.Fatalf("expected no applied rules for non-target, got %+v", result.AppliedRules)
	}
	if req.Messages[0].Content != "baseline" {
		t.Fatalf("expected original request unchanged, got %q", req.Messages[0].Content)
	}
}
