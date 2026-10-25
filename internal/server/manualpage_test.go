package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/manual"
)

func getManual(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	h := ManualHandler(ManualOptions{PublicURL: "https://sluss.example.eu"})
	mux.HandleFunc("GET /manual", h)
	mux.HandleFunc("GET /manual/{page}", h)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestManualServesEveryPageWithTheInstanceAddress(t *testing.T) {
	for _, p := range manual.Pages {
		path := "/manual/" + p.Slug
		if p.Slug == "index" {
			path = "/manual"
		}
		rec := getManual(t, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, "{{BASE}}") {
			t.Fatalf("%s: placeholder left unreplaced", path)
		}
		if rec.Header().Get("X-Robots-Tag") == "" {
			t.Fatalf("%s: must be noindex", path)
		}
		raw := getManual(t, "/manual/"+p.Slug+".md")
		if raw.Code != http.StatusOK || !strings.HasPrefix(raw.Header().Get("Content-Type"), "text/markdown") || strings.Contains(raw.Body.String(), "{{BASE}}") {
			t.Fatalf("%s.md: status %d type %q", p.Slug, raw.Code, raw.Header().Get("Content-Type"))
		}
	}
	body := getManual(t, "/manual/stop-chatgpt").Body.String()
	if !strings.Contains(body, `href="https://sluss.example.eu/blocked?from=chatgpt.com"`) || !strings.Contains(body, "<ol>") {
		t.Fatalf("links and numbered steps should render:\n%s", body)
	}
	if getManual(t, "/manual/nope").Code != http.StatusNotFound || getManual(t, "/manual/..%2fgo.mod").Code != http.StatusNotFound {
		t.Fatal("unknown pages must 404")
	}
}

// Every internal link in the manual points at a manual page or a console path,
// and every manual page on disk is in the menu.
func TestManualLinksAndMenuAreConsistent(t *testing.T) {
	slugs := map[string]bool{}
	for _, p := range manual.Pages {
		slugs[p.Slug] = true
	}
	files, _ := fs.Glob(manual.Files, "*.md")
	for _, f := range files {
		if !slugs[strings.TrimSuffix(f, ".md")] {
			t.Fatalf("%s is not in manual.Pages", f)
		}
	}
	link := regexp.MustCompile(`\]\((/manual/[a-z-]+)\)`)
	for _, f := range files {
		b, _ := fs.ReadFile(manual.Files, f)
		for _, m := range link.FindAllStringSubmatch(string(b), -1) {
			if !slugs[strings.TrimPrefix(m[1], "/manual/")] {
				t.Fatalf("%s links to missing page %s", f, m[1])
			}
		}
	}
}

func TestMarkdownLinksAreSafe(t *testing.T) {
	out := string(markdownToHTML("See [ok](/router/log), [ext](https://x.eu) and [bad](javascript:alert(1)).\n\n1. one\n2. two"))
	if !strings.Contains(out, `<a href="/router/log">ok</a>`) || !strings.Contains(out, `target="_blank"`) {
		t.Fatalf("safe links should render: %s", out)
	}
	if strings.Contains(out, `href="javascript`) {
		t.Fatalf("javascript: links must not render: %s", out)
	}
	if !strings.Contains(out, "<ol>\n<li>one</li>\n<li>two</li>\n</ol>") {
		t.Fatalf("numbered list: %s", out)
	}
}
