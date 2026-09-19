package server

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/retention"
	"github.com/magnusfroste/sluss/internal/router"
)

func debugLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return logger, &buf
}

func TestLogPromptDisabledByDefault(t *testing.T) {
	logger, buf := debugLogger()
	settings := retention.NewSettings(30, false) // prompt logging off
	cfg := &ChatOptions{Logger: logger, Retention: settings}

	job := &router.JobDescriptor{RequestID: "req_1", TenantID: "tn_1"}
	req := &openai.ChatRequest{Messages: []openai.Message{{Role: "user", Content: "hello"}}}
	cfg.logPrompt(context.Background(), job, req)

	if strings.Contains(buf.String(), "prompt_message") {
		t.Errorf("prompt should not be logged when disabled: %s", buf.String())
	}
}

func TestLogPromptEnabledMasksSecrets(t *testing.T) {
	logger, buf := debugLogger()
	settings := retention.NewSettings(30, true) // prompt logging on globally
	cfg := &ChatOptions{Logger: logger, Retention: settings}

	job := &router.JobDescriptor{RequestID: "req_2", TenantID: "tn_1"}
	secret := "sk-proj-0123456789abcdefghij"
	req := &openai.ChatRequest{Messages: []openai.Message{
		{Role: "user", Content: "my key is " + secret},
	}}
	cfg.logPrompt(context.Background(), job, req)

	out := buf.String()
	if !strings.Contains(out, "prompt_message") {
		t.Fatalf("prompt should be logged when enabled: %s", out)
	}
	if strings.Contains(out, secret) {
		t.Errorf("secret leaked into prompt log: %s", out)
	}
	if !strings.Contains(out, "REDACTED:api_key") {
		t.Errorf("expected masked content in prompt log: %s", out)
	}
}

func TestLogPromptPerTenantOverride(t *testing.T) {
	logger, buf := debugLogger()
	settings := retention.NewSettings(30, true) // on globally
	settings.SetTenant("tn_quiet", retention.TenantSettings{PromptLogging: boolFalse()})
	cfg := &ChatOptions{Logger: logger, Retention: settings}

	job := &router.JobDescriptor{RequestID: "req_3", TenantID: "tn_quiet"}
	req := &openai.ChatRequest{Messages: []openai.Message{{Role: "user", Content: "hi"}}}
	cfg.logPrompt(context.Background(), job, req)

	if strings.Contains(buf.String(), "prompt_message") {
		t.Errorf("tenant override should suppress prompt logging: %s", buf.String())
	}
}

func TestLogPromptNilRetentionIsNoop(t *testing.T) {
	logger, buf := debugLogger()
	cfg := &ChatOptions{Logger: logger} // no retention settings
	cfg.logPrompt(context.Background(),
		&router.JobDescriptor{TenantID: "tn"},
		&openai.ChatRequest{Messages: []openai.Message{{Role: "user", Content: "x"}}})
	if strings.Contains(buf.String(), "prompt_message") {
		t.Error("nil retention settings must not log prompts")
	}
}

func boolFalse() *bool { b := false; return &b }
