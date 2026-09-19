package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConnectPageShowsInstanceBaseURL(t *testing.T) {
	h := ConnectHandler(LandingOptions{PublicURL: "https://tokenizer.acme.se"})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/connect", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"https://tokenizer.acme.se/v1", // OpenAI base
		"https://tokenizer.acme.se",    // Anthropic base
		"Cursor", "Claude Code", "AnythingLLM", "OpenWebUI",
		"/v1/messages", // the Anthropic shim is referenced
		">auto<",       // model auto
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("connect page missing %q", want)
		}
	}
}

func TestConnectPagePlaceholderWithoutBase(t *testing.T) {
	h := ConnectHandler(LandingOptions{})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/connect", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "ROUTER_PUBLIC_URL") {
		t.Fatal("without a public URL the page should hint to set ROUTER_PUBLIC_URL")
	}
	if !strings.Contains(body, "tokenizer.your-company") {
		t.Fatal("expected a placeholder host in the public/dev view")
	}
}
