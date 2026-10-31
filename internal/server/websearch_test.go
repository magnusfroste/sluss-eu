package server

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/audit"
)

// fakeSearx answers like SearXNG's JSON API and records the query it got.
func fakeSearx(t *testing.T, status int, results int) (*httptest.Server, *string) {
	t.Helper()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("q")
		if r.URL.Query().Get("format") != "json" {
			t.Errorf("format=%q, want json", r.URL.Query().Get("format"))
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		var rs []map[string]string
		for i := 0; i < results; i++ {
			rs = append(rs, map[string]string{"title": "Result " + string(rune('A'+i)), "url": "https://example.org/" + string(rune('a'+i)), "content": "snippet " + string(rune('a'+i))})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": rs})
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

// capture is the downstream chat handler: it records the forwarded body.
func capture(body *[]byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*body = b
		w.WriteHeader(http.StatusOK)
	})
}

func sendChat(h http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/chat/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestWebSearchInjectsResultsAndSources(t *testing.T) {
	srv, got := fakeSearx(t, http.StatusOK, 7)
	sink := audit.NewMemorySink(0)
	var fwd []byte
	h := SearchOptions{URL: srv.URL + "/search", Auditor: sink}.Middleware(capture(&fwd))
	rec := sendChat(h, `{"model":"auto","stream":true,"search":true,"messages":[{"role":"user","content":"what is the EU AI Act timeline"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if *got != "what is the EU AI Act timeline" {
		t.Fatalf("search engine got %q", *got)
	}
	if h := rec.Header().Get(hdrSearch); h != "done;5" {
		t.Fatalf("%s = %q, want done;5 (capped)", hdrSearch, h)
	}
	raw, err := base64.RawURLEncoding.DecodeString(rec.Header().Get(hdrSources))
	if err != nil {
		t.Fatalf("sources header not base64url: %v", err)
	}
	var src []searchSource
	if err := json.Unmarshal(raw, &src); err != nil || len(src) != 5 || src[0].URL != "https://example.org/a" {
		t.Fatalf("sources = %v err=%v", src, err)
	}
	// The forwarded request: a system message first, the user's message intact, "search" gone.
	var out struct {
		Messages []struct{ Role, Content string }
		Search   *bool `json:"search"`
		Model    string
	}
	if err := json.Unmarshal(fwd, &out); err != nil {
		t.Fatalf("forwarded body: %v\n%s", err, fwd)
	}
	if len(out.Messages) != 2 || out.Messages[0].Role != "system" || !strings.Contains(out.Messages[0].Content, "[1] Result A — https://example.org/a") || out.Messages[1].Content != "what is the EU AI Act timeline" {
		t.Fatalf("forwarded messages wrong: %+v", out.Messages)
	}
	if out.Search != nil || out.Model != "auto" {
		t.Fatalf("search flag should be stripped and the model kept: %+v", out)
	}
	var audited bool
	for _, e := range sink.Entries() {
		if e.Action == ActionChatSearch && e.Detail["results"] == "5" && !strings.Contains(e.Reason+e.Target, "AI Act") {
			audited = true
		}
	}
	if !audited {
		t.Fatalf("expected a chat.search audit entry without the query, got %+v", sink.Entries())
	}
}

func TestWebSearchSkippedForSensitivePrompt(t *testing.T) {
	srv, got := fakeSearx(t, http.StatusOK, 3)
	sink := audit.NewMemorySink(0)
	var fwd []byte
	h := SearchOptions{URL: srv.URL, Auditor: sink}.Middleware(capture(&fwd))
	body := `{"search":true,"messages":[{"role":"user","content":"Summarise the case for customer 811218-9876 who complained about an invoice"}]}`
	rec := sendChat(h, body)
	if *got != "" {
		t.Fatalf("a PII prompt must never reach the search engine, got %q", *got)
	}
	if h := rec.Header().Get(hdrSearch); !strings.HasPrefix(h, "skipped-sensitive;pii") {
		t.Fatalf("%s = %q", hdrSearch, h)
	}
	if string(fwd) != body {
		t.Fatalf("body must be forwarded unchanged when skipped:\n%s", fwd)
	}
	var blocked bool
	for _, e := range sink.Entries() {
		if e.Action == ActionChatSearch && e.Outcome == audit.OutcomeBlocked && !strings.Contains(e.Reason, "811218") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("expected a blocked chat.search entry without the prompt, got %+v", sink.Entries())
	}
}

func TestWebSearchOffFailedAndEmpty(t *testing.T) {
	var fwd []byte
	// Not configured: the chat still works, the header says off.
	rec := sendChat(SearchOptions{}.Middleware(capture(&fwd)), `{"search":true,"messages":[{"role":"user","content":"hello"}]}`)
	if rec.Code != http.StatusOK || rec.Header().Get(hdrSearch) != "off" {
		t.Fatalf("off: status %d header %q", rec.Code, rec.Header().Get(hdrSearch))
	}
	// Not asked: no header at all, body untouched.
	rec = sendChat(SearchOptions{URL: "http://127.0.0.1:1"}.Middleware(capture(&fwd)), `{"messages":[{"role":"user","content":"hello"}]}`)
	if rec.Header().Get(hdrSearch) != "" || !strings.Contains(string(fwd), `"hello"`) {
		t.Fatalf("not asked: header %q body %s", rec.Header().Get(hdrSearch), fwd)
	}
	// Engine down: failed, chat continues.
	srv, _ := fakeSearx(t, http.StatusBadGateway, 0)
	rec = sendChat(SearchOptions{URL: srv.URL}.Middleware(capture(&fwd)), `{"search":true,"messages":[{"role":"user","content":"hello"}]}`)
	if rec.Code != http.StatusOK || rec.Header().Get(hdrSearch) != "failed" {
		t.Fatalf("failed: status %d header %q", rec.Code, rec.Header().Get(hdrSearch))
	}
	// Nothing found: empty.
	srv2, _ := fakeSearx(t, http.StatusOK, 0)
	rec = sendChat(SearchOptions{URL: srv2.URL}.Middleware(capture(&fwd)), `{"search":true,"messages":[{"role":"user","content":"hello"}]}`)
	if rec.Header().Get(hdrSearch) != "empty" {
		t.Fatalf("empty: header %q", rec.Header().Get(hdrSearch))
	}
}

func TestWebSearchKeepsArrayContentIntact(t *testing.T) {
	srv, got := fakeSearx(t, http.StatusOK, 1)
	var fwd []byte
	h := SearchOptions{URL: srv.URL}.Middleware(capture(&fwd))
	body := `{"search":true,"messages":[{"role":"user","content":[{"type":"text","text":"compare these"},{"type":"text","text":"--- Attached file: notes.md ---\nsome notes"}]}]}`
	sendChat(h, body)
	if *got != "compare these--- Attached file: notes.md ---\nsome notes" && !strings.HasPrefix(*got, "compare these") {
		t.Fatalf("query should be the concatenated text, got %q", *got)
	}
	if !strings.Contains(string(fwd), `"type":"text","text":"--- Attached file: notes.md ---\nsome notes"`) {
		t.Fatalf("array content must be forwarded byte-for-byte:\n%s", fwd)
	}
}

func TestChatPageVariantsByRole(t *testing.T) {
	render := func(role, user, dept string) string {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/chat", nil)
		ctx := withDepartment(withRole(withAdminUser(req.Context(), user), role), dept)
		DemoChatPageHandler(true)(rec, req.WithContext(ctx))
		return rec.Body.String()
	}
	emp := render(roleUser, "anna@example.eu", "finance")
	if strings.Contains(emp, `href="/router/dashboard"`) || strings.Contains(emp, `id="modelsel"`) {
		t.Fatal("an employee must not see the admin nav or the model picker")
	}
	for _, want := range []string{"anna@example.eu", "finance", "/router/logout", "const EMPLOYEE=true,SEARCH_ON=true,WHO=\"anna@example.eu\",DEPT=\"finance\"", "company AI"} {
		if !strings.Contains(emp, want) {
			t.Fatalf("employee page lacks %q", want)
		}
	}
	adm := render(roleAdmin, "ops", "")
	if !strings.Contains(adm, `href="/router/dashboard"`) || !strings.Contains(adm, `id="modelsel"`) || !strings.Contains(adm, "const EMPLOYEE=false") || strings.Contains(adm, "/router/logout\">Sign out</a></div>") {
		t.Fatal("admin page should keep nav and picker and carry no identity line")
	}
	// A name from the identity provider must not be able to end the script.
	if evil := render(roleUser, "x</script><script>alert(1)//", "d"); strings.Contains(evil, "x</script><script>alert(1)") {
		t.Fatal("WHO must be escaped inside the script")
	}
	guest := render(roleDemo, "demo-guest", "")
	if strings.Contains(guest, `href="/router/dashboard"`) || !strings.Contains(guest, "live routing demo") {
		t.Fatal("demo guest page should stay the focused demo")
	}
	rec := httptest.NewRecorder()
	DemoChatPageHandler(false)(rec, httptest.NewRequest(http.MethodGet, "/chat", nil))
	if !strings.Contains(rec.Body.String(), "SEARCH_ON=false") {
		t.Fatal("search flag should follow configuration")
	}
}
