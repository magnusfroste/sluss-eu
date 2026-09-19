package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/openai"
)

// EnableThinking maps to vLLM's chat_template_kwargs on the wire; nil sends
// nothing, so providers that don't know the field never see it.
func TestToOpenAIChatTemplateKwargs(t *testing.T) {
	think := false
	req := &NormalizedModelRequest{Model: "m", EnableThinking: &think}
	b, _ := json.Marshal(req.ToOpenAI())
	if !strings.Contains(string(b), `"chat_template_kwargs":{"enable_thinking":false}`) {
		t.Fatalf("body missing enable_thinking=false: %s", b)
	}

	think = true
	b, _ = json.Marshal(req.ToOpenAI())
	if !strings.Contains(string(b), `"enable_thinking":true`) {
		t.Fatalf("body missing enable_thinking=true: %s", b)
	}

	plain := &NormalizedModelRequest{Model: "m"}
	b, _ = json.Marshal(plain.ToOpenAI())
	if strings.Contains(string(b), "chat_template_kwargs") {
		t.Fatalf("nil directive must not emit chat_template_kwargs: %s", b)
	}
}

func TestCloneCopiesThinkingAndTimeout(t *testing.T) {
	think := true
	orig := &NormalizedModelRequest{Model: "m", EnableThinking: &think, TimeoutHint: 90 * time.Second}
	c := orig.Clone()
	if c.EnableThinking == nil || !*c.EnableThinking || c.TimeoutHint != 90*time.Second {
		t.Fatalf("clone lost thinking/timeout: %+v", c)
	}
	// Deep copy: mutating the clone must not touch the original.
	*c.EnableThinking = false
	if !*orig.EnableThinking {
		t.Fatal("clone shares EnableThinking pointer with original")
	}
}

// End-to-end at the adapter: the outbound HTTP body carries the kwargs, and the
// TimeoutHint beats both the adapter default and the hard client timeout.
func TestOpenAIAdapterSendsThinkingDirective(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		// Respond slower than the adapter's default/client timeout (50ms) but
		// well within the request's TimeoutHint.
		time.Sleep(120 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	a := &OpenAIAdapter{
		BaseURL: srv.URL,
		Client:  &http.Client{Timeout: 50 * time.Millisecond},
		Timeout: 50 * time.Millisecond,
	}
	think := false
	req := &NormalizedModelRequest{
		Model:          "qwen36-27b",
		Messages:       []openai.Message{{Role: "user", Content: "hej"}},
		EnableThinking: &think,
		TimeoutHint:    5 * time.Second,
	}
	resp, err := a.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("complete (TimeoutHint should lift the 50ms limits): %v", err)
	}
	if resp.Choices[0].Message.Content != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !strings.Contains(string(gotBody), `"chat_template_kwargs":{"enable_thinking":false}`) {
		t.Fatalf("outbound body missing thinking directive: %s", gotBody)
	}

	// Without a hint, the adapter default applies and the slow server times out.
	req2 := &NormalizedModelRequest{Model: "qwen36-27b", Messages: []openai.Message{{Role: "user", Content: "hej"}}}
	if _, err := a.Complete(context.Background(), req2); err == nil {
		t.Fatal("without TimeoutHint the 50ms default should time out")
	}
}

// On a non-2xx, the adapter logs a WARN carrying the upstream status, Server
// header and a body snippet — so a container log shows a Cloudflare error page
// vs an origin error — while returning the clean typed error to the caller.
func TestOpenAIAdapterLogsUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "cloudflare")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<!DOCTYPE html><title>froste.eu | 502: Bad gateway</title>"))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	a := &OpenAIAdapter{BaseURL: srv.URL, ProviderID: "dgx", Logger: logger,
		Client: &http.Client{Timeout: 5 * time.Second}}

	_, err := a.Complete(context.Background(), &NormalizedModelRequest{
		Model: "qwen36-27b", Messages: []openai.Message{{Role: "user", Content: "hej"}},
	})
	if !errors.Is(err, ErrProvider5xx) {
		t.Fatalf("want provider_5xx, got %v", err)
	}
	log := buf.String()
	for _, want := range []string{"provider_upstream_error", "status=502", "provider_id=dgx", "cloudflare", "Bad gateway"} {
		if !strings.Contains(log, want) {
			t.Fatalf("log missing %q\n---\n%s", want, log)
		}
	}
}
