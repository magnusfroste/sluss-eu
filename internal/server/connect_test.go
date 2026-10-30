package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConnectPageShowsInstanceBaseURL(t *testing.T) {
	h := ConnectHandler(LandingOptions{PublicURL: "https://sluss.acme.se"})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/connect", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"https://sluss.acme.se/v1", // OpenAI base
		"https://sluss.acme.se",    // Anthropic base
		"Cursor", "Claude Code", "AnythingLLM", "OpenWebUI",
		"/v1/messages", // the Anthropic shim is referenced
		">auto<",       // model auto
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("connect page missing %q", want)
		}
	}
}

func TestConnectPageUsesRequestHostWithoutBase(t *testing.T) {
	h := ConnectHandler(LandingOptions{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/connect", nil)
	req.Host = "sluss.internal:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	h(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "ROUTER_PUBLIC_URL") {
		t.Fatal("without a public URL the page should hint to set ROUTER_PUBLIC_URL")
	}
	if !strings.Contains(body, "https://sluss.internal:8080/v1") {
		t.Fatal("expected the address the page was opened with")
	}
	if strings.Contains(body, "tokenizer") {
		t.Fatal("the pre-rename name is back")
	}
}

func TestConsoleConnectGroupsClientsWithSnippets(t *testing.T) {
	rec := httptest.NewRecorder()
	ConsoleConnectHandler("https://demo.sluss.eu", nil)(rec, httptest.NewRequest(http.MethodGet, "/router/connect", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"Connect clients", `class="tk-shell"`, `href="/router/connect"`, // console frame, Rollout tab
		"Editors", "Coding agents", "Chat apps", "SDKs",
		"export ANTHROPIC_BASE_URL=https://demo.sluss.eu",
		"https://demo.sluss.eu/v1/chat/completions", // the curl check
		`href="/router/keys"`, `id="key"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("console connect page missing %q", want)
		}
	}
	if strings.Contains(body, "ROUTER_PUBLIC_URL if clients") {
		t.Error("with a public URL the page should not hint to set one")
	}
}
