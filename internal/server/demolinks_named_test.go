package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/magnusfroste/sluss/internal/audit"
	"github.com/magnusfroste/sluss/internal/history"
)

func namedLinksStore(t *testing.T) *history.Store {
	t.Helper()
	s, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// A named link opens the demo, attributes the session/audit to its label, and
// counts the open; disabling revokes exactly that link.
func TestNamedDemoLinkFlow(t *testing.T) {
	store := namedLinksStore(t)
	mem := audit.NewMemorySink(10)
	a := &AdminAuth{Sessions: NewSessionStore(nil), Store: store}

	tok, _ := GenerateDemoShareToken()
	if err := store.CreateDemoLink(tok, "Acme AB", time.Now()); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Open via the named link.
	rec := httptest.NewRecorder()
	DemoEntryHandler(a, store, mem)(rec, httptest.NewRequest(http.MethodGet, "/demo?token="+tok, nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("named link should open the demo, got %d", rec.Code)
	}
	entries := mem.Entries()
	if len(entries) == 0 || entries[len(entries)-1].Actor != "demo:Acme AB" {
		t.Fatalf("audit should attribute the open to the label, got %+v", entries)
	}

	// The open is counted with a timestamp.
	links, _ := store.ListDemoLinks()
	if len(links) != 1 || links[0].Opens != 1 || links[0].LastOpenAt.IsZero() {
		t.Fatalf("open not counted: %+v", links)
	}

	// Disable → the same token no longer works (fail closed to login).
	if ok, _ := store.DisableDemoLink(tok, time.Now()); !ok {
		t.Fatal("disable should succeed")
	}
	rec2 := httptest.NewRecorder()
	DemoEntryHandler(a, store, mem)(rec2, httptest.NewRequest(http.MethodGet, "/demo?token="+tok, nil))
	if rec2.Code != http.StatusFound || rec2.Header().Get("Location") != "/router/login" {
		t.Fatalf("disabled link must fail closed, got %d %q", rec2.Code, rec2.Header().Get("Location"))
	}
}

// The legacy single shared token still works alongside named links.
func TestLegacySharedTokenStillWorks(t *testing.T) {
	store := namedLinksStore(t)
	a := &AdminAuth{Sessions: NewSessionStore(nil), Store: store}
	_ = SetDemoShareToken(store, "legacy-token")
	rec := httptest.NewRecorder()
	DemoEntryHandler(a, store, nil)(rec, httptest.NewRequest(http.MethodGet, "/demo?token=legacy-token", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/chat" {
		t.Fatalf("legacy token should still open the demo, got %d", rec.Code)
	}
}

// MCP: create_demo_link with a label creates a named link; demo_link_usage
// lists it with opens — and never returns the token.
func TestMCPNamedDemoLink(t *testing.T) {
	store := namedLinksStore(t)
	o := MCPOptions{History: store, WriteEnabled: true, PublicURL: "https://t.example.com", Version: "t"}
	// Engine isn't needed for these two tools.
	created := callTool(t, o, "create_demo_link", map[string]any{"label": "Beta Corp"})
	url, _ := created["url"].(string)
	if !strings.HasPrefix(url, "https://t.example.com/demo?token=") {
		t.Fatalf("unexpected url: %q", url)
	}
	if created["label"] != "Beta Corp" {
		t.Fatalf("label not echoed: %+v", created)
	}

	usage := callTool(t, o, "demo_link_usage", map[string]any{})
	named, _ := usage["named_links"].([]any)
	if len(named) != 1 {
		t.Fatalf("want 1 named link, got %+v", usage)
	}
	first, _ := named[0].(map[string]any)
	if first["label"] != "Beta Corp" || first["active"] != true {
		t.Fatalf("named link wrong: %+v", first)
	}
	if _, hasToken := first["token"]; hasToken {
		t.Fatal("usage must not leak tokens")
	}
}
