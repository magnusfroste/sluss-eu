package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/history"
)

func newHistory(t *testing.T) *history.Store {
	t.Helper()
	h, err := history.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	t.Cleanup(func() { h.Close() })
	return h
}

// The curated default set must include the PII→local showcase — the demo's star.
func TestDefaultQuickPromptsIncludePII(t *testing.T) {
	var pii bool
	for _, p := range defaultQuickPrompts() {
		if strings.Contains(p.Text, "811218-9876") {
			pii = true
			if !strings.Contains(strings.ToLower(p.Note), "local") {
				t.Fatalf("PII prompt note should explain the local routing: %q", p.Note)
			}
		}
	}
	if !pii {
		t.Fatal("default prompts must include the synthetic-personnummer showcase")
	}
}

func TestSeedThenLoadFromDB(t *testing.T) {
	h := newHistory(t)
	// Empty store → LoadQuickPrompts falls back to defaults.
	if got := LoadQuickPrompts(h); len(got) != len(defaultQuickPrompts()) {
		t.Fatalf("empty store load = %d, want defaults", len(got))
	}
	SeedQuickPrompts(h)
	if v, ok := h.KVGet(quickPromptsKey); !ok || v == "" {
		t.Fatal("seed did not persist prompts to the DB")
	}
	// Seed is idempotent: a second seed must not overwrite edits.
	custom := []QuickPrompt{{Label: "x", Text: "y"}}
	if err := SaveQuickPrompts(h, custom); err != nil {
		t.Fatalf("save: %v", err)
	}
	SeedQuickPrompts(h)
	if got := LoadQuickPrompts(h); len(got) != 1 || got[0].Label != "x" {
		t.Fatalf("seed overwrote existing prompts: %+v", got)
	}
}

func TestQuickPromptsHandlerJSON(t *testing.T) {
	h := newHistory(t)
	SeedQuickPrompts(h)
	rec := httptest.NewRecorder()
	QuickPromptsHandler(h)(rec, httptest.NewRequest(http.MethodGet, "/chat/quickprompts", nil))
	var got []QuickPrompt
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("handler returned no prompts")
	}
}

func TestPromptsAddAndDelete(t *testing.T) {
	h := newHistory(t)
	SeedQuickPrompts(h)
	n0 := len(LoadQuickPrompts(h))

	// Add.
	form := url.Values{"label": {"Ny"}, "tier": {"billig"}, "text": {"säg hej"}, "note": {"test"}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/router/prompts", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	PromptsAddHandler(h)(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("add status=%d", rr.Code)
	}
	if got := LoadQuickPrompts(h); len(got) != n0+1 || got[len(got)-1].Label != "Ny" {
		t.Fatalf("add failed: %+v", got)
	}

	// Delete the one we added (last index).
	del := url.Values{"index": {strconv.Itoa(n0)}}
	dr := httptest.NewRecorder()
	dreq := httptest.NewRequest(http.MethodPost, "/router/prompts/delete", strings.NewReader(del.Encode()))
	dreq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	PromptsDeleteHandler(h)(dr, dreq)
	if got := LoadQuickPrompts(h); len(got) != n0 {
		t.Fatalf("delete failed: len=%d want %d", len(got), n0)
	}
}

func TestPromptsAddRequiresLabelAndText(t *testing.T) {
	h := newHistory(t)
	form := url.Values{"label": {""}, "text": {""}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/router/prompts", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	PromptsAddHandler(h)(rr, req)
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "err=") {
		t.Fatalf("empty add should redirect with error, got %q", loc)
	}
}

func TestPromptsReadOnlyWithoutStore(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/router/prompts", strings.NewReader("label=x&text=y"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	PromptsAddHandler(nil)(rr, req)
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "err=") {
		t.Fatalf("nil store add should redirect with error, got %q", loc)
	}
	// Page still renders the built-in defaults, marked read-only.
	pr := httptest.NewRecorder()
	PromptsPageHandler(nil, "")(pr, httptest.NewRequest(http.MethodGet, "/router/prompts", nil))
	if !strings.Contains(pr.Body.String(), "Read-only mode") {
		t.Fatal("read-only banner missing")
	}
}
