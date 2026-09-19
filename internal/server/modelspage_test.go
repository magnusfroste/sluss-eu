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

// modelsTestOpts opens a SQLite-backed roster (history.db) in a temp dir, seeds
// the built-ins into it (ISSUE-073), and builds a ModelsOptions whose live
// registry snapshot is derived from that roster.
func modelsTestOpts(t *testing.T) ModelsOptions {
	t.Helper()
	store, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if _, _, err := providercfg.MigrateAndSeedDB(store, "", ""); err != nil {
		t.Fatalf("seed roster: %v", err)
	}
	provs, _ := store.LoadRosterProviders()
	models, _ := store.LoadRosterModels()
	regProvs, regModels := providercfg.RegistryEntries(provs, models)
	snap, err := registry.NewSnapshot(registry.Definition{
		RegistryVersion: "test", Providers: regProvs, Models: regModels,
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	regStore, err := registry.NewStore(snap)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	return ModelsOptions{
		Engine:  engine.New(regStore),
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version: "test",
		Roster:  store,
	}
}

func TestModelsPageRendersRoster(t *testing.T) {
	opts := modelsTestOpts(t)
	rec := httptest.NewRecorder()
	ModelsPageHandler(opts).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/router/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"cheap-general", "balanced-coder", "premium-reasoning", "Models", "Add model"} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered page missing %q", want)
		}
	}
}

func TestModelsAddPersistsAndReTiers(t *testing.T) {
	opts := modelsTestOpts(t)

	// Add a new custom model referencing the seeded openrouter connection.
	form := url.Values{
		"id":                  {"glm-fast"},
		"provider_id":         {"openrouter"},
		"provider_model_id":   {"z-ai/glm-4.6"},
		"tier":                {"cheap"},
		"input_usd_per_mtok":  {"0.6"},
		"output_usd_per_mtok": {"2.2"},
		"enabled":             {"on"},
	}
	req := httptest.NewRequest(http.MethodPost, "/router/models", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ModelsAddHandler(opts).ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("add status = %d, want 303", rec.Code)
	}
	models, _ := opts.Roster.LoadRosterModels()
	if !hasModel(models, "glm-fast", "cheap") {
		t.Fatalf("added model not persisted: %+v", models)
	}

	// Re-tier the same model (upsert by ID) to premium.
	form.Set("tier", "premium")
	req = httptest.NewRequest(http.MethodPost, "/router/models", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	ModelsAddHandler(opts).ServeHTTP(rec, req)
	models, _ = opts.Roster.LoadRosterModels()
	if !hasModel(models, "glm-fast", "premium") {
		t.Fatalf("re-tier not persisted: %+v", models)
	}
	if countModel(models, "glm-fast") != 1 {
		t.Fatalf("re-tier should upsert, not duplicate: %+v", models)
	}
}

func TestModelsAddRejectsBadTier(t *testing.T) {
	opts := modelsTestOpts(t)
	form := url.Values{"id": {"x"}, "provider_id": {"openrouter"}, "provider_model_id": {"a/b"}, "tier": {"ultra"}}
	req := httptest.NewRequest(http.MethodPost, "/router/models", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ModelsAddHandler(opts).ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (redirect with error)", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=") {
		t.Errorf("expected error redirect, got %q", loc)
	}
	models, _ := opts.Roster.LoadRosterModels()
	if countModel(models, "x") != 0 {
		t.Errorf("invalid model should not be persisted: %+v", models)
	}
}

func TestModelsDelete(t *testing.T) {
	opts := modelsTestOpts(t)
	form := url.Values{"id": {"cheap-general"}}
	req := httptest.NewRequest(http.MethodPost, "/router/models/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ModelsDeleteHandler(opts).ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	models, _ := opts.Roster.LoadRosterModels()
	if countModel(models, "cheap-general") != 0 {
		t.Errorf("model not deleted: %+v", models)
	}
}

func hasModel(ms []providercfg.Model, id, tier string) bool {
	for _, m := range ms {
		if m.ID == id && m.Tier == tier {
			return true
		}
	}
	return false
}

func countModel(ms []providercfg.Model, id string) int {
	n := 0
	for _, m := range ms {
		if m.ID == id {
			n++
		}
	}
	return n
}

// The row edit form must let the admin change the SLUG inline — the field most
// often wrong (zai/glm-5.2 vs glm-5.2) was previously locked in a hidden input.
func TestModelsRowFormHasEditableSlug(t *testing.T) {
	opts := modelsTestOpts(t)
	rec := httptest.NewRecorder()
	ModelsPageHandler(opts).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/router/models", nil))
	body := rec.Body.String()
	if strings.Contains(body, `type="hidden" name="provider_model_id"`) {
		t.Fatal("slug must not be a hidden field in the row form")
	}
	if !strings.Contains(body, `name="provider_model_id" value=`) {
		t.Fatal("row form should carry an editable slug input")
	}
}
