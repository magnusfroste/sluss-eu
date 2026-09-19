package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/engine"
	"github.com/magnusfroste/sluss/internal/history"
	"github.com/magnusfroste/sluss/internal/providercfg"
	"github.com/magnusfroste/sluss/internal/registry"
)

func providersTestOpts(t *testing.T) ProvidersOptions {
	t.Helper()
	store, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if _, _, err := providercfg.MigrateAndSeedDB(store, "", ""); err != nil {
		t.Fatalf("seed roster: %v", err)
	}
	snap, err := registry.DefaultSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	regStore, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return ProvidersOptions{
		Engine:  engine.New(regStore),
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version: "test",
		Roster:  store,
	}
}

func TestProvidersAddPersistsToSQLite(t *testing.T) {
	opts := providersTestOpts(t)
	form := url.Values{
		"id":       {"zai"},
		"name":     {"Z.ai"},
		"base_url": {"https://api.z.ai/api/coding/paas/v4"},
		"key_env":  {"ZAI_API_KEY"},
	}
	req := httptest.NewRequest(http.MethodPost, "/router/providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ProvidersAddHandler(opts).ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("add status = %d, want 303", rec.Code)
	}
	ps, _ := opts.Roster.LoadRosterProviders()
	if !hasProvider(ps, "zai") {
		t.Fatalf("provider not persisted: %+v", ps)
	}

	// Delete it again.
	del := url.Values{"id": {"zai"}}
	req = httptest.NewRequest(http.MethodPost, "/router/providers/delete", strings.NewReader(del.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	ProvidersDeleteHandler(opts).ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete status = %d, want 303", rec.Code)
	}
	ps, _ = opts.Roster.LoadRosterProviders()
	if hasProvider(ps, "zai") {
		t.Errorf("provider not deleted: %+v", ps)
	}
}

func TestProvidersAddRejectsBadBaseURL(t *testing.T) {
	opts := providersTestOpts(t)
	form := url.Values{"id": {"bad"}, "base_url": {"ftp://x"}, "key_env": {"K"}}
	req := httptest.NewRequest(http.MethodPost, "/router/providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ProvidersAddHandler(opts).ServeHTTP(rec, req)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=") {
		t.Errorf("expected error redirect, got %q", loc)
	}
	ps, _ := opts.Roster.LoadRosterProviders()
	if hasProvider(ps, "bad") {
		t.Errorf("invalid provider should not be persisted: %+v", ps)
	}
}

func hasProvider(ps []providercfg.Provider, id string) bool {
	for _, p := range ps {
		if p.ID == id {
			return true
		}
	}
	return false
}
