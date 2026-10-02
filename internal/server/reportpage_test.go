package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ISSUE-122: a browser gets the report as a themed page; tools and agents keep
// the Markdown, and ?format=md forces it.
func TestReportContentNegotiation(t *testing.T) {
	h := GapReportHandler(GapReportOptions{})
	get := func(accept, q string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/router/gap-report"+q, nil)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		h(rec, req)
		return rec
	}
	if rec := get("text/html,application/xhtml+xml", ""); !strings.Contains(rec.Header().Get("Content-Type"), "text/html") ||
		!strings.Contains(rec.Body.String(), "Download as Markdown") || !strings.Contains(rec.Body.String(), "<h2>Monitor mode is OFF</h2>") {
		t.Fatalf("browser should get the themed page, got %q", rec.Header().Get("Content-Type"))
	}
	if rec := get("", ""); !strings.Contains(rec.Header().Get("Content-Type"), "text/markdown") || !strings.HasPrefix(rec.Body.String(), "# Shadow AI") {
		t.Fatalf("curl/agents should keep Markdown, got %q", rec.Header().Get("Content-Type"))
	}
	if rec := get("text/html", "?format=md"); !strings.Contains(rec.Header().Get("Content-Type"), "text/markdown") {
		t.Fatal("?format=md must force Markdown even for a browser")
	}
	if rec := get("text/html", "?format=json"); !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatal("?format=json must stay JSON")
	}
}

func TestMarkdownToHTMLCoversReportDialect(t *testing.T) {
	md := "# Title\n\nIntro with **bold** and `code`.\n\n> quoted\n> more\n\n## Numbers\n\n- a — **1**\n- b <script>\n\n| Task | Out |\n|---|---|\n| x | y |\n\n```\nROUTER_X=1\n```\n"
	got := string(markdownToHTML(md))
	for _, want := range []string{"<p>Intro with <strong>bold</strong> and <code>code</code>.</p>", "<blockquote>quoted more</blockquote>", "<h2>Numbers</h2>",
		"<li>a — <strong>1</strong></li>", "<li>b &lt;script&gt;</li>", "<th>Task</th>", "<td>y</td>", "<pre><code>ROUTER_X=1\n</code></pre>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<h1>") {
		t.Error("the H1 belongs to the page header, not the body")
	}
}
