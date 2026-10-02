package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/policy"
	"github.com/magnusfroste/sluss/internal/provider"
)

// ISSUE-122: the whole conversation is classified, so a harmless follow-up
// after a personal ID still routes locally. The response says whether the
// sensitivity sits in the latest message or earlier in the conversation.
func TestSensitivitySourceHeader(t *testing.T) {
	eng := localTaggedEngine(t) // premium-reasoning carries "local", so PII routes instead of blocking
	snap, err := eng.Registry.Active()
	if err != nil {
		t.Fatal(err)
	}
	cache, err := policy.NewRuntimeCache(snap, "builtin:nis2-baseline")
	if err != nil {
		t.Fatal(err)
	}
	h := ChatCompletionsHandler(&provider.MockAdapter{BaseURL: "http://127.0.0.1:1"}, ChatOptions{
		Engine: eng, PolicyCache: cache,
		Adapters: map[string]provider.Adapter{"openai": &provider.MockAdapter{BaseURL: "http://127.0.0.1:1"}, "anthropic": &provider.MockAdapter{BaseURL: "http://127.0.0.1:1"}},
	})
	send := func(body string) http.Header {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))
		return rec.Header()
	}
	pii := `{"role":"user","content":"Summarise the case for customer 811218-9876 who complained about an invoice"}`
	if got := send(`{"model":"auto","messages":[` + pii + `]}`).Get("X-Router-Sensitivity-Source"); got != "" {
		t.Fatalf("single message: header should be absent, got %q", got)
	}
	followUp := `{"model":"auto","messages":[` + pii + `,{"role":"assistant","content":"Done."},{"role":"user","content":"now write a concise git commit message for a bugfix"}]}`
	if got := send(followUp).Get("X-Router-Sensitivity-Source"); got != "conversation" {
		t.Fatalf("follow-up after PII: want conversation, got %q", got)
	}
	again := `{"model":"auto","messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"hi"},` + pii + `]}`
	if got := send(again).Get("X-Router-Sensitivity-Source"); got != "message" {
		t.Fatalf("PII in latest message: want message, got %q", got)
	}
}
