package router

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
)

func TestNewJobDescriptorPrefersAuthenticatedTenantOverUntrustedHints(t *testing.T) {
	req := &openai.ChatRequest{
		Model: "auto",
		Messages: []openai.Message{
			{Role: "user", Content: "hello"},
		},
		Metadata: map[string]any{
			"tenant_id":  "tn_from_metadata",
			"project_id": "prj_from_metadata",
		},
	}
	headers := http.Header{}
	headers.Set("X-Router-Tenant-Id", "tn_from_header")
	headers.Set("X-Router-Project-Id", "prj_from_header")

	got := NewJobDescriptor(JobDescriptorInput{
		RequestID: "req_123",
		Auth: AuthTenantContext{
			TenantID:  "tn_auth",
			ProjectID: "prj_auth",
		},
		Headers: headers,
		Request: req,
	})

	if got.TenantID != "tn_auth" || got.ProjectID != "prj_auth" {
		t.Fatalf("expected authenticated tenant context, got tenant=%q project=%q", got.TenantID, got.ProjectID)
	}
	if got.TenantIDHint != "" || got.ProjectIDHint != "" {
		t.Fatalf("expected tenant/project hints ignored when auth context exists, got tenant_hint=%q project_hint=%q", got.TenantIDHint, got.ProjectIDHint)
	}
}

func TestNewJobDescriptorMapsMetadataAndHeaderHintsAsUntrusted(t *testing.T) {
	maxCompletionTokens := 321
	req := &openai.ChatRequest{
		Model: "auto",
		Messages: []openai.Message{
			{Role: "user", Content: "return json"},
		},
		MaxCompletionTokens: &maxCompletionTokens,
		ResponseFormat: map[string]any{
			"type": "json_schema",
		},
		Metadata: map[string]any{
			"tenant_id":          "tn_hint",
			"project_id":         "prj_hint",
			"latency_preference": "fast",
			"budget_preference":  "cheap",
			"router_mode":        "premium",
			"explicit_model":     "untrusted-metadata-model",
			"trace": map[string]any{
				"tags": []any{"alpha"},
			},
		},
	}
	headers := http.Header{}
	headers.Set("X-Router-Quality-Preference", "high")

	got := NewJobDescriptor(JobDescriptorInput{
		RequestID: "req_hint",
		Headers:   headers,
		Request:   req,
	})

	if got.TenantID != "" || got.ProjectID != "" {
		t.Fatalf("expected no trusted tenant context, got tenant=%q project=%q", got.TenantID, got.ProjectID)
	}
	if got.TenantIDHint != "tn_hint" || got.ProjectIDHint != "prj_hint" {
		t.Fatalf("expected untrusted tenant/project hints, got tenant_hint=%q project_hint=%q", got.TenantIDHint, got.ProjectIDHint)
	}
	if got.LatencyPreference != LatencyFast || got.QualityPreference != QualityHigh || got.BudgetPreference != BudgetCheap {
		t.Fatalf("unexpected preferences: latency=%q quality=%q budget=%q", got.LatencyPreference, got.QualityPreference, got.BudgetPreference)
	}
	if got.RouterMode != RouterModePremium {
		t.Fatalf("expected router mode hint premium, got %q", got.RouterMode)
	}
	if got.ExplicitModel != nil {
		t.Fatalf("untrusted metadata explicit model must not become selected model, got %#v", got.ExplicitModel)
	}
	if got.MaxOutputTokensEstimate != maxCompletionTokens {
		t.Fatalf("expected max completion estimate %d, got %d", maxCompletionTokens, got.MaxOutputTokensEstimate)
	}
	if !got.RequiresJSONSchema {
		t.Fatal("expected response_format to set JSON schema requirement")
	}

	req.Metadata["trace"].(map[string]any)["tags"].([]any)[0] = "mutated"
	if got.Metadata["trace"].(map[string]any)["tags"].([]any)[0] != "alpha" {
		t.Fatalf("descriptor metadata was aliased: %+v", got.Metadata)
	}
}

func TestNewJobDescriptorDefaultValuesAreDeterministic(t *testing.T) {
	got := NewJobDescriptor(JobDescriptorInput{
		RequestID: "req_default",
	})

	if got.TaskType != TaskUnknownHighRisk {
		t.Fatalf("expected default task %q, got %q", TaskUnknownHighRisk, got.TaskType)
	}
	if got.RiskLevel != RiskHigh {
		t.Fatalf("expected default risk %q, got %q", RiskHigh, got.RiskLevel)
	}
	if got.Sensitivity != SensitivityNone {
		t.Fatalf("expected default sensitivity %q, got %q", SensitivityNone, got.Sensitivity)
	}
	if got.LatencyPreference != LatencyBalanced || got.QualityPreference != QualityBalanced || got.BudgetPreference != BudgetNormal {
		t.Fatalf("unexpected default preferences: latency=%q quality=%q budget=%q", got.LatencyPreference, got.QualityPreference, got.BudgetPreference)
	}
	if got.RouterMode != RouterModeAuto {
		t.Fatalf("expected router mode auto, got %q", got.RouterMode)
	}
	if got.ExplicitModel != nil {
		t.Fatalf("expected no explicit model for auto, got %#v", got.ExplicitModel)
	}
}

func TestNewJobDescriptorClientRiskLowRemainsHint(t *testing.T) {
	got := NewJobDescriptor(JobDescriptorInput{
		RequestID: "req_risk",
		Request: &openai.ChatRequest{
			Model: "auto",
			Messages: []openai.Message{
				{Role: "user", Content: "debug auth payment panic"},
			},
			Metadata: map[string]any{
				"risk_level": "low",
			},
		},
	})

	if got.RiskLevel != RiskHigh {
		t.Fatalf("client risk hint must not lower policy truth, got %q", got.RiskLevel)
	}
	if got.RiskLevelHint != RiskLow {
		t.Fatalf("expected low client risk hint, got %q", got.RiskLevelHint)
	}
}

func TestNewJobDescriptorClientRiskHintCanEscalate(t *testing.T) {
	got := NewJobDescriptor(JobDescriptorInput{
		RequestID: "req_critical_hint",
		Request: &openai.ChatRequest{
			Model:    "auto",
			Messages: []openai.Message{{Role: "user", Content: "Say hello."}},
			Metadata: map[string]any{
				"risk_level": "critical",
			},
		},
	})

	if got.RiskLevel != RiskCritical {
		t.Fatalf("client critical risk hint should escalate risk, got risk=%q hint=%q", got.RiskLevel, got.RiskLevelHint)
	}
	if got.RiskLevelHint != RiskCritical {
		t.Fatalf("expected critical risk hint, got %q", got.RiskLevelHint)
	}
}

func TestNewJobDescriptorCopiesSafeFeatureSignals(t *testing.T) {
	got := NewJobDescriptor(JobDescriptorInput{
		RequestID: "req_features",
		Request: &openai.ChatRequest{
			Model: "auto",
			Messages: []openai.Message{{
				Role:    "user",
				Content: "Fix panic in src/auth/session.ts:\n```go\npanic(\"boom\")\n```\nReturn JSON for the production JWT issue.",
			}},
			ResponseFormat: map[string]any{"type": "json_schema"},
		},
	})

	if !got.RequiresCode || !got.RequiresJSONSchema {
		t.Fatalf("expected code and json schema requirements, got %+v", got)
	}
	if got.TaskType != TaskHardCodeDebugging {
		t.Fatalf("expected hard code debugging task from panic signal, got %q", got.TaskType)
	}
	if got.RiskLevel != RiskHigh {
		t.Fatalf("expected high risk from auth/code features, got %q", got.RiskLevel)
	}
	if got.Sensitivity != SensitivitySourceCode || got.SensitivityHint != SensitivitySourceCode {
		t.Fatalf("expected source code sensitivity and hint, got sensitivity=%q hint=%q", got.Sensitivity, got.SensitivityHint)
	}
	assertJobContains(t, got.FilesTouched, "src/auth/session.ts")
	for _, keyword := range []string{"auth", "json_schema"} {
		assertJobContains(t, got.Keywords, keyword)
	}
	payload, err := got.SafeJSON()
	if err != nil {
		t.Fatalf("safe json: %v", err)
	}
	serialized := string(payload)
	if strings.Contains(serialized, "panic(\"boom\")") || strings.Contains(serialized, "production JWT issue") {
		t.Fatalf("safe descriptor projection leaked prompt text: %s", serialized)
	}
	if !strings.Contains(serialized, "src/auth/session.ts") || !strings.Contains(serialized, "json_schema") {
		t.Fatalf("safe descriptor projection omitted derived signals: %s", serialized)
	}
}

func TestJobDescriptorSafeProjectionExcludesPromptAndSecretMetadata(t *testing.T) {
	secretPrompt := "deploy password sk-live-secret from private prompt"
	got := NewJobDescriptor(JobDescriptorInput{
		RequestID: "req_safe",
		Request: &openai.ChatRequest{
			Model: "gpt-test",
			Messages: []openai.Message{
				{Role: "system", Content: "hidden system instruction"},
				{Role: "user", Content: secretPrompt},
			},
			Metadata: map[string]any{
				"api_key":        "sk-secret",
				"explicit_model": "untrusted-secret-model",
				"nested":         map[string]any{"password": "p4ss"},
			},
		},
	})

	payload, err := got.SafeJSON()
	if err != nil {
		t.Fatalf("safe json: %v", err)
	}
	serialized := string(payload)
	for _, forbidden := range []string{"hidden system instruction", secretPrompt, "sk-secret", "p4ss", "api_key", "password", "messages", "untrusted-secret-model", "explicit_model"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("safe projection leaked %q in %s", forbidden, serialized)
		}
	}
	if !json.Valid(payload) {
		t.Fatalf("safe projection is not valid JSON: %s", serialized)
	}
	if !strings.Contains(serialized, `"prompt_tokens_estimate"`) || !strings.Contains(serialized, `"requires_tool_use"`) {
		t.Fatalf("safe projection missing derived fields: %s", serialized)
	}
}

func assertJobContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("expected %q in %#v", want, values)
}
